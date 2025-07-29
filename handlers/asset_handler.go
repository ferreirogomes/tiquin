package handlers

import (
	"encoding/json"
	"net/http"

	"tiquin/services"

	"github.com/go-chi/chi/v5"
)

// AssetHandler lida com requisições HTTP relacionadas a ativos.
type AssetHandler struct {
	Service *services.TokenizationService
}

// NewAssetHandler cria uma nova instância do handler de ativos.
func NewAssetHandler(s *services.TokenizationService) *AssetHandler {
	return &AssetHandler{Service: s}
}

// CreateAsset cria um novo ativo.
// POST /assets
func (h *AssetHandler) CreateAsset(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		Symbol      string  `json:"symbol"`
		Name        string  `json:"name"`
		TotalShares float64 `json:"total_shares"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	asset, err := h.Service.CreateAsset(requestBody.Symbol, requestBody.Name, requestBody.TotalShares)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(asset)
}

// GetAssetByID obtém um ativo pelo ID.
// GET /assets/{id}
func (h *AssetHandler) GetAssetByID(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "id")
	if assetID == "" {
		http.Error(w, "ID do ativo é obrigatório", http.StatusBadRequest)
		return
	}

	asset, found := h.Service.DB.GetAsset(assetID)
	if !found {
		http.Error(w, "Ativo não encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(asset)
}
