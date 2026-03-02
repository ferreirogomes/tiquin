package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ferreirogomes/tiquin/blockchain_listener" // Importar o listener
	"github.com/ferreirogomes/tiquin/handlers"
	"github.com/ferreirogomes/tiquin/services"
	"github.com/ferreirogomes/tiquin/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Load configuration (Assuming these are loaded from env vars or config file)
	dataSourceName := os.Getenv("DB_CONNECTION_STRING")
	solanaRPCURL := os.Getenv("SOLANA_RPC_URL")
	solanaFeePayerPrivateKey := os.Getenv("SOLANA_FEE_PAYER_PRIVATE_KEY")

	// ... (Configuração DB e Solana como antes) ...

	db, err := storage.NewDB(dataSourceName)
	if err != nil {
		log.Fatalf("Falha fatal ao conectar ao banco de dados e aplicar migrações: %v", err)
	}
	defer db.Close()

	solanaIntegrationService := services.NewSolanaIntegrationService(solanaRPCURL, solanaFeePayerPrivateKey)

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
	server := &http.Server{Addr: port, Handler: r}

	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		// Shutdown signal with grace period of 30 seconds
		shutdownCtx, _ := context.WithTimeout(serverCtx, 30*time.Second)
		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		// Trigger graceful shutdown
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
		serverStopCtx()
	}()

	fmt.Printf("Servidor backend rodando na porta %s...\n", port)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
	log.Println("Servidor parado com sucesso.")
}
