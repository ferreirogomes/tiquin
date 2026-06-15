package handlers

import (
	"encoding/json"
	"fmt" // For fmt.Errorf
	"net/http"

	"github.com/ferreirogomes/tiquin/services"
	"github.com/go-chi/chi/v5"

	"github.com/gagliardetto/solana-go" // For PublicKey
)

type TokenHandler struct {
	Service *services.TokenizationService
}

func NewTokenHandler(s *services.TokenizationService) *TokenHandler {
	return &TokenHandler{Service: s}
}

// Request struct for transfer preparation
type PrepareTransferRequest struct {
	AssetID    string  `json:"asset_id"`
	FromUserID string  `json:"from_user_id"`
	ToUserID   string  `json:"to_user_id"`
	Amount     float64 `json:"amount"`
}

// Response struct for transfer preparation
type PrepareTransferResponse struct {
	SerializedTransaction string `json:"serialized_transaction"` // Transaction in Base64 for signing
	DestinationATA        string `json:"destination_ata"`        // Destination ATA address
}

// PrepareTransfer prepares a transfer transaction for user signing.
// POST /tokens/transfer/prepare
func (h *TokenHandler) PrepareTransfer(w http.ResponseWriter, r *http.Request) {
	var req PrepareTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	serializedTx, destATA, err := h.Service.PrepareTransferTokenFromUser(
		req.AssetID, req.FromUserID, req.ToUserID, req.Amount,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := PrepareTransferResponse{
		SerializedTransaction: serializedTx,
		DestinationATA:        destATA.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Request struct for completing the transfer
type CompleteTransferRequest struct {
	AssetID           string  `json:"asset_id"`
	FromUserID        string  `json:"from_user_id"`
	ToUserID          string  `json:"to_user_id"`
	Amount            float64 `json:"amount"`
	SignedTransaction string  `json:"signed_transaction"` // Transaction signed by the user (Base64)
	DestinationATA    string  `json:"destination_ata"`    // Destination ATA, passed back by the frontend
}

// CompleteTransfer sends the signed transfer transaction to Solana.
// POST /tokens/transfer/complete
func (h *TokenHandler) CompleteTransfer(w http.ResponseWriter, r *http.Request) {
	var req CompleteTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	destATA, err := solana.PublicKeyFromBase58(req.DestinationATA)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid destination ATA address: %v", err), http.StatusBadRequest)
		return
	}

	token, err := h.Service.CompleteTransferTokenFromUser(
		req.AssetID, req.FromUserID, req.ToUserID, req.Amount, req.SignedTransaction, destATA,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

// GetTokenByID retrieves a token by ID
// GET /tokens/{id}
func (h *TokenHandler) GetTokenByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	token, found, err := h.Service.DB.GetToken(id)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

// GetTokensByAssetID retrieves all tokens for a given asset
// GET /tokens/by-asset/{assetID}
func (h *TokenHandler) GetTokensByAssetID(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "assetID")
	if assetID == "" {
		http.Error(w, "Asset ID is required", http.StatusBadRequest)
		return
	}

	tokens, err := h.Service.DB.GetTokensByAssetID(assetID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// ... (GetTokenByID, GetTokensByAssetID remain unchanged) ...
