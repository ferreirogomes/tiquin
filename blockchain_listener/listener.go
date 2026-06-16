package blockchain_listener

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ferreirogomes/tiquin/models"
	"github.com/ferreirogomes/tiquin/storage"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws" // For WebSockets
)

// BlockchainListener listens for events on Solana to keep the DB synchronized.
type BlockchainListener struct {
	RPCClient   *rpc.Client
	RPCEndpoint string
	DB          *storage.DB
	FeePayerPK  solana.PrivateKey // Fee Payer key used to identify relevant transactions
	stopCh      chan struct{}      // QW3: graceful shutdown channel
}

// NewBlockchainListener creates a new listener instance.
func NewBlockchainListener(rpcEndpoint string, db *storage.DB, feePayerKeyBase58 string) *BlockchainListener {
	rpcClient := rpc.New(rpcEndpoint)

	feePayer, err := solana.PrivateKeyFromBase58(feePayerKeyBase58)
	if err != nil {
		log.Fatalf("Failed to load Fee Payer private key for listener: %v", err)
	}

	return &BlockchainListener{
		RPCClient:   rpcClient,
		RPCEndpoint: rpcEndpoint,
		DB:          db,
		FeePayerPK:  feePayer,
		stopCh:      make(chan struct{}),
	}
}

// Stop signals the listener to shut down gracefully.
func (l *BlockchainListener) Stop() {
	close(l.stopCh)
}

