package services_test

import (
	"testing"

	"github.com/ferreirogomes/tiquin/models"
	"github.com/ferreirogomes/tiquin/services"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	// Usar storage para mocks
)

// MockDB é uma implementação mock do storage.DB para testes de unidade
type MockDB struct {
	mock.Mock
}

// ... (Implementar todos os métodos de storage.DB com mock.Called e Retorno) ...
func (m *MockDB) SaveUser(user models.User) error {
	args := m.Called(user)
	return args.Error(0)
}
func (m *MockDB) GetUser(id string) (models.User, bool, error) {
	args := m.Called(id)
	return args.Get(0).(models.User), args.Bool(1), args.Error(2)
}
func (m *MockDB) GetUserBySolanaPubKey(pubKey string) (models.User, bool, error) {
	args := m.Called(pubKey)
	return args.Get(0).(models.User), args.Bool(1), args.Error(2)
}
func (m *MockDB) SaveAsset(asset models.Asset) error {
	args := m.Called(asset)
	return args.Error(0)
}
func (m *MockDB) GetAsset(id string) (models.Asset, bool, error) {
	args := m.Called(id)
	return args.Get(0).(models.Asset), args.Bool(1), args.Error(2)
}
func (m *MockDB) GetAssetByMintAddress(mintAddress string) (models.Asset, bool, error) {
	args := m.Called(mintAddress)
	return args.Get(0).(models.Asset), args.Bool(1), args.Error(2)
}
func (m *MockDB) SaveToken(token models.Token) error {
	args := m.Called(token)
	return args.Error(0)
}
func (m *MockDB) GetToken(id string) (models.Token, bool, error) {
	args := m.Called(id)
	return args.Get(0).(models.Token), args.Bool(1), args.Error(2)
}
func (m *MockDB) GetTokensByAssetID(assetID string) ([]models.Token, error) {
	args := m.Called(assetID)
	return args.Get(0).([]models.Token), args.Error(1)
}
func (m *MockDB) GetTokensByOwnerID(ownerID string) ([]models.Token, error) {
	args := m.Called(ownerID)
	return args.Get(0).([]models.Token), args.Error(1)
}
func (m *MockDB) UpdateToken(token models.Token) error {
	args := m.Called(token)
	return args.Error(0)
}

// MockSolanaIntegrationService é uma implementação mock do services.SolanaIntegrationService
type MockSolanaIntegrationService struct {
	mock.Mock
}

// ... (Implementar todos os métodos de services.SolanaIntegrationService com mock.Called e Retorno) ...
func (m *MockSolanaIntegrationService) CreateMintAndTokenAccount(ownerPubKey solana.PublicKey, assetSymbol string) (solana.PublicKey, solana.PublicKey, error) {
	args := m.Called(ownerPubKey, assetSymbol)
	return args.Get(0).(solana.PublicKey), args.Get(1).(solana.PublicKey), args.Error(2)
}
func (m *MockSolanaIntegrationService) MintTokensToAccount(mintAddress, destinationATA solana.PublicKey, amount uint64) (solana.Signature, error) {
	args := m.Called(mintAddress, destinationATA, amount)
	return args.Get(0).(solana.Signature), args.Error(1)
}
func (m *MockSolanaIntegrationService) PrepareTransferTransaction(mintAddress, fromATA, toATA, fromOwnerPubKey solana.PublicKey, amount uint64) (string, error) {
	args := m.Called(mintAddress, fromATA, toATA, fromOwnerPubKey, amount)
	return args.String(0), args.Error(1)
}
func (m *MockSolanaIntegrationService) SendSignedTransaction(signedTxBase64 string) (solana.Signature, error) {
	args := m.Called(signedTxBase64)
	return args.Get(0).(solana.Signature), args.Error(1)
}
func (m *MockSolanaIntegrationService) GetTokenAccountBalance(tokenAccountAddress solana.PublicKey) (uint64, error) {
	args := m.Called(tokenAccountAddress)
	return args.Get(0).(uint64), args.Error(1)
}
func (m *MockSolanaIntegrationService) GetTokenSupply(mintAddress solana.PublicKey) (uint64, error) {
	args := m.Called(mintAddress)
	return args.Get(0).(uint64), args.Error(1)
}

// TestCreateAsset verifica a criação de um ativo e tokenização
func TestCreateAsset(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	service := services.NewTokenizationService(mockDB, mockSolanaS)

	ownerID := "user-123"
	ownerPubKey := solana.MustPublicKeyFromBase58("7jP2k12Xg5S9Q4L8R7T6V5B4A3D2E1F0G9H8I7J6K5L4M3N2O1P0Q9R8S7T6U5V4W3X2Y1Z0")
	mintAddr := solana.NewWallet().PublicKey()
	ataAddr := solana.NewWallet().PublicKey()
	txID := solana.NewWallet().PublicKey() // Simular uma Signature

	mockDB.On("GetUser", ownerID).Return(models.User{ID: ownerID, SolanaPubKey: ownerPubKey.String()}, true, nil).Once()
	mockSolanaS.On("CreateMintAndTokenAccount", ownerPubKey, "TSLA").Return(mintAddr, ataAddr, nil).Once()
	mockDB.On("SaveAsset", mock.AnythingOfType("models.Asset")).Return(nil).Once()
	mockSolanaS.On("MintTokensToAccount", mintAddr, ataAddr, mock.AnythingOfType("uint64")).Return(solana.SignatureFromBytes(txID.Bytes()), nil).Once()
	mockDB.On("SaveToken", mock.AnythingOfType("models.Token")).Return(nil).Once()

	asset, err := service.CreateAsset("TSLA", "Tesla Inc.", 1000.0, ownerID)

	assert.Nil(t, err)
	assert.NotEmpty(t, asset.ID)
	assert.Equal(t, "TSLA", asset.Symbol)
	assert.Equal(t, mintAddr.String(), asset.MintAddress)

	mockDB.AssertExpectations(t)
	mockSolanaS.AssertExpectations(t)
}

