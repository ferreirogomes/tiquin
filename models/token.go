package models

import "time"

// Token represents a fractional digital unit of a share.
type Token struct {
	ID                  string    `json:"id"`
	AssetID             string    `json:"asset_id"`             // ID of the asset this token belongs to
	OwnerID             string    `json:"owner_id"`             // ID of the user who owns this token
	Amount              float64   `json:"amount"`               // Fraction of the asset this token represents (e.g., 0.001 of a share)
	SmartContractRules  string    `json:"smart_contract_rules"` // Simulates smart contract rules (e.g., "voting rights", "dividends")
	IsTradable          bool      `json:"is_tradable"`          // Indicates whether the token can be traded
	MintAddress         string    `json:"mint_address"`
	TokenAccountAddress string    `json:"token_account_address"`
	TransactionID       string    `json:"transaction_id"`
	CreatedAt           time.Time `json:"created_at"`
}
