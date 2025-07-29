package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"tiquin/data"
	"tiquin/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UserHandler lida com requisições HTTP relacionadas a usuários.
type UserHandler struct {
	DB *data.MockDB // Direto para o DB mock para simplificar a criação de usuários
}

// NewUserHandler cria uma nova instância do handler de usuários.
func NewUserHandler(db *data.MockDB) *UserHandler {
	return &UserHandler{DB: db}
}

// CreateUser cria um novo usuário.
// POST /users
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if requestBody.Name == "" || requestBody.Email == "" {
		http.Error(w, "Nome e email são obrigatórios", http.StatusBadRequest)
		return
	}

	user := models.User{
		ID:        uuid.New().String(),
		Name:      requestBody.Name,
		Email:     requestBody.Email,
		CreatedAt: time.Now(),
	}

	h.DB.SaveUser(user)

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

	user, found := h.DB.GetUser(userID)
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

	tokens := h.DB.GetTokensByOwnerID(userID)
	if len(tokens) == 0 {
		http.Error(w, "Nenhum token encontrado para este usuário", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}
