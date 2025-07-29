package models

import "time"

// Token representa uma unidade digital fracionada de uma ação.
type Token struct {
	ID                 string    `json:"id"`
	AssetID            string    `json:"asset_id"`             // ID da ação a qual este token pertence
	OwnerID            string    `json:"owner_id"`             // ID do usuário que possui este token
	Amount             float64   `json:"amount"`               // Fração da ação que este token representa (ex: 0.001 de uma ação)
	SmartContractRules string    `json:"smart_contract_rules"` // Simula regras do smart contract (ex: "direitos de voto", "dividendos")
	IsTradable         bool      `json:"is_tradable"`          // Indica se o token pode ser negociado
	CreatedAt          time.Time `json:"created_at"`
}