// TestPrepareTransferTokenFromUser verifica a preparação da transação
func TestPrepareTransferTokenFromUser(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	service := services.NewTokenizationService(mockDB, mockSolanaS)

	fromUserID := "user-from"
	toUserID := "user-to"
	assetID := "asset-xyz"
	amount := 10.0

	fromUserPubKey := solana.NewWallet().PublicKey()
	toUserPubKey := solana.NewWallet().PublicKey()
	mintAddress := solana.NewWallet().PublicKey()
	fromATA, _, _ := solana.FindAssociatedTokenAddress(fromUserPubKey, mintAddress)
	toATA, _, _ := solana.FindAssociatedTokenAddress(toUserPubKey, mintAddress)

	// Configurar mocks
	mockDB.On("GetUser", fromUserID).Return(models.User{ID: fromUserID, SolanaPubKey: fromUserPubKey.String()}, true, nil).Once()
	mockDB.On("GetUser", toUserID).Return(models.User{ID: toUserID, SolanaPubKey: toUserPubKey.String()}, true, nil).Once()
	mockDB.On("GetAsset", assetID).Return(models.Asset{ID: assetID, MintAddress: mintAddress.String()}, true, nil).Once()
	mockSolanaS.On("GetTokenAccountBalance", fromATA).Return(uint64(20*1e9), nil).Once() // Saldo suficiente
	mockSolanaS.On("PrepareTransferTransaction", mintAddress, fromATA, toATA, fromUserPubKey, uint64(amount*1e9)).Return("serialized_tx_base64", nil).Once()

	serializedTx, destATA, err := service.PrepareTransferTokenFromUser(assetID, fromUserID, toUserID, amount)

	assert.Nil(t, err)
	assert.Equal(t, "serialized_tx_base64", serializedTx)
	assert.Equal(t, toATA, destATA)

	mockDB.AssertExpectations(t)
	mockSolanaS.AssertExpectations(t)
}

// TestCompleteTransferTokenFromUser verifica o envio da transação assinada
func TestCompleteTransferTokenFromUser(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	service := services.NewTokenizationService(mockDB, mockSolanaS)

	fromUserID := "user-from"
	toUserID := "user-to"
	assetID := "asset-xyz"
	amount := 10.0
	signedTxBase64 := "signed_tx_base64"
	destATA := solana.NewWallet().PublicKey()
	txID := solana.NewWallet().PublicKey()

	// Configurar mocks
	mockDB.On("GetUser", fromUserID).Return(models.User{ID: fromUserID}, true, nil).Once()
	mockDB.On("GetUser", toUserID).Return(models.User{ID: toUserID}, true, nil).Once()
	mockDB.On("GetAsset", assetID).Return(models.Asset{ID: assetID, MintAddress: solana.NewWallet().PublicKey().String(), Name: "Test Asset"}, true, nil).Once()
	mockSolanaS.On("SendSignedTransaction", signedTxBase664).Return(solana.SignatureFromBytes(txID.Bytes()), nil).Once()
	mockDB.On("SaveToken", mock.AnythingOfType("models.Token")).Return(nil).Once()

	token, err := service.CompleteTransferTokenFromUser(assetID, fromUserID, toUserID, amount, signedTxBase64, destATA)

	assert.Nil(t, err)
	assert.Equal(t, amount, token.Amount)
	assert.Equal(t, toUserID, token.OwnerID)
	assert.Equal(t, txID.String(), token.TransactionID)

	mockDB.AssertExpectations(t)
	mockSolanaS.AssertExpectations(t)
}

// TestPrepareTransferTokenInsufficientBalance verifica saldo insuficiente
func TestPrepareTransferTokenInsufficientBalance(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	service := services.NewTokenizationService(mockDB, mockSolanaS)

	fromUserID := "user-from"
	toUserID := "user-to"
	assetID := "asset-xyz"
	amount := 10.0

	fromUserPubKey := solana.NewWallet().PublicKey()
	toUserPubKey := solana.NewWallet().PublicKey()
	mintAddress := solana.NewWallet().PublicKey()
	fromATA, _, _ := solana.FindAssociatedTokenAddress(fromUserPubKey, mintAddress)
	toATA, _, _ := solana.FindAssociatedTokenAddress(toUserPubKey, mintAddress)

	// Configurar mocks para falha de saldo
	mockDB.On("GetUser", fromUserID).Return(models.User{ID: fromUserID, SolanaPubKey: fromUserPubKey.String()}, true, nil).Once()
	mockDB.On("GetUser", toUserID).Return(models.User{ID: toUserID, SolanaPubKey: toUserPubKey.String()}, true, nil).Once()
	mockDB.On("GetAsset", assetID).Return(models.Asset{ID: assetID, MintAddress: mintAddress.String()}, true, nil).Once()
	mockSolanaS.On("GetTokenAccountBalance", fromATA).Return(uint64(5*1e9), nil).Once() // Saldo INSUFICIENTE

	_, _, err := service.PrepareTransferTokenFromUser(assetID, fromUserID, toUserID, amount)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "saldo insuficiente")

	mockDB.AssertExpectations(t)
	mockSolanaS.AssertExpectations(t)
}
