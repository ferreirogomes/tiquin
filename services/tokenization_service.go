package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ferreirogomes/tiquin/models"

	"github.com/ferreirogomes/tiquin/storage"
	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
)

type TokenizationService struct {
	DB      *storage.DB
	SolanaS *SolanaIntegrationService
}

func NewTokenizationService(db *storage.DB, solanaS *SolanaIntegrationService) *TokenizationService {
	return &TokenizationService{
		DB:      db,
		SolanaS: solanaS,
	}
}

// CreateAsset creates an asset record in the DB AND mints it on Solana.
func (s *TokenizationService) CreateAsset(symbol, name string, totalShares float64, ownerPubKey string) (models.Asset, error) {
	ownerKey, err := solana.PublicKeyFromBase58(ownerPubKey)
	if err != nil {
		return models.Asset{}, fmt.Errorf("invalid owner public key: %w", err)
	}

	mintAddress, _, err := s.SolanaS.CreateMintAndTokenAccount(ownerKey, symbol)
	if err != nil {
		return models.Asset{}, fmt.Errorf("failed to create mint on Solana: %w", err)
	}

	asset := models.Asset{
		ID:          uuid.New().String(),
		Symbol:      symbol,
		Name:        name,
		TotalShares: totalShares,
		MintAddress: mintAddress.String(),
	}
	err = s.DB.SaveAsset(asset)
	return asset, err
}

// PrepareTransferTokenFromUser builds a transaction to be signed by the user.
// Returns the transaction serialized in Base64 and the destination TokenAccountAddress.
func (s *TokenizationService) PrepareTransferTokenFromUser(
	assetID, fromUserID, toUserID string, amount float64,
) (string, solana.PublicKey, error) { // Returns Base64 string and toATA
	fromUser, foundFrom, err := s.DB.GetUser(fromUserID)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("error fetching sender user: %w", err)
	}
	if !foundFrom || fromUser.SolanaPubKey == "" {
		return "", solana.PublicKey{}, errors.New("sender user not found or missing Solana public key")
	}
	toUser, foundTo, err := s.DB.GetUser(toUserID)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("error fetching recipient user: %w", err)
	}
	if !foundTo || toUser.SolanaPubKey == "" {
		return "", solana.PublicKey{}, errors.New("recipient user not found or missing Solana public key")
	}

	asset, foundAsset, err := s.DB.GetAsset(assetID)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("error fetching asset: %w", err)
	}
	if !foundAsset || asset.MintAddress == "" {
		return "", solana.PublicKey{}, errors.New("asset not found or not tokenized")
	}

	mintAddress, err := solana.PublicKeyFromBase58(asset.MintAddress)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("invalid Mint address: %w", err)
	}

	fromUserPubKey, err := solana.PublicKeyFromBase58(fromUser.SolanaPubKey)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("invalid sender public key: %w", err)
	}
	toUserPubKey, err := solana.PublicKeyFromBase58(toUser.SolanaPubKey)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("invalid recipient public key: %w", err)
	}

	fromATA, _, err := solana.FindAssociatedTokenAddress(fromUserPubKey, mintAddress)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to find sender ATA: %w", err)
	}

	toATA, _, err := solana.FindAssociatedTokenAddress(toUserPubKey, mintAddress)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to find recipient ATA: %w", err)
	}

	// Ensure destination ATA exists; create it if not (FeePayer covers the cost)
	created, err := s.SolanaS.EnsureATAExists(toUserPubKey, mintAddress, toATA)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to ensure destination ATA exists: %w", err)
	}
	if created {
		log.Printf("Created destination ATA %s for user %s", toATA, toUserID)
	}

	// P1 fix: real balance check from Solana
	currentBalance, err := s.SolanaS.GetTokenAccountBalance(fromATA)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to check sender balance on Solana: %w", err)
	}
	amountAtomic := uint64(amount * 1e9)
	if currentBalance < amountAtomic {
		return "", solana.PublicKey{}, fmt.Errorf("insufficient balance: have %d, need %d atomic units", currentBalance, amountAtomic)
	}

	// Prepare the transaction, but do not sign with the user's key
	serializedTx, err := s.SolanaS.PrepareTransferTransaction(mintAddress, fromATA, toATA, fromUserPubKey, amountAtomic)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to prepare transfer transaction: %w", err)
	}

	return serializedTx, toATA, nil
}

