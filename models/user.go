package models

import "time"

// User represents an investor or token holder.
type User struct {
	ID           string    `json:"id"`
	Name         *string   `json:"name,omitempty"`
	Email        *string   `json:"email,omitempty"`
	SolanaPubKey string    `json:"solana_pub_key"`
	CreatedAt    time.Time `json:"created_at"`
}
