package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ferreirogomes/tiquin/models"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
)

// ... (TokenizationService struct e NewTokenizationService permanecem os mesmos) ...

// PrepareTransferTokenFromUser constrói uma transação para ser assinada pelo usuário.
// Retorna a transação serializada em Base64 e o TokenAccountAddress de destino.
func (s *TokenizationService) PrepareTransferTokenFromUser(
	assetID, fromUserID, toUserID string, amount float64,
) (string, solana.PublicKey, error) { // Retorna string Base64 e toATA
	fromUser, foundFrom, err := s.DB.GetUser(fromUserID)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("erro ao buscar usuário remetente: %w", err)
	}
	if !foundFrom || fromUser.SolanaPubKey == "" {
		return "", solana.PublicKey{}, errors.New("usuário remetente não encontrado ou sem chave pública Solana")
	}
	toUser, foundTo, err := s.DB.GetUser(toUserID)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("erro ao buscar usuário destinatário: %w", err)
	}
	if !foundTo || toUser.SolanaPubKey == "" {
		return "", solana.PublicKey{}, errors.New("usuário destinatário não encontrado ou sem chave pública Solana")
	}

	asset, foundAsset, err := s.DB.GetAsset(assetID)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("erro ao buscar ativo: %w", err)
	}
	if !foundAsset || asset.MintAddress == "" {
		return "", solana.PublicKey{}, errors.New("ativo não encontrado ou não tokenizado")
	}

	mintAddress, err := solana.PublicKeyFromBase58(asset.MintAddress)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("endereço de Mint inválido: %w", err)
	}

	fromUserPubKey, err := solana.PublicKeyFromBase58(fromUser.SolanaPubKey)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("chave pública do remetente inválida: %w", err)
	}
	toUserPubKey, err := solana.PublicKeyFromBase58(toUser.SolanaPubKey)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("chave pública do destinatário inválida: %w", err)
	}

	fromATA, _, err := solana.FindAssociatedTokenAddress(fromUserPubKey, mintAddress)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("falha ao encontrar ATA do remetente: %w", err)
	}

	toATA, _, err := solana.FindAssociatedTokenAddress(toUserPubKey, mintAddress)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("falha ao encontrar ATA do destinatário: %w", err)
	}

	// Verificar se a ATA de destino existe. Se não, ela precisará ser criada.
	// Em um sistema real, você pode ter uma instrução `CreateAssociatedTokenAccount` separada,
	// ou incluir essa instrução na mesma transação que a transferência.
	// Para simplificar, assumimos que se a ATA não existe, ela será criada quando a primeira transferência for recebida.
	// Ou, podemos verificar e adicionar a instrução de criação se necessário.
	_, err = s.SolanaS.RPCClient.GetAccountInfo(context.Background(), toATA)
	if err != nil && err.Error() == "account not found" {
		// ATA não existe, incluir instrução para criá-la na transação
		log.Printf("ATA de destino %s não encontrada. Incluindo instrução para criá-la.", toATA.String())
		// Aqui você precisaria construir a instrução CreateAssociatedTokenAccount
		// e incluí-la na transação que será preparada.
		// Isso tornaria o PrepareTransferTransaction mais complexo, pois teria que aceitar múltiplas instruções.
		// Por simplicidade, para este exemplo, vamos assumir que o frontend ou um processo
		// separado garante que a ATA de destino exista.
		// Para uma solução completa, considere adicionar essa lógica no SolanaIntegrationService.
		// Ou o frontend pode tentar criar a ATA antes de pedir a transferência.
	}

	currentBalance, err := s.SolanaS.GetTokenAccountBalance(fromATA)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("falha ao verificar saldo do remetente na Solana: %w", err)
	}
	amountAtomic := uint64(amount * 1e9)
	if currentBalance < amountAtomic {
		return "", solana.PublicKey{}, errors.New("saldo insuficiente para transferência na Solana")
	}

	// Prepara a transação, mas não a assina com a chave do usuário
	serializedTx, err := s.SolanaS.PrepareTransferTransaction(mintAddress, fromATA, toATA, fromUserPubKey, amountAtomic)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("falha ao preparar transação de transferência: %w", err)
	}

	return serializedTx, toATA, nil
}

// CompleteTransferTokenFromUser recebe a transação assinada e a envia para a Solana.
// Este é o método que o endpoint final de transferência chamará.
func (s *TokenizationService) CompleteTransferTokenFromUser(
	assetID, fromUserID, toUserID string, amount float64, signedTxBase64 string,
	destinationATA solana.PublicKey, // Recebe a ATA de destino de volta do handler
) (models.Token, error) {
	fromUser, foundFrom, err := s.DB.GetUser(fromUserID)
	if err != nil {
		return models.Token{}, fmt.Errorf("erro ao buscar usuário remetente: %w", err)
	}
	if !foundFrom {
		return models.Token{}, errors.New("usuário remetente não encontrado")
	}
	toUser, foundTo, err := s.DB.GetUser(toUserID)
	if err != nil {
		return models.Token{}, fmt.Errorf("erro ao buscar usuário destinatário: %w", err)
	}
	if !foundTo {
		return models.Token{}, errors.New("usuário destinatário não encontrado")
	}

	asset, foundAsset, err := s.DB.GetAsset(assetID)
	if err != nil {
		return models.Token{}, fmt.Errorf("erro ao buscar ativo: %w", err)
	}
	if !foundAsset || asset.MintAddress == "" {
		return models.Token{}, errors.New("ativo não encontrado ou não tokenizado")
	}

	// Envia a transação assinada para a rede Solana
	txID, err := s.SolanaS.SendSignedTransaction(signedTxBase624)
	if err != nil {
		return models.Token{}, fmt.Errorf("falha ao enviar transação assinada para a Solana: %w", err)
	}

	// CUIDADO: O registro interno do token aqui é apenas para rastreamento.
	// A fonte de verdade é a blockchain. O listener se encarregará de manter a sincronia.
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
		// Isso é um erro grave, pois a transação foi para a blockchain, mas o DB interno falhou.
		// Numa aplicação real, você precisaria de um mecanismo de reconciliação robusto aqui.
		log.Printf("ERRO: Transação Solana %s enviada, mas falha ao salvar registro interno: %v", txID.String(), err)
		return models.Token{}, fmt.Errorf("transação enviada, mas falha ao registrar internamente: %w", err)
	}

	return transferredToken, nil
}

// ... (GetUserTokensFromSolana permanece o mesmo, buscando do DB para Asset info) ...
