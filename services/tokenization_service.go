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

func (s *TokenizationService) CreateAsset(symbol, name string, totalShares float64) (models.Asset, error) {
	asset := models.Asset{
		ID:          uuid.New().String(),
		Symbol:      symbol,
		Name:        name,
		TotalShares: totalShares,
	}
	err := s.DB.SaveAsset(asset)
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

	// Check if the destination ATA exists. If not, it will need to be created.
	// In a real system, you may have a separate `CreateAssociatedTokenAccount` instruction,
	// or include that instruction in the same transaction as the transfer.
	// For simplicity, assume that if the ATA does not exist, it will be created when the first transfer is received.
	// Or, we can check and add the creation instruction if needed.
	_, err = s.SolanaS.RPCClient.GetAccountInfo(context.Background(), toATA)
	if err != nil && err.Error() == "account not found" {
		// ATA does not exist; include instruction to create it in the transaction
		log.Printf("Destination ATA %s not found. Including instruction to create it.", toATA.String())
		// Here you would need to build the CreateAssociatedTokenAccount instruction
		// and include it in the transaction being prepared.
		// This would make PrepareTransferTransaction more complex, as it would need to accept multiple instructions.
		// For simplicity in this example, assume the frontend or a separate process
		// ensures the destination ATA exists.
		// For a complete solution, consider adding this logic to SolanaIntegrationService.
		// Or the frontend can attempt to create the ATA before requesting the transfer.
	}

	currentBalance, err := s.SolanaS.GetTokenAccountBalance(fromATA)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to check sender balance on Solana: %w", err)
	}
	amountAtomic := uint64(amount * 1e9)
	if currentBalance < amountAtomic {
		return "", solana.PublicKey{}, errors.New("insufficient balance for transfer on Solana")
	}

	// Prepare the transaction, but do not sign with the user's key
	serializedTx, err := s.SolanaS.PrepareTransferTransaction(mintAddress, fromATA, toATA, fromUserPubKey, amountAtomic)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to prepare transfer transaction: %w", err)
	}

	return serializedTx, toATA, nil
}

// CompleteTransferTokenFromUser receives the signed transaction and sends it to Solana.
// This is the method that the final transfer endpoint will call.
func (s *TokenizationService) CompleteTransferTokenFromUser(
	assetID, fromUserID, toUserID string, amount float64, signedTxBase64 string,
	destinationATA solana.PublicKey, // Receives the destination ATA back from the handler
) (models.Token, error) {
	_, foundFrom, err := s.DB.GetUser(fromUserID)
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

	// Envia a transação assinada para a rede Solana
	txID, err := s.SolanaS.SendSignedTransaction(signedTxBase64)
	if err != nil {
		return models.Token{}, fmt.Errorf("failed to send signed transaction to Solana: %w", err)
	}

	// WARNING: The internal token record here is for tracking purposes only.
	// The source of truth is the blockchain. The listener will handle keeping things in sync.
	transferredToken := models.Token{
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
	if err := s.DB.SaveToken(transferredToken); err != nil {
		// This is a serious error: the transaction went to the blockchain, but the internal DB failed.
		// In a real application, you would need a robust reconciliation mechanism here.
		log.Printf("ERROR: Solana transaction %s sent, but failed to save internal record: %v", txID.String(), err)
		return models.Token{}, fmt.Errorf("transaction sent but failed to register internally: %w", err)
	}

	return transferredToken, nil
}
func (s *TokenizationService) GetUserTokensFromSolana(userID string) ([]models.Token, error) {
	return []models.Token{}, nil
}
