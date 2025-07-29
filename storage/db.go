package storage

import (
	"database/sql" // Importar sql base
	"fmt"
	"log"

	"tokenization-backend/models"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate" // Importar sql-migrate
)

// DB representa a conexão com o banco de dados PostgreSQL.
type DB struct {
	*sqlx.DB
}

// NewDB conecta-se ao PostgreSQL e executa as migrações.
func NewDB(dataSourceName string) (*DB, error) {
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao banco de dados: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("falha ao pingar o banco de dados: %w", err)
	}
	log.Println("Conexão com PostgreSQL estabelecida com sucesso.")

	// Executar migrações
	if err := runMigrations(db.DB); err != nil { // Passar *sql.DB
		return nil, fmt.Errorf("falha ao executar migrações: %w", err)
	}

	return &DB{db}, nil
}

// runMigrations executa as migrações usando sql-migrate.
func runMigrations(db *sql.DB) error {
	migrations := &migrate.FileMigrationSource{
		Dir: "./storage/migrations", // Caminho para as migrações SQL
	}

	n, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		return fmt.Errorf("erro ao aplicar migrações: %w", err)
	}
	if n > 0 {
		log.Printf("Aplicadas %d migrações ao banco de dados.", n)
	} else {
		log.Println("Nenhuma migração nova para aplicar.")
	}
	return nil
}

// ... Métodos Save/Get/Update para models.User, models.Asset, models.Token permanecem os mesmos ...
// (Lembre-se de que os métodos SaveUser, SaveAsset, SaveToken já incluem ON CONFLICT para updates)

// Adicione aqui um método para atualizar um token, útil para listeners ou reconciliação
func (d *DB) UpdateToken(token models.Token) error {
	query := `UPDATE tokens SET amount = $1, owner_id = $2, transaction_id = $3 WHERE id = $4`
	_, err := d.Exec(query, token.Amount, token.OwnerID, token.TransactionID, token.ID)
	if err != nil {
		return fmt.Errorf("falha ao atualizar token: %w", err)
	}
	return nil
}
