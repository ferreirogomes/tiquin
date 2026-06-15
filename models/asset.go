package models

import "time"

// Asset represents a traditional share that will be tokenized.
type Asset struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`       // e.g., "AAPL", "PETR4"
	Name        string    `json:"name"`         // e.g., "Apple Inc.", "Petrobras S.A."
	TotalShares float64   `json:"total_shares"` // Total number of shares in existence
	MintAddress string    `json:"mint_address,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
