package handlers

import (
	"encoding/json"
	"fmt" // Para fmt.Errorf
	"net/http"

	"tokenization-backend/services"

	"github.com/gagliardetto/solana-go" // Para PublicKey
)

type TokenHandler struct {
	Service *services.TokenizationService
}

func NewTokenHandler(s *services.TokenizationService) *TokenHandler {
	return &TokenHandler{Service: s}
}

// Request struct para a preparação da transferência
type PrepareTransferRequest struct {
	AssetID    string  `json:"asset_id"`
	FromUserID string  `json:"from_user_id"`
	ToUserID   string  `json:"to_user_id"`
	Amount     float64 `json:"amount"`
}

// Response struct para a preparação da transferência
type PrepareTransferResponse struct {
	SerializedTransaction string `json:"serialized_transaction"` // Transação em Base64 para assinatura
	DestinationATA        string `json:"destination_ata"`        // Endereço da ATA de destino
}

// PrepareTransfer prepara uma transação de transferência para assinatura do usuário.
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

// Request struct para completar a transferência
type CompleteTransferRequest struct {
	AssetID           string  `json:"asset_id"`
	FromUserID        string  `json:"from_user_id"`
	ToUserID          string  `json:"to_user_id"`
	Amount            float64 `json:"amount"`
	SignedTransaction string  `json:"signed_transaction"` // Transação assinada pelo usuário (Base64)
	DestinationATA    string  `json:"destination_ata"`    // ATA de destino, passada de volta pelo frontend
}

// CompleteTransfer envia a transação de transferência assinada para a Solana.
// POST /tokens/transfer/complete
func (h *TokenHandler) CompleteTransfer(w http.ResponseWriter, r *http.Request) {
	var req CompleteTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	destATA, err := solana.PublicKeyFromBase58(req.DestinationATA)
	if err != nil {
		http.Error(w, fmt.Sprintf("Endereço de ATA de destino inválido: %v", err), http.StatusBadRequest)
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

// ... (GetTokenByID, GetTokensByAssetID permanecem os mesmos) ...
