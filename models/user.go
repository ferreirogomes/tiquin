package models

import "time"

// User representa um investidor ou titular de tokens.
type User struct {
	ID           string    `json:"id"`
	Name         *string   `json:"name,omitempty"`
	Email        *string   `json:"email,omitempty"`
	SolanaPubKey string    `json:"solana_pub_key"`
	CreatedAt    time.Time `json:"created_at"`
}
