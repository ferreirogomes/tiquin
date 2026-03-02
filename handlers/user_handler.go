package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ferreirogomes/tiquin/models"

	"github.com/ferreirogomes/tiquin/services"
	"github.com/ferreirogomes/tiquin/storage" // Use real storage

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UserHandler lida com requisições HTTP relacionadas a usuários.
type UserHandler struct {
	DB      *storage.DB
	SolanaS *services.SolanaIntegrationService
	TokenS  *services.TokenizationService
}

// NewUserHandler cria uma nova instância do handler de usuários.
func NewUserHandler(db *storage.DB, solanaS *services.SolanaIntegrationService, tokenS *services.TokenizationService) *UserHandler {
	return &UserHandler{DB: db, SolanaS: solanaS, TokenS: tokenS}
}

// CreateUser cria um novo usuário.
// POST /users
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		Name         *string `json:"name,omitempty"`
		Email        *string `json:"email,omitempty"`
		SolanaPubKey string  `json:"solana_pub_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if requestBody.SolanaPubKey == "" {
		http.Error(w, "solana_pub_key é obrigatória no padrão Web3", http.StatusBadRequest)
		return
	}

	// Verificar se usuário já existe
	existingUser, found, err := h.DB.GetUserBySolanaPubKey(requestBody.SolanaPubKey)
	if err != nil {
		http.Error(w, "Erro ao verificar usuário existente", http.StatusInternalServerError)
		return
	}
	if found {
		// No Web3 "Login" é conectar a carteira. Se já existe, retorna o usuário existente.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(existingUser)
		return
	}

	user := models.User{
		ID:           uuid.New().String(),
		Name:         requestBody.Name,
		Email:        requestBody.Email,
		SolanaPubKey: requestBody.SolanaPubKey,
		CreatedAt:    time.Now(),
	}

	err = h.DB.SaveUser(user)
	if err != nil {
		http.Error(w, "Erro ao salvar usuário no banco de dados", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// GetUserByID obtém um usuário pelo ID.
// GET /users/{id}
func (h *UserHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "ID do usuário é obrigatório", http.StatusBadRequest)
		return
	}

	user, found, err := h.DB.GetUser(userID)
	if err != nil {
		http.Error(w, "Erro ao buscar usuário", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Usuário não encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// GetUserTokens obtém todos os tokens de um usuário.
// GET /users/{id}/tokens
func (h *UserHandler) GetUserTokens(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "ID do usuário é obrigatório", http.StatusBadRequest)
		return
	}

	tokens, err := h.DB.GetTokensByOwnerID(userID)
	if err != nil {
		http.Error(w, "Erro ao buscar tokens", http.StatusInternalServerError)
		return
	}
	if len(tokens) == 0 {
		// Retornar array vazio em invés de erro NotFound para padrão de listagens
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.Token{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}
