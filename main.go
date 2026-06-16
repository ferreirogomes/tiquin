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

	"github.com/ferreirogomes/tiquin/blockchain_listener"
	"github.com/ferreirogomes/tiquin/handlers"
	apimiddleware "github.com/ferreirogomes/tiquin/middleware"
	"github.com/ferreirogomes/tiquin/services"
	"github.com/ferreirogomes/tiquin/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	dataSourceName := os.Getenv("DB_CONNECTION_STRING")
	solanaRPCURL := os.Getenv("SOLANA_RPC_URL")
	solanaFeePayerPrivateKey := os.Getenv("SOLANA_FEE_PAYER_PRIVATE_KEY")

	db, err := storage.NewDB(dataSourceName)
	if err != nil {
		log.Fatalf("Fatal error connecting to database and applying migrations: %v", err)
	}
	defer db.Close()

	solanaIntegrationService := services.NewSolanaIntegrationService(solanaRPCURL, solanaFeePayerPrivateKey)
	tokenizationService := services.NewTokenizationService(db, solanaIntegrationService)

	assetHandler := handlers.NewAssetHandler(tokenizationService)
	tokenHandler := handlers.NewTokenHandler(tokenizationService)
	userHandler := handlers.NewUserHandler(db, solanaIntegrationService, tokenizationService)

	// Initialize and start the blockchain listener in a separate goroutine
	listener := blockchain_listener.NewBlockchainListener(solanaRPCURL, db, solanaFeePayerPrivateKey)
	go listener.StartListening()
	log.Println("Blockchain listener started.")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)

	// P4: Apply API key authentication to all routes
	authMiddleware := apimiddleware.APIKeyAuth(db.DB)
	r.Use(authMiddleware)

	r.Route("/assets", func(r chi.Router) {
		r.Post("/", assetHandler.CreateAsset)
		r.Get("/{id}", assetHandler.GetAssetByID)
	})

	r.Route("/tokens", func(r chi.Router) {
		r.Post("/transfer/prepare", tokenHandler.PrepareTransfer)
		r.Post("/transfer/complete", tokenHandler.CompleteTransfer)
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

		// QW1 fix: properly capture and defer the cancel function
		shutdownCtx, cancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		// QW3: Stop the blockchain listener gracefully before server shuts down
		log.Println("Stopping blockchain listener...")
		listener.Stop()

		// Trigger graceful HTTP server shutdown
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
		serverStopCtx()
	}()

	fmt.Printf("Backend server running on port %s...\n", port)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
	log.Println("Server stopped successfully.")
}
