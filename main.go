package main

import (
	"fmt"
	"log"
	"net/http"

	"tiquin/blockchain_listener" // Importar o listener
	"tiquin/handlers"
	"tiquin/services"
	"tiquin/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// ... (Configuração DB e Solana como antes) ...
	db, err := storage.NewDB(dataSourceName)
	if err != nil {
		log.Fatalf("Falha fatal ao conectar ao banco de dados e aplicar migrações: %v", err)
	}
	defer db.Close()

	solanaIntegrationService, err := services.NewSolanaIntegrationService(solanaRPCURL, db, solanaFeePayerPrivateKey)
	if err != nil {
		log.Fatalf("Falha ao inicializar serviço Solana: %v", err)
	}

	tokenizationService := services.NewTokenizationService(db, solanaIntegrationService)

	assetHandler := handlers.NewAssetHandler(tokenizationService)
	tokenHandler := handlers.NewTokenHandler(tokenizationService)
	userHandler := handlers.NewUserHandler(db, solanaIntegrationService, tokenizationService)

	// Inicializa e inicia o listener da blockchain em uma goroutine separada
	listener := blockchain_listener.NewBlockchainListener(solanaRPCURL, db, solanaFeePayerPrivateKey)
	go listener.StartListening()
	log.Println("Listener da blockchain iniciado.")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)

	r.Route("/assets", func(r chi.Router) {
		r.Post("/", assetHandler.CreateAsset)
		r.Get("/{id}", assetHandler.GetAssetByID)
	})

	r.Route("/tokens", func(r chi.Router) {
		r.Post("/transfer/prepare", tokenHandler.PrepareTransfer)   // Nova rota para preparar
		r.Post("/transfer/complete", tokenHandler.CompleteTransfer) // Nova rota para completar
		r.Get("/{id}", tokenHandler.GetTokenByID)
		r.Get("/by-asset/{assetID}", tokenHandler.GetTokensByAssetID)
	})

	r.Route("/users", func(r chi.Router) {
		r.Post("/", userHandler.CreateUser)
		r.Get("/{id}", userHandler.GetUserByID)
		r.Get("/{id}/tokens", userHandler.GetUserTokens)
	})

	port := ":8080"
	fmt.Printf("Servidor backend rodando na porta %s...\n", port)
	log.Fatal(http.ListenAndServe(port, r))
}
