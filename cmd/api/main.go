package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kudadonbe/kd-server/internal/auth"
	apphttp "github.com/kudadonbe/kd-server/internal/http"
	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

const (
	defaultAddr          = ":8080"
	appVersion           = "1.0.0"
	shutdownTimeout      = 5 * time.Second
	defaultMongoURI      = "mongodb://localhost:27017"
	defaultMongoDatabase = "kdserver"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	addr := defaultAddr
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = defaultMongoURI
	}

	mongoDatabase := os.Getenv("MONGO_DB")
	if mongoDatabase == "" {
		mongoDatabase = defaultMongoDatabase
	}

	jwtSecret := os.Getenv("JWT_SIGNING_KEY")
	if jwtSecret == "" {
		logger.Fatal("environment variable JWT_SIGNING_KEY is required")
	}

	authVerifier, err := auth.NewHMACVerifier(jwtSecret)
	if err != nil {
		logger.Fatalf("failed to configure auth verifier: %v", err)
	}

	rootCtx := context.Background()
	mongoStore, err := store.Connect(rootCtx, store.Config{
		URI:      mongoURI,
		Database: mongoDatabase,
		Logger:   logger,
		Timeout:  shutdownTimeout,
	})
	if err != nil {
		logger.Fatalf("mongo connect failed: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := mongoStore.Close(closeCtx); err != nil {
			logger.Printf("mongo disconnect error: %v", err)
		}
	}()

	indexCtx, cancelIndexes := context.WithTimeout(rootCtx, shutdownTimeout)
	defer cancelIndexes()

	if err := mongoStore.EnsureIndexes(indexCtx); err != nil {
		logger.Fatalf("mongo ensure indexes failed: %v", err)
	}

	versionService := services.NewStaticVersionService(appVersion)
	ingestService := services.NewIngestService(mongoStore.IngestWriter())
	resolveService := services.NewResolveService(mongoStore)
	lookupService := services.NewLookupService(mongoStore)
	reviewService := services.NewReviewService(mongoStore)
	handler := apphttp.NewHandler(apphttp.Config{
		Logger:         logger,
		VersionService: versionService,
		AuthVerifier:   authVerifier,
		IngestService:  ingestService,
		ResolveService: resolveService,
		LookupService:  lookupService,
		ReviewService:  reviewService,
	})

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	logger.Printf("starting kd-server api on %s", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Printf("received signal %s, shutting down", sig.String())
	case err := <-errCh:
		logger.Fatalf("server error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("graceful shutdown failed: %v", err)
	}

	logger.Println("server stopped cleanly")
}
