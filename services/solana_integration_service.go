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
		fromOwnerPubKey,      // O "owner" da conta de origem é o remetente real
		[]solana.PublicKey{}, // Multisingers (nenhum neste caso)
	).Build()

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

func (s *SolanaIntegrationService) SendSignedTransaction(signedTxBase64 string) (solana.Signature, error) {
	tx, err := solana.TransactionFromBase64(signedTxBase64)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("falha ao decodificar/deserializar transação assinada: %w", err)
	}

	txID, err := s.RPCClient.SendTransactionWithOpts(context.Background(), tx, rpc.TransactionOpts{
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
func (s *SolanaIntegrationService) GetTokenAccountBalance(tokenAccountAddress solana.PublicKey) (uint64, error) {
	return 0, nil
}

func (s *SolanaIntegrationService) GetTokenSupply(mintAddress solana.PublicKey) (uint64, error) {
	return 0, nil
}
