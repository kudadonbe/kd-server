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

	apphttp "github.com/kudadonbe/kd-server/internal/http"
	"github.com/kudadonbe/kd-server/internal/services"
)

const (
	defaultAddr     = ":8080"
	appVersion      = "1.0.0"
	shutdownTimeout = 5 * time.Second
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	addr := defaultAddr
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	versionService := services.NewStaticVersionService(appVersion)
	handler := apphttp.NewHandler(apphttp.Config{
		Logger:         logger,
		VersionService: versionService,
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