// StartListening starts listening for events. Retries on disconnection.
// Blocks until Stop() is called.
func (l *BlockchainListener) StartListening() {
	log.Println("Starting blockchain listener...")

	for {
		select {
		case <-l.stopCh:
			log.Println("Blockchain listener stopped.")
			return
		default:
		}

		err := l.listenLoop()
		if err != nil {
			log.Printf("Listener disconnected or error: %v. Retrying in 5 seconds...", err)
			select {
			case <-l.stopCh:
				log.Println("Blockchain listener stopped during retry wait.")
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (l *BlockchainListener) listenLoop() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop the context when shutdown is requested
	go func() {
		select {
		case <-l.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// 1. Connect WebSocket
	wsClient, err := ws.Connect(ctx, l.RPCEndpoint)
	if err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}
	defer wsClient.Close()

	// 2. Backfill: Process recent transactions that may have been missed
	l.backfillTransactions()

	// Subscribe to transactions involving the FeePayer (Mint Authority)
	sub, err := wsClient.LogsSubscribeMentions(
		l.FeePayerPK.PublicKey(),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %w", err)
	}
	defer sub.Unsubscribe()

	log.Println("Listening for new transactions (logs)...")
	for {
		got, err := sub.Recv(ctx)
		if err != nil {
			select {
			case <-l.stopCh:
				return nil // clean shutdown
			default:
				return fmt.Errorf("error receiving log: %w", err)
			}
		}

		// Only process if no error was reported in the transaction log
		if got.Value.Err == nil {
			log.Printf("Transaction with FeePayer detected (Signature: %s). Processing...", got.Value.Signature)
			l.ProcessTransaction(got.Value.Signature)
		} else {
			log.Printf("Transaction %s failed in log: %v", got.Value.Signature, got.Value.Err)
		}
	}
}

// backfillTransactions fetches recent transactions to ensure nothing was missed.
func (l *BlockchainListener) backfillTransactions() {
	log.Println("Running transaction backfill...")
	limit := 50 // Fetch the last 50 transactions
	signatures, err := l.RPCClient.GetSignaturesForAddressWithOpts(
		context.Background(),
		l.FeePayerPK.PublicKey(),
		&rpc.GetSignaturesForAddressOpts{Limit: &limit, Commitment: rpc.CommitmentFinalized},
	)
	if err != nil {
		log.Printf("Error fetching transaction history for backfill: %v", err)
		return
	}

	// Process from oldest to newest to maintain chronological order
	for i := len(signatures) - 1; i >= 0; i-- {
		sig := signatures[i]
		if sig.Err == nil {
			l.ProcessTransaction(sig.Signature)
		}
	}
}

// ProcessTransaction fetches transaction details and routes to the appropriate handler.
// P2 fix: this now actually parses and dispatches to handleMintTo / handleTransfer.
func (l *BlockchainListener) ProcessTransaction(signature solana.Signature) {
	log.Printf("Fetching transaction details for %s...", signature.String())

	txResp, err := l.RPCClient.GetTransaction(context.Background(), signature, &rpc.GetTransactionOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   solana.EncodingJSONParsed,
	})
	if err != nil {
		log.Printf("Failed to get transaction details for %s: %v", signature.String(), err)
		return
	}
	if txResp == nil || txResp.Transaction == nil {
		log.Printf("Transaction details for %s are empty.", signature.String())
		return
	}

	// Parse the JSON-encoded transaction to find SPL token instructions
	// The transaction meta contains pre/post token balances which is the most reliable source
	if txResp.Meta == nil {
		log.Printf("No meta for transaction %s, skipping.", signature.String())
		return
	}

	// Use pre/post token balances to detect transfers and mints
	preBalances := txResp.Meta.PreTokenBalances
	postBalances := txResp.Meta.PostTokenBalances

	if len(postBalances) == 0 {
		log.Printf("No token balance changes in %s, skipping.", signature.String())
		return
	}

	// Build a map of accountIndex -> post balance
	postMap := make(map[uint16]rpc.TokenBalance)
	for _, pb := range postBalances {
		postMap[pb.AccountIndex] = pb
	}
	preMap := make(map[uint16]rpc.TokenBalance)
	for _, pb := range preBalances {
		preMap[pb.AccountIndex] = pb
	}

	// Detect MintTo: post balance exists but pre balance doesn't (new tokens created)
	// Detect Transfer: both pre and post exist, with delta
	for idx, post := range postMap {
		pre, hasPre := preMap[idx]
		mintAddr := post.Mint.String()

		postAmt := parseTokenAmount(post.UiTokenAmount.Amount)
		var preAmt uint64
		if hasPre {
			preAmt = parseTokenAmount(pre.UiTokenAmount.Amount)
		}

		if postAmt <= preAmt {
			continue // No increase, skip (this account was debited or unchanged)
		}

		delta := postAmt - preAmt
		owner := ""
		if post.Owner != nil {
			owner = post.Owner.String()
		}

		if !hasPre {
			// MintTo: token account created and funded
			l.handleMintTo(signature, mintAddr, post.Mint.String(), owner, float64(delta)/1e9)
		} else {
			// Transfer: existing account received tokens
			l.handleTransfer(signature, mintAddr, owner, float64(delta)/1e9)
		}
	}
}

// handleMintTo processes a detected MintTo event from balance analysis.
func (l *BlockchainListener) handleMintTo(signature solana.Signature, tokenAccountAddr, mintAddr, ownerPubKey string, amount float64) {
	log.Printf("'mintTo' event detected for mint %s, owner %s, amount %f", mintAddr, ownerPubKey, amount)

	asset, foundAsset, err := l.DB.GetAssetByMintAddress(mintAddr)
	if err != nil {
		log.Printf("Error fetching asset by MintAddress %s: %v", mintAddr, err)
		return
	}
	if !foundAsset {
		log.Printf("Asset for MintAddress %s not found in internal DB. Skipping.", mintAddr)
		return
	}

	ownerUser, foundUser, err := l.DB.GetUserBySolanaPubKey(ownerPubKey)
	if err != nil {
		log.Printf("Error fetching owner by SolanaPubKey %s: %v", ownerPubKey, err)
		return
	}
	if !foundUser {
		log.Printf("Owner for ATA %s (pubkey %s) not in internal DB. Skipping.", tokenAccountAddr, ownerPubKey)
		return
	}

	// Idempotency check
	txExists, err := l.DB.TransactionExists(signature.String())
	if err != nil {
		log.Printf("Error checking transaction idempotency: %v", err)
		return
	}
	if txExists {
		log.Printf("Transaction %s already processed for MintTo. Skipping.", signature.String())
		return
	}

	tokenRecord := models.Token{
		ID:                  signature.String() + "-mint", // Deterministic ID
		AssetID:             asset.ID,
		OwnerID:             ownerUser.ID,
		Amount:              amount,
		SmartContractRules:  asset.Name + " rules",
		IsTradable:          true,
		MintAddress:         mintAddr,
		TokenAccountAddress: tokenAccountAddr,
		TransactionID:       signature.String(),
		CreatedAt:           time.Now(),
	}
	if err := l.DB.SaveToken(tokenRecord); err != nil {
		log.Printf("Failed to save token record for MintTo %s: %v", signature.String(), err)
	} else {
		log.Printf("MintTo synced: asset %s, owner %s, amount %f, tx %s", asset.Symbol, ownerUser.ID, amount, signature.String())
	}
}

// handleTransfer processes a detected Transfer event from balance analysis.
func (l *BlockchainListener) handleTransfer(signature solana.Signature, mintAddr, toOwnerPubKey string, amount float64) {
	log.Printf("'transfer' event detected for mint %s, to owner %s, amount %f", mintAddr, toOwnerPubKey, amount)

	asset, foundAsset, err := l.DB.GetAssetByMintAddress(mintAddr)
	if err != nil {
		log.Printf("Error fetching asset by MintAddress %s: %v", mintAddr, err)
		return
	}
	if !foundAsset {
		log.Printf("Asset for MintAddress %s not found in internal DB. Skipping transfer.", mintAddr)
		return
	}

	toUser, foundToUser, err := l.DB.GetUserBySolanaPubKey(toOwnerPubKey)
	if err != nil {
		log.Printf("Error fetching recipient user by SolanaPubKey %s: %v", toOwnerPubKey, err)
		return
	}
	if !foundToUser {
		log.Printf("Recipient (pubkey %s) not found in internal DB for TxID %s. Skipping.", toOwnerPubKey, signature.String())
		return
	}

	// Idempotency check
	txExists, err := l.DB.TransactionExists(signature.String())
	if err != nil {
		log.Printf("Error checking transaction idempotency: %v", err)
		return
	}
	if txExists {
		log.Printf("Transaction %s already processed for Transfer. Skipping.", signature.String())
		return
	}

	// Record the transfer for the recipient
	// Note: sender debit is handled in CompleteTransferTokenFromUser for internal transfers.
	// For external/unknown senders, we just record the receipt.
	transferredTokenRecord := models.Token{
		ID:            signature.String() + "-transfer",
		AssetID:       asset.ID,
		OwnerID:       toUser.ID,
		Amount:        amount,
		SmartContractRules: "Transferred via blockchain listener",
		IsTradable:    true,
		MintAddress:   mintAddr,
		TransactionID: signature.String(),
		CreatedAt:     time.Now(),
	}

	if err := l.DB.SaveToken(transferredTokenRecord); err != nil {
		log.Printf("Failed to save token record for Transfer %s: %v", signature.String(), err)
	} else {
		log.Printf("Transfer synced: asset %s, to %s, amount %f, tx %s", asset.Symbol, toUser.ID, amount, signature.String())
	}
}

// parseTokenAmount safely parses a token amount string to uint64.
func parseTokenAmount(s string) uint64 {
	if s == "" {
		return 0
	}
	var f big.Float
	_, _, err := f.Parse(s, 10)
	if err != nil {
		return 0
	}
	val, _ := f.Uint64()
	return val
}

// parseAmountFromString attempts to parse a string value to float64.
func parseAmountFromString(s string) (float64, error) {
	var f big.Float
	_, _, err := f.Parse(s, 10)
	if err != nil {
		return 0, fmt.Errorf("failed to parse string to float: %w", err)
	}
	val, _ := f.Float64()
	return val, nil
}
