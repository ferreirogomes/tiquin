package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ferreirogomes/tiquin/services"

	"github.com/go-chi/chi/v5"
)

// AssetHandler handles HTTP requests related to assets.
type AssetHandler struct {
	Service *services.TokenizationService
}

// NewAssetHandler creates a new asset handler instance.
func NewAssetHandler(s *services.TokenizationService) *AssetHandler {
	return &AssetHandler{Service: s}
}

// CreateAsset creates a new asset.
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

// GetAssetByID retrieves an asset by ID.
// GET /assets/{id}
func (h *AssetHandler) GetAssetByID(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "id")
	if assetID == "" {
		http.Error(w, "Asset ID is required", http.StatusBadRequest)
		return
	}

	asset, found, err := h.Service.DB.GetAsset(assetID)
	if err != nil {
		http.Error(w, "Error fetching asset", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Asset not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(asset)
}
