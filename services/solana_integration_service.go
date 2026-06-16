package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
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
		log.Fatalf("Failed to load FeePayer key: %v", err)
	}
	return &SolanaIntegrationService{
		RPCClient: client,
		FeePayer:  feePayer,
	}
}

// CreateMintAndTokenAccount creates a new SPL Token Mint and the owner's
// Associated Token Account on Solana. The FeePayer acts as the Mint Authority.
// Returns (mintAddress, tokenAccountAddress, error).
func (s *SolanaIntegrationService) CreateMintAndTokenAccount(
	ownerPubKey solana.PublicKey, assetSymbol string,
) (solana.PublicKey, solana.PublicKey, error) {
	ctx := context.Background()

	// 1. Generate a new keypair for the Mint account
	mintKeypair, err := solana.NewRandomPrivateKey()
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to generate mint keypair: %w", err)
	}
	mintPubKey := mintKeypair.PublicKey()

	// 2. Get the minimum lamports needed to exempt the mint account from rent
	mintAccountSize := uint64(82) // SPL Mint account size in bytes
	rentExemption, err := s.RPCClient.GetMinimumBalanceForRentExemption(ctx, mintAccountSize, rpc.CommitmentFinalized)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to get rent exemption: %w", err)
	}

	// 3. Get recent blockhash
	resp, err := s.RPCClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to get blockhash: %w", err)
	}

	// 4. Build instructions:
	//    a) CreateAccount for the Mint
	//    b) InitializeMint (9 decimals, FeePayer as mint authority and freeze authority)
	//    c) CreateAssociatedTokenAccount for the owner

	ownerATA, _, err := solana.FindAssociatedTokenAddress(ownerPubKey, mintPubKey)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to derive ATA: %w", err)
	}

	feePayerPubKey := s.FeePayer.PublicKey()

	createAccountIx := system.NewCreateAccountInstruction(
		rentExemption,
		mintAccountSize,
		solana.TokenProgramID,
		feePayerPubKey,
		mintPubKey,
	).Build()

	initMintIx := token.NewInitializeMintInstruction(
		9,             // decimals
		feePayerPubKey, // mint authority
		feePayerPubKey, // freeze authority
		mintPubKey,
		solana.SysVarRentPubkey,
	).Build()

	createATAIx := associatedtokenaccount.NewCreateInstruction(
		feePayerPubKey,
		ownerPubKey,
		mintPubKey,
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{createAccountIx, initMintIx, createATAIx},
		resp.Value.Blockhash,
		solana.TransactionPayer(feePayerPubKey),
	)
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to build create-mint transaction: %w", err)
	}

	// Sign with both FeePayer and the new Mint keypair
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(feePayerPubKey) {
			return &s.FeePayer
		}
		if key.Equals(mintPubKey) {
			return &mintKeypair
		}
		return nil
	})
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to sign create-mint transaction: %w", err)
	}

	sig, err := s.RPCClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return solana.PublicKey{}, solana.PublicKey{}, fmt.Errorf("failed to send create-mint transaction: %w", err)
	}
	log.Printf("Mint created: %s | ATA: %s | TxID: %s", mintPubKey, ownerATA, sig)

	return mintPubKey, ownerATA, nil
}

// MintTokensToAccount mints `amount` atomic units of `mintAddress` tokens
// to `destinationATA`. The FeePayer must be the Mint Authority.
func (s *SolanaIntegrationService) MintTokensToAccount(
	mintAddress, destinationATA solana.PublicKey, amount uint64,
) (solana.Signature, error) {
	ctx := context.Background()

	resp, err := s.RPCClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get blockhash: %w", err)
	}

	mintToIx := token.NewMintToInstruction(
		amount,
		mintAddress,
		destinationATA,
		s.FeePayer.PublicKey(), // mint authority
		[]solana.PublicKey{},
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{mintToIx},
		resp.Value.Blockhash,
		solana.TransactionPayer(s.FeePayer.PublicKey()),
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to build mint-to transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(s.FeePayer.PublicKey()) {
			return &s.FeePayer
		}
		return nil
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to sign mint-to transaction: %w", err)
	}

	sig, err := s.RPCClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send mint-to transaction: %w", err)
	}
	log.Printf("Minted %d tokens to %s | TxID: %s", amount, destinationATA, sig)

	return sig, nil
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

// EnsureATAExists checks if a token account exists and creates it if not.
// Returns true if it was created, false if it already existed.
func (s *SolanaIntegrationService) EnsureATAExists(
	ownerPubKey, mintAddress, ataAddress solana.PublicKey,
) (bool, error) {
	ctx := context.Background()

	_, err := s.RPCClient.GetAccountInfo(ctx, ataAddress)
	if err == nil {
		return false, nil // Already exists
	}

	// Create the ATA
	resp, err := s.RPCClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return false, fmt.Errorf("failed to get blockhash: %w", err)
	}

	createATAIx := associatedtokenaccount.NewCreateInstruction(
		s.FeePayer.PublicKey(),
		ownerPubKey,
		mintAddress,
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{createATAIx},
		resp.Value.Blockhash,
		solana.TransactionPayer(s.FeePayer.PublicKey()),
	)
	if err != nil {
		return false, fmt.Errorf("failed to build create-ATA transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(s.FeePayer.PublicKey()) {
			return &s.FeePayer
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to sign create-ATA transaction: %w", err)
	}

	sig, err := s.RPCClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return false, fmt.Errorf("failed to send create-ATA transaction: %w", err)
	}
	log.Printf("Created ATA %s for owner %s | TxID: %s", ataAddress, ownerPubKey, sig)

	return true, nil
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

// GetTokenAccountBalance fetches the real token balance from Solana for a given ATA.
// Returns the amount in atomic units (raw SPL token amount as uint64).
func (s *SolanaIntegrationService) GetTokenAccountBalance(tokenAccountAddress solana.PublicKey) (uint64, error) {
	ctx := context.Background()

	result, err := s.RPCClient.GetTokenAccountBalance(ctx, tokenAccountAddress, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("failed to get token account balance for %s: %w", tokenAccountAddress, err)
	}
	if result == nil || result.Value == nil {
		return 0, fmt.Errorf("empty token balance response for %s", tokenAccountAddress)
	}

	// Solana returns amounts as strings to preserve uint64 precision in JSON
	amount, err := strconv.ParseUint(result.Value.Amount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse token balance amount %q: %w", result.Value.Amount, err)
	}
	return amount, nil
}

func (s *SolanaIntegrationService) GetTokenSupply(mintAddress solana.PublicKey) (uint64, error) {
	ctx := context.Background()

	result, err := s.RPCClient.GetTokenSupply(ctx, mintAddress, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("failed to get token supply for %s: %w", mintAddress, err)
	}
	if result == nil || result.Value == nil {
		return 0, fmt.Errorf("empty token supply response for %s", mintAddress)
	}

	amount, err := strconv.ParseUint(result.Value.Amount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse token supply amount %q: %w", result.Value.Amount, err)
	}
	return amount, nil
}
