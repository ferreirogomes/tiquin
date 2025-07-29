package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	// Mudar de data para storage
)

// ... (SolanaIntegrationService struct e NewSolanaIntegrationService permanecem os mesmos) ...

// CreateMintAndTokenAccount permanece o mesmo (ainda assinado pelo FeePayer)
// MintTokensToAccount permanece o mesmo (ainda assinado pelo FeePayer)

// PrepareTransferTransaction serializa uma transação de transferência para assinatura pelo usuário.
// Esta função CONSTRÓI a transação, mas NÃO a ASSINA com a chave privada do remetente.
// O FeePayer paga as taxas de rede.
func (s *SolanaIntegrationService) PrepareTransferTransaction(
	mintAddress, fromATA, toATA solana.PublicKey,
	fromOwnerPubKey solana.PublicKey, // Public key do remetente real
	amount uint64,
) (string, error) { // Retorna a transação codificada em Base64
	resp, err := s.RPCClient.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("falha ao obter blockhash: %w", err)
	}
	recentBlockhash := resp.Value.Blockhash

	// Instrução para transferir tokens
	transferInstruction := token.NewTransferInstruction(
		amount,
		fromATA,
		toATA,
		fromOwnerPubKey, // O "owner" da conta de origem é o remetente real
	).SetProgramID(token.ProgramID).Build()

	// O FeePayer paga a taxa da transação
	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			transferInstruction,
		},
		recentBlockhash,
		solana.TransactionPayer(s.FeePayer.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("falha ao criar transação de transferência: %w", err)
	}

	// O FeePayer PRECISA assinar, pois ele é o pagador da transação
	// O fromOwnerPubKey (remetente) assinará no frontend
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(s.FeePayer.PublicKey()) {
			return &s.FeePayer
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("falha ao assinar transação pelo FeePayer: %w", err)
	}

	// Serializar a transação para ser enviada ao cliente
	serializedTx, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("falha ao serializar transação: %w", err)
	}

	return base64.StdEncoding.EncodeToString(serializedTx), nil
}

// SendSignedTransaction recebe uma transação já assinada e a envia para a rede.
func (s *SolanaIntegrationService) SendSignedTransaction(signedTxBase64 string) (solana.Signature, error) {
	signedTxBytes, err := base64.StdEncoding.DecodeString(signedTxBase64)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("falha ao decodificar transação assinada: %w", err)
	}

	var tx solana.Transaction
	if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
		return solana.Signature{}, fmt.Errorf("falha ao deserializar transação: %w", err)
	}

	txID, err := s.RPCClient.SendTransactionWithOpts(context.Background(), &tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("falha ao enviar transação assinada: %w", err)
	}
	log.Printf("Transação assinada enviada: %s\n", txID)

	// Aguardar confirmação (opcional, mas recomendado para operações críticas)
	_, err = s.RPCClient.GetSignatureStatuses(context.Background(), true, txID)
	if err != nil {
		log.Printf("Erro ao verificar status da transação %s: %v\n", txID, err)
	} else {
		log.Printf("Transação %s confirmada.\n", txID)
	}

	return txID, nil
}

// ... (GetTokenAccountBalance, GetTokenSupply permanecem os mesmos) ...
