package storage

import (
	"database/sql" // Import base sql
	"fmt"
	"log"

	"github.com/ferreirogomes/tiquin/models"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate" // Import sql-migrate
)

// DB represents the PostgreSQL database connection.
type DB struct {
	*sqlx.DB
}

// NewDB connects to PostgreSQL and runs migrations.
func NewDB(dataSourceName string) (*DB, error) {
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	log.Println("PostgreSQL connection established successfully.")

	// Executar migrações
	if err := runMigrations(db.DB); err != nil { // Pass *sql.DB
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{db}, nil
}

// runMigrations runs the migrations using sql-migrate.
func runMigrations(db *sql.DB) error {
	migrations := &migrate.FileMigrationSource{
		Dir: "./storage/migrations", // Path to SQL migrations
	}

	n, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		return fmt.Errorf("error applying migrations: %w", err)
	}
	if n > 0 {
		log.Printf("Applied %d migration(s) to the database.", n)
	} else {
		log.Println("No new migrations to apply.")
	}
	return nil
}

// SaveUser creates or updates a user based on their ID.
func (d *DB) SaveUser(user models.User) error {
	query := `
		INSERT INTO users (id, name, email, solana_pub_key, created_at)
		VALUES (:id, :name, :email, :solana_pub_key, :created_at)
		ON CONFLICT (solana_pub_key) DO UPDATE 
		SET name = EXCLUDED.name, email = EXCLUDED.email
	`
	_, err := d.NamedExec(query, user)
	return err
}

// GetUser retrieves a user by ID.
func (d *DB) GetUser(id string) (models.User, bool, error) {
	var user models.User
	err := d.Get(&user, "SELECT * FROM users WHERE id = $1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return user, false, nil
		}
		return user, false, err
	}
	return user, true, nil
}

// GetUserBySolanaPubKey retrieves a user by their Solana public key.
func (d *DB) GetUserBySolanaPubKey(pubKey string) (models.User, bool, error) {
	var user models.User
	err := d.Get(&user, "SELECT * FROM users WHERE solana_pub_key = $1", pubKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return user, false, nil
		}
		return user, false, err
	}
	return user, true, nil
}

// SaveAsset creates or updates an asset.
func (d *DB) SaveAsset(asset models.Asset) error {
	query := `
		INSERT INTO assets (id, symbol, name, total_shares, mint_address, created_at)
		VALUES (:id, :symbol, :name, :total_shares, :mint_address, :created_at)
		ON CONFLICT (symbol) DO UPDATE 
		SET mint_address = EXCLUDED.mint_address
	`
	_, err := d.NamedExec(query, asset)
	return err
}

// GetAsset retrieves an asset by ID.
func (d *DB) GetAsset(id string) (models.Asset, bool, error) {
	var asset models.Asset
	err := d.Get(&asset, "SELECT * FROM assets WHERE id = $1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return asset, false, nil
		}
		return asset, false, err
	}
	return asset, true, nil
}

// GetAssetByMintAddress retrieves an asset by its mint address.
func (d *DB) GetAssetByMintAddress(mintAddress string) (models.Asset, bool, error) {
	var asset models.Asset
	err := d.Get(&asset, "SELECT * FROM assets WHERE mint_address = $1", mintAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			return asset, false, nil
		}
		return asset, false, err
	}
	return asset, true, nil
}

// SaveToken creates or updates a token record.
func (d *DB) SaveToken(token models.Token) error {
	query := `
		INSERT INTO tokens (id, asset_id, owner_id, amount, smart_contract_rules, is_tradable, mint_address, token_account_address, transaction_id, created_at)
		VALUES (:id, :asset_id, :owner_id, :amount, :smart_contract_rules, :is_tradable, :mint_address, :token_account_address, :transaction_id, :created_at)
		ON CONFLICT (id) DO UPDATE 
		SET amount = EXCLUDED.amount, owner_id = EXCLUDED.owner_id, token_account_address = EXCLUDED.token_account_address
	`
	_, err := d.NamedExec(query, token)
	return err
}

// GetTokensByOwnerID retrieves all tokens for a specific user.
func (d *DB) GetTokensByOwnerID(ownerID string) ([]models.Token, error) {
	var tokens []models.Token
	err := d.Select(&tokens, "SELECT * FROM tokens WHERE owner_id = $1", ownerID)
	return tokens, err
}

// GetToken retrieves a token by ID
func (d *DB) GetToken(id string) (models.Token, bool, error) {
	var token models.Token
	err := d.Get(&token, "SELECT * FROM tokens WHERE id = $1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return token, false, nil
		}
		return token, false, err
	}
	return token, true, nil
}

// GetTokensByAssetID retrieves tokens for a given asset
func (d *DB) GetTokensByAssetID(assetID string) ([]models.Token, error) {
	var tokens []models.Token
	err := d.Select(&tokens, "SELECT * FROM tokens WHERE asset_id = $1", assetID)
	if err != nil {
		return nil, err
	}
	if tokens == nil {
		tokens = []models.Token{}
	}
	return tokens, nil
}

// TransactionExists checks if a transaction ID has already been processed and saved.
func (d *DB) TransactionExists(txID string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM tokens WHERE transaction_id = $1)"
	err := d.Get(&exists, query, txID)
	return exists, err
}

// UpdateToken updates a token's data.
func (d *DB) UpdateToken(token models.Token) error {
	query := `UPDATE tokens SET amount = $1, owner_id = $2, transaction_id = $3 WHERE id = $4`
	_, err := d.Exec(query, token.Amount, token.OwnerID, token.TransactionID, token.ID)
	if err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}
	return nil
}
