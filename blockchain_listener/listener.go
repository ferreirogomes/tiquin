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
	}
}

// StartListening starts listening for events.
func (l *BlockchainListener) StartListening() {
	log.Println("Starting blockchain listener...")

	for {
		err := l.listenLoop()
		if err != nil {
			log.Printf("Listener disconnected or error: %v. Retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (l *BlockchainListener) listenLoop() error {
	// 1. Connect WebSocket
	wsClient, err := ws.Connect(context.Background(), l.RPCEndpoint)
	if err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}
	defer wsClient.Close()

	// 2. Backfill: Process recent transactions that may have been missed
	l.backfillTransactions()

	// Example: Subscribe to transactions involving the FeePayer (who creates Mints and signs transactions)
	// In a real system, you would subscribe to specific token accounts or all known Mints.
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
		got, err := sub.Recv(context.Background())
		if err != nil {
			return fmt.Errorf("error receiving log: %w", err)
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

// ProcessTransaction fetches transaction details and updates the DB.
func (l *BlockchainListener) ProcessTransaction(signature solana.Signature) {
	log.Printf("Fetching transaction details for %s...", signature.String())

	// Get full transaction details
	txResp, err := l.RPCClient.GetTransaction(context.Background(), signature, &rpc.GetTransactionOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   solana.EncodingJSONParsed, // To get token details
	})
	if err != nil {
		log.Printf("Failed to get transaction details for %s: %v", signature.String(), err)
		return
	}
	if txResp == nil || txResp.Transaction == nil {
		log.Printf("Transaction details for %s are empty.", signature.String())
		return
	}

	// To fetch logs and simpler transfers, real applications use sub.LogsSubscribe,
	// then extract the transaction with GetTransaction as already done.

	log.Printf("Advanced transaction parse processing omitted in PoC for %s", signature.String())
}

// handleMintTo processes a MintTo instruction.
func (l *BlockchainListener) handleMintTo(signature solana.Signature, info interface{}) {
	log.Println("'mintTo' instruction detected.")
	// 'info' is an interface map. We need to do a type assertion.
	infoMap, ok := info.(map[string]interface{})
	if !ok {
		log.Println("Failed to convert info to map for 'mintTo'.")
		return
	}

	mint := infoMap["mint"].(string)
	account := infoMap["account"].(string)
	amountFloat, ok := infoMap["amount"].(float64) // May come as float or string
	if !ok {
		amountStr, ok := infoMap["amount"].(string)
		if ok {
			var err error
			amountFloat, err = parseAmountFromString(amountStr)
			if err != nil {
				log.Printf("Failed to parse 'amount' from string to float: %v", err)
				return
			}
		} else {
			log.Println("'amount' field in 'mintTo' is neither float64 nor string.")
			return
		}
	}

	// Find the asset by mint_address to get the asset.ID
	asset, foundAsset, err := l.DB.GetAssetByMintAddress(mint)
	if err != nil {
		log.Printf("Error fetching asset by MintAddress %s: %v", mint, err)
		return
	}
	if !foundAsset {
		log.Printf("Asset for MintAddress %s not found in internal DB. Skipping.", mint)
		return
	}

	// Try to find the user who owns the `account` (ATA)
	ownerUser, foundUser, err := l.DB.GetUserBySolanaPubKey(infoMap["owner"].(string))
	if err != nil {
		log.Printf("Error fetching owner by SolanaPubKey %s: %v", infoMap["owner"].(string), err)
		return
	}
	if !foundUser {
		log.Printf("Owner user for ATA %s not found in internal DB. May be an external user's ATA.", account)
		// You can decide to create a record for external users or skip.
		// For simplicity, skip if user is not internal.
		return
	}

	// Idempotency: Check if we have already processed this transaction
	txExists, err := l.DB.TransactionExists(signature.String())
	if err != nil {
		log.Printf("Error checking transaction idempotency in database: %v", err)
		return
	}
	if txExists {
		log.Printf("Transaction %s already processed for MintTo. Skipping.", signature.String())
		return
	}

	// Create or update the token record to reflect the mint
	tokenID, _ := solana.NewRandomPrivateKey()
	tokenRecord := models.Token{
		ID:                  tokenID.PublicKey().String(), // Random ID for the internal record
		AssetID:             asset.ID,
		OwnerID:             ownerUser.ID,
		Amount:              amountFloat / 1e9, // Convert back from atomic units (if 9 decimals)
		SmartContractRules:  asset.Name + " rules",
		IsTradable:          true,
		MintAddress:         mint,
		TokenAccountAddress: account,
		TransactionID:       signature.String(),
		CreatedAt:           time.Now(),
	}
	if err := l.DB.SaveToken(tokenRecord); err != nil { // SaveToken does ON CONFLICT UPDATE
		log.Printf("Failed to save/update token record for MintTo %s: %v", signature.String(), err)
	} else {
		log.Printf("Token minted (mintTo) for Asset %s, OwnerID %s, Amount %f, TxID %s", asset.Symbol, ownerUser.ID, tokenRecord.Amount, signature.String())
	}
}

// handleTransfer processes a Transfer instruction.
func (l *BlockchainListener) handleTransfer(signature solana.Signature, info interface{}) {
	log.Println("'transfer' instruction detected.")
	infoMap, ok := info.(map[string]interface{})
	if !ok {
		log.Println("Failed to convert info to map for 'transfer'.")
		return
	}

	destination := infoMap["destination"].(string)
	amountFloat, ok := infoMap["amount"].(float64)
	if !ok {
		amountStr, ok := infoMap["amount"].(string)
		if ok {
			var err error
			amountFloat, err = parseAmountFromString(amountStr)
			if err != nil {
				log.Printf("Failed to parse 'amount' from string to float: %v", err)
				return
			}
		} else {
			log.Println("'amount' field in 'transfer' is neither float64 nor string.")
			return
		}
	}

	// For a transfer, we would need to identify the MintAddress of the transferred token.
	// This can be obtained by looking up the 'source' or 'destination' account and checking its 'mint'.
	// For simplicity, assume mint info is available via the token account.
	mintAddress := "MockMintAddress"

	asset, foundAsset, err := l.DB.GetAssetByMintAddress(mintAddress)
	if err != nil {
		log.Printf("Error fetching asset by MintAddress %s: %v", mintAddress, err)
		return
	}
	if !foundAsset {
		log.Printf("Asset for MintAddress %s not found in internal DB. Skipping transfer.", mintAddress)
		return
	}

	// Identify sender and recipient in our DB
	// This is slightly complex since GetTokenAccountsByOwner returns ATAs, not users directly.
	// The best approach is to fetch the ATA owner from Solana itself, then map to our DB.
	fromOwnerPubKey := infoMap["authority"].(string)      // Who signed the transfer transaction
	toOwnerPubKey := infoMap["destinationOwner"].(string) // Who owns the destination account

	fromUser, foundFromUser, err := l.DB.GetUserBySolanaPubKey(fromOwnerPubKey)
	if err != nil {
		log.Printf("Error fetching sender user by SolanaPubKey %s: %v", fromOwnerPubKey, err)
		return
	}
	toUser, foundToUser, err := l.DB.GetUserBySolanaPubKey(toOwnerPubKey)
	if err != nil {
		log.Printf("Error fetching recipient user by SolanaPubKey %s: %v", toOwnerPubKey, err)
		return
	}

	if !foundFromUser || !foundToUser {
		log.Printf("Sender or recipient (or both) not found in internal DB for TxID %s. From %s to %s. Skipping.",
			signature.String(), fromOwnerPubKey, toOwnerPubKey)
		return
	}

	// Idempotency: Check if we have already processed this transaction
	txExists, err := l.DB.TransactionExists(signature.String())
	if err != nil {
		log.Printf("Error checking transaction idempotency in database: %v", err)
		return
	}
	if txExists {
		log.Printf("Transaction %s already processed for Transfer. Skipping.", signature.String())
		return
	}

	// Now that we identified internal users, we can update the database.
	// In a real system, you would not "create a new token" for every transfer.
	// You would update the token balance a user holds.
	// For this example, we simplify the record updates.

	// Ownership update logic:
	// 1. Find the "original" token record associated with asset_id and fromUser.ID
	//    and subtract the amount.
	// 2. Find the "original" token record associated with asset_id and toUser.ID
	//    and add the amount.
	// This would require methods in storage.DB to fetch and update balances by (AssetID, OwnerID).

	// Simplified example of how to record the transfer for internal history purposes:
	// Creates a new token record representing the transfer to the new owner.
	// In production, the logic would be more complex to manage balances per user/asset.
	tokenID, _ := solana.NewRandomPrivateKey()
	transferredTokenRecord := models.Token{
		ID:                  tokenID.PublicKey().String(),
		AssetID:             asset.ID,
		OwnerID:             toUser.ID, // The new owner
		Amount:              amountFloat / 1e9,
		SmartContractRules:  "Transferred via blockchain",
		IsTradable:          true,
		MintAddress:         mintAddress,
		TokenAccountAddress: destination, // The destination account
		TransactionID:       signature.String(),
		CreatedAt:           time.Now(),
	}

	if err := l.DB.SaveToken(transferredTokenRecord); err != nil {
		log.Printf("Failed to save token record for Transfer %s: %v", signature.String(), err)
	} else {
		log.Printf("Token transferred (transfer) from %s to %s. Asset: %s, Amount: %f, TxID: %s",
			fromUser.ID, toUser.ID, asset.Symbol, transferredTokenRecord.Amount, signature.String())
	}
}

// parseAmountFromString attempts to parse a string value to float64.
// Useful because the 'amount' field may come as a string in some RPC instructions.
func parseAmountFromString(s string) (float64, error) {
	var f big.Float
	_, _, err := f.Parse(s, 10)
	if err != nil {
		return 0, fmt.Errorf("failed to parse string to float: %w", err)
	}
	val, _ := f.Float64()
	return val, nil
}

// ... Other helper functions if needed ...

// For parseAmountFromString
//import (
//    "math/big"
//)
