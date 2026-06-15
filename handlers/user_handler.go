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

// UserHandler handles HTTP requests related to users.
type UserHandler struct {
	DB      *storage.DB
	SolanaS *services.SolanaIntegrationService
	TokenS  *services.TokenizationService
}

// NewUserHandler creates a new user handler instance.
func NewUserHandler(db *storage.DB, solanaS *services.SolanaIntegrationService, tokenS *services.TokenizationService) *UserHandler {
	return &UserHandler{DB: db, SolanaS: solanaS, TokenS: tokenS}
}

// CreateUser creates a new user.
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
		http.Error(w, "solana_pub_key is required in Web3 standard", http.StatusBadRequest)
		return
	}

	// Verificar se usuário já existe
	existingUser, found, err := h.DB.GetUserBySolanaPubKey(requestBody.SolanaPubKey)
	if err != nil {
		http.Error(w, "Error checking existing user", http.StatusInternalServerError)
		return
	}
	if found {
		// In Web3, "Login" means connecting the wallet. If already exists, return the existing user.
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
		http.Error(w, "Error saving user to database", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// GetUserByID retrieves a user by ID.
// GET /users/{id}
func (h *UserHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	user, found, err := h.DB.GetUser(userID)
	if err != nil {
		http.Error(w, "Error fetching user", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// GetUserTokens retrieves all tokens for a user.
// GET /users/{id}/tokens
func (h *UserHandler) GetUserTokens(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	tokens, err := h.DB.GetTokensByOwnerID(userID)
	if err != nil {
		http.Error(w, "Error fetching tokens", http.StatusInternalServerError)
		return
	}
	if len(tokens) == 0 {
		// Return empty array instead of NotFound error for listing patterns
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.Token{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}
