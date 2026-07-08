package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kudadonbe/kd-server/internal/ai"
	"github.com/kudadonbe/kd-server/internal/auth"
	"github.com/kudadonbe/kd-server/internal/config"
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

	if err := config.LoadDotEnv(); err != nil {
		logger.Fatalf("configuration load failed: %v", err)
	}

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
	assetService := services.NewAssetService(mongoStore)
	adminService := services.NewAdminService(mongoStore)
	identityDocumentService := services.NewIdentityDocumentService(mongoStore)
	documentExtractor, err := services.NewLocalDocumentExtractor()
	if err != nil {
		logger.Printf("warning: document extraction disabled: %v", err)
	}

	classificationService, err := services.NewClassificationService()
	if err != nil {
		logger.Printf("warning: classification service init failed: %v", err)
	} else {
		assetService.SetClassificationService(classificationService)
	}

	aiService, err := ai.NewServiceFromEnv(mongoStore)
	if err != nil {
		logger.Printf("warning: AI service disabled: %v", err)
	}

	// Document extraction: prefer the vision model, fall back to local OCR.
	// documentExtractor is a nil pointer when OCR is unavailable, so guard the
	// interface conversion to avoid a non-nil interface wrapping a nil pointer.
	var docExtractor services.DocumentExtractor
	if documentExtractor != nil {
		docExtractor = documentExtractor
	}
	if aiService != nil {
		docExtractor = services.NewAIDocumentExtractor(aiService, docExtractor)
	}

	handler := apphttp.NewHandler(apphttp.Config{
		Logger:                logger,
		VersionService:        versionService,
		AuthVerifier:          authVerifier,
		APIKeyVerifier:        mongoStore,
		IngestService:         ingestService,
		ResolveService:        resolveService,
		LookupService:         lookupService,
		ReviewService:         reviewService,
		AssetService:          assetService,
		ClassificationService: classificationService,
		AdminService:          adminService,
		IdentityDocuments:     identityDocumentService,
		DocumentExtractor:     docExtractor,
		AIService:             aiService,
		AdminUsername:         os.Getenv("KD_ADMIN_USERNAME"),
		AdminPassword:         os.Getenv("KD_ADMIN_PASSWORD"),
	})

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("api listen failed on %s: %v", addr, err)
	}

	baseURL := localBaseURL(addr)
	adminConfigured := os.Getenv("KD_ADMIN_USERNAME") != "" && os.Getenv("KD_ADMIN_PASSWORD") != ""
	logger.Println("------------------------------------------------------------")
	logger.Printf("KD-Server %s is ready", appVersion)
	logger.Printf("API:       %s", baseURL)
	logger.Printf("Home:      %s/", baseURL)
	logger.Printf("Admin:     %s/admin", baseURL)
	logger.Printf("Health:    %s/v1/healthz", baseURL)
	logger.Printf("MongoDB:   %s", mongoDatabase)
	logger.Printf("Admin UI:  %s", enabledLabel(adminConfigured))
	logger.Println("Press Ctrl+C to stop")
	logger.Println("------------------------------------------------------------")

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func localBaseURL(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "http://localhost" + addr
	}
	return "http://" + addr
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "not configured"
}
