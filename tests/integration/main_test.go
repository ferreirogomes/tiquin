package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tokenization-backend/models"
	"tokenization-backend/storage" // Para acessar o DB para limpeza
)

const (
	baseURL = "http://localhost:8080"
	dbHost  = "localhost" // Para se conectar ao DB do host para limpeza/verificação
	dbPort  = "5433"      // A porta mapeada no docker-compose.test.yml
)

var (
	testDB *storage.DB // Usaremos isso para limpar o DB entre os testes
)

// TestMain é executado antes e depois de todos os testes de integração.
func TestMain(m *testing.M) {
	// 1. Esperar o Docker Compose estar pronto
	fmt.Println("Esperando o Docker Compose estar pronto...")
	err := waitForServices()
	if err != nil {
		fmt.Printf("Serviços Docker não estão prontos: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Serviços Docker estão prontos.")

	// 2. Conectar ao banco de dados de teste para limpeza
	dbUser := os.Getenv("DB_USER_TEST")
	dbPassword := os.Getenv("DB_PASSWORD_TEST")
	dbName := os.Getenv("DB_NAME_TEST")
	dataSourceName := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		testDB, err = storage.NewDB(dataSourceName)
		if err == nil {
			break
		}
		fmt.Printf("Falha ao conectar ao DB para testes de integração, retentando... (%d/%d): %v\n", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		fmt.Printf("Falha fatal ao conectar ao DB de teste: %v\n", err)
		os.Exit(1)
	}
	defer testDB.Close()

	// 3. Executar os testes
	code := m.Run()

	// 4. Limpeza (opcional, pode ser feita por teste também)
	// cleanupDB() // Chamado antes de cada teste no setupTest

	os.Exit(code)
}

// waitForServices tenta conectar ao backend até que esteja disponível.
func waitForServices() error {
	client := http.Client{Timeout: 5 * time.Second}
	for i := 0; i < 30; i++ { // Tenta por 30 * 5s = 150 segundos
		_, err := client.Get(baseURL + "/assets") // Endpoint simples para verificar se o app está de pé
		if err == nil {
			return nil
		}
		fmt.Printf("Aguardando o serviço Go (%s)... %v\n", baseURL, err)
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("tempo limite excedido ao esperar pelos serviços")
}

// cleanupDB limpa as tabelas do banco de dados entre os testes.
func cleanupDB() {
	if testDB == nil {
		fmt.Println("testDB é nulo, pulando limpeza.")
		return
	}
	_, err := testDB.Exec(`
        DELETE FROM tokens;
        DELETE FROM assets;
        DELETE FROM users;
    `)
	if err != nil {
		fmt.Printf("Erro ao limpar o banco de dados: %v\n", err)
	} else {
		fmt.Println("Banco de dados limpo para o próximo teste.")
	}
}

// setupTest é executado antes de cada teste.
func setupTest(t *testing.T) {
	cleanupDB() // Garante um DB limpo para cada teste
}

func TestIntegration_CreateUserAndAssetAndTransfer(t *testing.T) {
	setupTest(t)

	// --- 1. Criar Usuário 1 (Remetente) ---
	user1PubKey := "8jS7zR2gW4F9H8T7U6V5B4A3D2E1F0G9H8I7J6K5L4M3N2O1P0Q9R8S7T6U5V4W3X2Y1Z0" // Gerar um novo para cada teste
	user1Req := map[string]string{
		"name":           "Integration User 1",
		"email":          "user1@integration.com",
		"solana_pub_key": user1PubKey,
	}
	user1Body, _ := json.Marshal(user1Req)
	resp, err := http.Post(baseURL+"/users", "application/json", bytes.NewBuffer(user1Body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var user1 models.User
	err = json.NewDecoder(resp.Body).Decode(&user1)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, user1.ID)
	assert.Equal(t, user1PubKey, user1.SolanaPubKey)

	// --- 2. Criar Usuário 2 (Destinatário) ---
	user2PubKey := "9kT8vU1aB3C4D5E6F7G8H9I0J1K2L3M4N5O6P7Q8R9S0T1U2V3W4X5Y6Z7A8B9C0D1E2"
	user2Req := map[string]string{
		"name":           "Integration User 2",
		"email":          "user2@integration.com",
		"solana_pub_key": user2PubKey,
	}
	user2Body, _ := json.Marshal(user2Req)
	resp, err = http.Post(baseURL+"/users", "application/json", bytes.NewBuffer(user2Body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var user2 models.User
	err = json.NewDecoder(resp.Body).Decode(&user2)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, user2.ID)
	assert.Equal(t, user2PubKey, user2.SolanaPubKey)

	// --- 3. Criar Ativo (Tokenizar na Solana) ---
	assetReq := map[string]interface{}{
		"symbol":                "GOOG",
		"name":                  "Google LLC",
		"total_shares":          100000.0,
		"initial_owner_user_id": user1.ID,
	}
	assetBody, _ := json.Marshal(assetReq)
	resp, err = http.Post(baseURL+"/assets", "application/json", bytes.NewBuffer(assetBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var asset models.Asset
	err = json.NewDecoder(resp.Body).Decode(&asset)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, asset.ID)
	assert.Equal(t, "GOOG", asset.Symbol)
	assert.NotEmpty(t, asset.MintAddress) // MintAddress deve ser preenchido pela Solana

	// Dê um tempo para a transação Solana ser confirmada e o listener processar
	time.Sleep(10 * time.Second)

	// --- 4. Verificar Tokens do Usuário 1 (Saldo Inicial) ---
	resp, err = http.Get(baseURL + "/users/" + user1.ID + "/tokens")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var user1Tokens []models.Token
	err = json.NewDecoder(resp.Body).Decode(&user1Tokens)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, user1Tokens)
	// Encontre o token GOOG para user1
	var googTokenUser1 models.Token
	for _, tkn := range user1Tokens {
		if tkn.AssetID == asset.ID {
			googTokenUser1 = tkn
			break
		}
	}
	assert.NotNil(t, googTokenUser1)
	assert.InDelta(t, 100000.0, googTokenUser1.Amount, 0.000001)

	// --- 5. Preparar Transferência (Backend) ---
	transferPrepareReq := handlers.PrepareTransferRequest{
		AssetID:    asset.ID,
		FromUserID: user1.ID,
		ToUserID:   user2.ID,
		Amount:     50.0,
	}
	prepareBody, _ := json.Marshal(transferPrepareReq)
	resp, err = http.Post(baseURL+"/tokens/transfer/prepare", "application/json", bytes.NewBuffer(prepareBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var prepareResp handlers.PrepareTransferResponse
	err = json.NewDecoder(resp.Body).Decode(&prepareResp)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, prepareResp.SerializedTransaction)
	assert.NotEmpty(t, prepareResp.DestinationATA)

	// --- 6. Simular Assinatura no Frontend (Apenas para Teste de Integração) ---
	// Em um cenário real, esta seria a parte do frontend/carteira do usuário
	// Aqui, vamos decodificar a transação, adicionar a assinatura do user1PubKey
	// (usando uma chave privada fictícia/de teste se necessário, ou usar o FeePayer para simplificar este teste)
	// NOTA: Para este teste, vamos SIMPLIFICAR e assumir que o FeePayer pode assinar.
	// Em um teste mais rigoroso, você precisaria de uma chave privada mock do user1PubKey.
	// Ou seja, o backend não deveria ter a chave privada do user1.

	// Para fins de teste de integração, onde não temos um frontend para assinar,
	// vamos assinar a transação que o backend preparou com a chave do FeePayer
	// (o que é uma simplificação para o teste, mas não é o fluxo de segurança real)
	feePayerPrivateKeyStr := os.Getenv("SOLANA_FEE_PAYER_PRIVATE_KEY_TEST")
	feePayerPK, err := solana.PrivateKeyFromBase58(feePayerPrivateKeyStr)
	require.NoError(t, err, "Falha ao carregar SOLANA_FEE_PAYER_PRIVATE_KEY_TEST para teste")

	txBytes, err := base64.StdEncoding.DecodeString(prepareResp.SerializedTransaction)
	require.NoError(t, err)
	var tx solana.Transaction
	err = tx.UnmarshalBinary(txBytes)
	require.NoError(t, err)

	// Adicionar a assinatura do remetente (simulado)
	// Se a transação já foi assinada pelo FeePayer (como no prepare do nosso backend),
	// você precisa adicionar a assinatura do user1PubKey aqui.
	// Isso exigiria que `tx` fosse construída para ter um `Signature` slice vazio
	// e o `FeePayer` e o `fromUserPubKey` fossem adicionados como signers.
	// Para simplificar, vou re-criar a transação do zero com a instrução e assinar com os dois.
	// Isso é uma gambiarra para o teste de integração sem frontend real.

	// A maneira "correta" seria o backend preparar a transação SEM assinaturas (ou apenas o fee payer parcial),
	// e então o frontend a receberia e adicionaria a assinatura do usuário.
	// Aqui, para o teste, vamos re-construir e assinar para simular o "frontend assinado".
	fromUserPubKeySol, _ := solana.PublicKeyFromBase58(user1PubKey)
	toUserPubKeySol, _ := solana.PublicKeyFromBase58(user2PubKey)
	mintAddressSol, _ := solana.PublicKeyFromBase58(asset.MintAddress)
	fromATASol, _, _ := solana.FindAssociatedTokenAddress(fromUserPubKeySol, mintAddressSol)
	toATASol, _, _ := solana.FindAssociatedTokenAddress(toUserPubKeySol, mintAddressSol)

	recentBlockhashResp, err := testDB.RPCClient.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized) // Precisa de um rpc client aqui.
	// Melhor mockar isso ou passar um serviço real para o teste se for usar RPC.
	// Ou então, assuma que o backend já tem o blockhash.
	// Para este teste, vamos mockar uma resposta de blockhash ou usar uma fixa.
	// Vou usar a que o prepareResp.SerializedTransaction já conteria.
	// No entanto, para assinar novamente, precisamos de um blockhash válido.
	// Vou pegar do RPC direto para o teste.
	rpcClientForTest := rpc.New(os.Getenv("SOLANA_RPC_URL_TEST"))
	blockhashResp, err := rpcClientForTest.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	require.NoError(t, err)
	recentBlockhash := blockhashResp.Value.Blockhash

	transferInstruction := token.NewTransferInstruction(
		uint64(50.0*1e9), // Amount em unidades atômicas
		fromATASol,
		toATASol,
		fromUserPubKeySol,
	).SetProgramID(token.ProgramID).Build()

	simulatedSignedTx, err := solana.NewTransaction(
		[]solana.Instruction{
			transferInstruction,
		},
		recentBlockhash,
		solana.TransactionPayer(feePayerPK.PublicKey()),
	)
	require.NoError(t, err)

	_, err = simulatedSignedTx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			switch key {
			case feePayerPK.PublicKey():
				return &feePayerPK
			case fromUserPubKeySol:
				// Em um cenário real, aqui viria a chave privada do usuário.
				// Para o teste, usamos a chave do FeePayer como se fosse a do usuário para assinar.
				// Isso não é seguro para produção.
				return &feePayerPK // Assinamos com o FeePayer novamente
			}
			return nil
		},
	)
	require.NoError(t, err)
	signedTxBase64Simulated, err := simulatedSignedTx.MarshalBinary()
	require.NoError(t, err)
	signedTxBase64Encoded := base64.StdEncoding.EncodeToString(signedTxBase64Simulated)

	// --- 7. Completar Transferência (Backend) ---
	transferCompleteReq := handlers.CompleteTransferRequest{
		AssetID:           asset.ID,
		FromUserID:        user1.ID,
		ToUserID:          user2.ID,
		Amount:            50.0,
		SignedTransaction: signedTxBase64Encoded, // Usar a transação simulada assinada
		DestinationATA:    prepareResp.DestinationATA,
	}
	completeBody, _ := json.Marshal(transferCompleteReq)
	resp, err = http.Post(baseURL+"/tokens/transfer/complete", "application/json", bytes.NewBuffer(completeBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var transferResp models.Token
	err = json.NewDecoder(resp.Body).Decode(&transferResp)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, user2.ID, transferResp.OwnerID)
	assert.InDelta(t, 50.0, transferResp.Amount, 0.000001)
	assert.NotEmpty(t, transferResp.TransactionID) // Deve ter um ID de transação Solana

	// Dê um tempo para a transação Solana ser confirmada e o listener processar
	time.Sleep(10 * time.Second)

	// --- 8. Verificar Tokens do Usuário 1 (Saldo Após Transferência) ---
	resp, err = http.Get(baseURL + "/users/" + user1.ID + "/tokens")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	err = json.NewDecoder(resp.Body).Decode(&user1Tokens)
	require.NoError(t, err)
	resp.Body.Close()
	var finalUser1GoogToken models.Token
	for _, tkn := range user1Tokens {
		if tkn.AssetID == asset.ID {
			finalUser1GoogToken = tkn
			break
		}
	}
	assert.NotNil(t, finalUser1GoogToken)
	assert.InDelta(t, 99950.0, finalUser1GoogToken.Amount, 0.000001) // 100000 - 50 = 99950

	// --- 9. Verificar Tokens do Usuário 2 (Saldo Após Transferência) ---
	resp, err = http.Get(baseURL + "/users/" + user2.ID + "/tokens")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var user2Tokens []models.Token
	err = json.NewDecoder(resp.Body).Decode(&user2Tokens)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, user2Tokens)
	var finalUser2GoogToken models.Token
	for _, tkn := range user2Tokens {
		if tkn.AssetID == asset.ID {
			finalUser2GoogToken = tkn
			break
		}
	}
	assert.NotNil(t, finalUser2GoogToken)
	assert.InDelta(t, 50.0, finalUser2GoogToken.Amount, 0.000001)
}

// Pequena alteração no services/solana_integration_service.go para expor RPCClient para o TestMain
// Isso é uma gambiarra para que o teste de integração possa usar o RPC client.
// Em um sistema real, o RPCClient não seria exposto assim.
/*
type SolanaIntegrationService struct {
	RPCClient *rpc.Client // Torne-o público para o teste
	DB        *storage.DB
	FeePayer  solana.PrivateKey
}
*/
