package models

import "time"

// Asset representa uma ação tradicional que será tokenizada.
type Asset struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`       // Ex: "AAPL", "PETR4"
	Name        string    `json:"name"`         // Ex: "Apple Inc.", "Petrobras S.A."
	TotalShares float64   `json:"total_shares"` // Quantidade total de ações existentes
	MintAddress string    `json:"mint_address,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