// CompleteTransferTokenFromUser receives the signed transaction, sends it to Solana,
// and updates the internal DB: debits the sender and credits the recipient.
func (s *TokenizationService) CompleteTransferTokenFromUser(
	assetID, fromUserID, toUserID string, amount float64, signedTxBase64 string,
	destinationATA solana.PublicKey, // Receives the destination ATA back from the handler
) (models.Token, error) {
	fromUser, foundFrom, err := s.DB.GetUser(fromUserID)
	if err != nil {
		return models.Token{}, fmt.Errorf("error fetching sender user: %w", err)
	}
	if !foundFrom {
		return models.Token{}, errors.New("sender user not found")
	}
	toUser, foundTo, err := s.DB.GetUser(toUserID)
	if err != nil {
		return models.Token{}, fmt.Errorf("error fetching recipient user: %w", err)
	}
	if !foundTo {
		return models.Token{}, errors.New("recipient user not found")
	}

	asset, foundAsset, err := s.DB.GetAsset(assetID)
	if err != nil {
		return models.Token{}, fmt.Errorf("error fetching asset: %w", err)
	}
	if !foundAsset || asset.MintAddress == "" {
		return models.Token{}, errors.New("asset not found or not tokenized")
	}

	// Send the signed transaction to Solana
	txID, err := s.SolanaS.SendSignedTransaction(signedTxBase64)
	if err != nil {
		return models.Token{}, fmt.Errorf("failed to send signed transaction to Solana: %w", err)
	}

	// P3 fix: Debit sender and credit recipient in the DB
	// Use a DB transaction to keep both operations atomic
	if err := s.DB.TransferTokenBalance(fromUser.ID, toUser.ID, asset.ID, amount, txID.String(), destinationATA.String()); err != nil {
		// This is a serious error: the transaction went to the blockchain, but the internal DB failed.
		// The blockchain listener will eventually reconcile via backfill.
		log.Printf("ERROR: Solana tx %s sent, but DB transfer failed: %v", txID, err)
		return models.Token{}, fmt.Errorf("transaction sent but failed to update internal records: %w", err)
	}

	// Return the new recipient's token record
	recipientToken := models.Token{
		ID:                  uuid.New().String(),
		AssetID:             asset.ID,
		OwnerID:             toUser.ID,
		Amount:              amount,
		SmartContractRules:  asset.Name + " rules",
		IsTradable:          true,
		CreatedAt:           time.Now(),
		MintAddress:         asset.MintAddress,
		TokenAccountAddress: destinationATA.String(),
		TransactionID:       txID.String(),
	}

	return recipientToken, nil
}

// MintInitialTokens mints the initial total supply of an asset to the owner's ATA.
func (s *TokenizationService) MintInitialTokens(asset models.Asset, ownerPubKey string) (models.Token, error) {
	ownerKey, err := solana.PublicKeyFromBase58(ownerPubKey)
	if err != nil {
		return models.Token{}, fmt.Errorf("invalid owner public key: %w", err)
	}

	mintAddress, err := solana.PublicKeyFromBase58(asset.MintAddress)
	if err != nil {
		return models.Token{}, fmt.Errorf("invalid mint address: %w", err)
	}

	ownerATA, _, err := solana.FindAssociatedTokenAddress(ownerKey, mintAddress)
	if err != nil {
		return models.Token{}, fmt.Errorf("failed to derive owner ATA: %w", err)
	}

	amountAtomic := uint64(asset.TotalShares * 1e9)
	sig, err := s.SolanaS.MintTokensToAccount(mintAddress, ownerATA, amountAtomic)
	if err != nil {
		return models.Token{}, fmt.Errorf("failed to mint tokens: %w", err)
	}

	// Use context.Background() for DB operations — no external context needed here
	_ = context.Background()

	tokenRecord := models.Token{
		ID:                  uuid.New().String(),
		AssetID:             asset.ID,
		OwnerID:             ownerPubKey, // Store pubkey as owner reference before user lookup
		Amount:              asset.TotalShares,
		SmartContractRules:  asset.Name + " rules",
		IsTradable:          true,
		MintAddress:         asset.MintAddress,
		TokenAccountAddress: ownerATA.String(),
		TransactionID:       sig.String(),
		CreatedAt:           time.Now(),
	}

	if err := s.DB.SaveToken(tokenRecord); err != nil {
		log.Printf("WARNING: minted %d tokens (tx %s) but failed to save record: %v", amountAtomic, sig, err)
	}

	return tokenRecord, nil
}

func (s *TokenizationService) GetUserTokensFromSolana(userID string) ([]models.Token, error) {
	return s.DB.GetTokensByOwnerID(userID)
}
