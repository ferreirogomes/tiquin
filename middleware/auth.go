package middleware

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/jmoiron/sqlx"
)

// APIKeyAuth returns a middleware that validates API keys from the X-API-Key header.
// Keys are stored as SHA-256 hashes in the api_keys table.
func APIKeyAuth(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := r.Header.Get("X-API-Key")
			if rawKey == "" {
				// Also accept Bearer token format
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					rawKey = strings.TrimPrefix(auth, "Bearer ")
				}
			}

			if rawKey == "" {
				http.Error(w, `{"error":"missing API key — provide X-API-Key header"}`, http.StatusUnauthorized)
				return
			}

			keyHash := hashAPIKey(rawKey)

			var keyID string
			err := db.Get(&keyID,
				`SELECT id FROM api_keys WHERE key_hash = $1 AND is_active = true`,
				keyHash,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, `{"error":"invalid or inactive API key"}`, http.StatusUnauthorized)
				} else {
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				}
				return
			}

			// Update last_used_at asynchronously (non-blocking)
			go func() {
				_, _ = db.Exec(
					`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`,
					keyID,
				)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// hashAPIKey returns the SHA-256 hex hash of a raw API key string.
func hashAPIKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", h)
}
