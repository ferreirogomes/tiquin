package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	// Change from data to storage
)

type SolanaIntegrationService struct {
	RPCClient *rpc.Client
	FeePayer  solana.PrivateKey
}

func NewSolanaIntegrationService(rpcEndpoint, feePayerKeyBase58 string) *SolanaIntegrationService {
	client := rpc.New(rpcEndpoint)
	feePayer, err := solana.PrivateKeyFromBase58(feePayerKeyBase58)
	if err != nil {
		log.Fatalf("Falha ao carregar chave do Fee Payer: %v", err)
	}
	return &SolanaIntegrationService{
		RPCClient: client,
		FeePayer:  feePayer,
	}
}

func (s *SolanaIntegrationService) CreateMintAndTokenAccount(ownerPubKey solana.PublicKey, assetSymbol string) (solana.PublicKey, solana.PublicKey, error) {
	return solana.PublicKey{}, solana.PublicKey{}, nil
}

func (s *SolanaIntegrationService) MintTokensToAccount(mintAddress, destinationATA solana.PublicKey, amount uint64) (solana.Signature, error) {
	return solana.Signature{}, nil
}

// PrepareTransferTransaction serializes a transfer transaction for signing by the user.
// This function BUILDS the transaction but does NOT SIGN it with the sender's private key.
// The FeePayer pays the network fees.
func (s *SolanaIntegrationService) PrepareTransferTransaction(
	mintAddress, fromATA, toATA solana.PublicKey,
	fromOwnerPubKey solana.PublicKey, // Public key of the actual sender
	amount uint64,
) (string, error) { // Returns the transaction encoded in Base64
	resp, err := s.RPCClient.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get blockhash: %w", err)
	}
	recentBlockhash := resp.Value.Blockhash

	// Instruction to transfer tokens
	transferInstruction := token.NewTransferInstruction(
		amount,
		fromATA,
		toATA,
		fromOwnerPubKey,      // The "owner" of the source account is the actual sender
		[]solana.PublicKey{}, // Multisigners (none in this case)
	).Build()

	// The FeePayer pays the transaction fee
	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			transferInstruction,
		},
		recentBlockhash,
		solana.TransactionPayer(s.FeePayer.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transfer transaction: %w", err)
	}

	// The FeePayer MUST sign, as they are the transaction payer
	// The fromOwnerPubKey (sender) will sign on the frontend
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(s.FeePayer.PublicKey()) {
			return &s.FeePayer
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction by FeePayer: %w", err)
	}

	// Serialize the transaction to be sent to the client
	serializedTx, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return base64.StdEncoding.EncodeToString(serializedTx), nil
}

func (s *SolanaIntegrationService) SendSignedTransaction(signedTxBase64 string) (solana.Signature, error) {
	tx, err := solana.TransactionFromBase64(signedTxBase64)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to decode/deserialize signed transaction: %w", err)
	}

	txID, err := s.RPCClient.SendTransactionWithOpts(context.Background(), tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send signed transaction: %w", err)
	}
	log.Printf("Signed transaction sent: %s\n", txID)

	// Wait for confirmation (optional, but recommended for critical operations)
	_, err = s.RPCClient.GetSignatureStatuses(context.Background(), true, txID)
	if err != nil {
		log.Printf("Error checking transaction status %s: %v\n", txID, err)
	} else {
		log.Printf("Transaction %s confirmed.\n", txID)
	}

	return txID, nil
}
func (s *SolanaIntegrationService) GetTokenAccountBalance(tokenAccountAddress solana.PublicKey) (uint64, error) {
	return 0, nil
}

func (s *SolanaIntegrationService) GetTokenSupply(mintAddress solana.PublicKey) (uint64, error) {
	return 0, nil
}
