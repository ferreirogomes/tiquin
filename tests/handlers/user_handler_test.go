package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock" // Para mocks

	"tokenization-backend/handlers"
	"tokenization-backend/models"
	"tokenization-backend/services"
	"tokenization-backend/storage"
)

// MockDB é uma implementação mock do storage.DB para testes de unidade
type MockDB struct {
	mock.Mock
}

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

// TestCreateUser testa a criação de um usuário
func TestCreateUser(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	mockTokenS := services.NewTokenizationService(mockDB, mockSolanaS) // Crie uma instância real do serviço tokenizationService

	userHandler := handlers.NewUserHandler(mockDB, mockSolanaS, mockTokenS)

	newUser := models.User{
		Name:         "Test User",
		Email:        "test@example.com",
		SolanaPubKey: "GnL5gP5tK25fN4W32L54wN92p24fJ84tJ62dK2s8S7b", // Exemplo de chave pública
	}

	mockDB.On("SaveUser", mock.AnythingOfType("models.User")).Return(nil).Once()

	body, _ := json.Marshal(newUser)
	req := httptest.NewRequest("POST", "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/users", userHandler.CreateUser)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var createdUser models.User
	err := json.Unmarshal(rr.Body.Bytes(), &createdUser)
	assert.Nil(t, err)
	assert.NotEmpty(t, createdUser.ID)
	assert.Equal(t, newUser.Name, createdUser.Name)
	assert.Equal(t, newUser.Email, createdUser.Email)
	assert.Equal(t, newUser.SolanaPubKey, createdUser.SolanaPubKey)

	mockDB.AssertExpectations(t)
}

// TestGetUserByID testa a obtenção de um usuário por ID
func TestGetUserByID(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	mockTokenS := services.NewTokenizationService(mockDB, mockSolanaS)

	userHandler := handlers.NewUserHandler(mockDB, mockSolanaS, mockTokenS)

	existingUser := models.User{
		ID:           "123",
		Name:         "Existing User",
		Email:        "existing@example.com",
		SolanaPubKey: "GnL5gP5tK25fN4W32L54wN92p24fJ84tJ62dK2s8S7b",
		CreatedAt:    time.Now(),
	}

	mockDB.On("GetUser", "123").Return(existingUser, true, nil).Once()

	req := httptest.NewRequest("GET", "/users/123", nil)
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/users/{id}", userHandler.GetUserByID)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var retrievedUser models.User
	err := json.Unmarshal(rr.Body.Bytes(), &retrievedUser)
	assert.Nil(t, err)
	assert.Equal(t, existingUser.ID, retrievedUser.ID)
	assert.Equal(t, existingUser.Name, retrievedUser.Name)

	mockDB.AssertExpectations(t)
}

// TestGetUserTokens testa a obtenção de tokens de um usuário (via Solana)
func TestGetUserTokens(t *testing.T) {
	mockDB := new(MockDB)
	mockSolanaS := new(MockSolanaIntegrationService)
	mockTokenS := services.NewTokenizationService(mockDB, mockSolanaS)

	userHandler := handlers.NewUserHandler(mockDB, mockSolanaS, mockTokenS)

	userID := "test-user-id"
	userPubKey := "7jP2k12Xg5S9Q4L8R7T6V5B4A3D2E1F0G9H8I7J6K5L4M3N2O1P0Q9R8S7T6U5V4W3X2Y1Z0" // Exemplo de chave pública
	mockUser := models.User{
		ID:           userID,
		Name:         "Test User",
		Email:        "test@example.com",
		SolanaPubKey: userPubKey,
	}
	mockDB.On("GetUser", userID).Return(mockUser, true, nil).Once()

	mockToken := []models.Token{
		{
			AssetID:             "asset-1",
			OwnerID:             userID,
			Amount:              100.0,
			MintAddress:         "MockMintAddress123",
			TokenAccountAddress: "MockTokenAccountAddress456",
		},
	}
	// Mock do método GetUserTokensFromSolana do TokenizationService
	mockTokenS.On("GetUserTokensFromSolana", userID).Return(mockToken, nil).Once()

	req := httptest.NewRequest("GET", "/users/"+userID+"/tokens", nil)
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/users/{id}/tokens", userHandler.GetUserTokens)
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var tokens []models.Token
	err := json.Unmarshal(rr.Body.Bytes(), &tokens)
	assert.Nil(t, err)
	assert.Len(t, tokens, 1)
	assert.Equal(t, mockToken[0].Amount, tokens[0].Amount)

	mockDB.AssertExpectations(t)
	mockSolanaS.AssertExpectations(t) // Verifique também mocks do serviço Solana
	// mockTokenS.AssertExpectations(t) // Se o mockTokenS fosse um mock, você chamaria isso
}
