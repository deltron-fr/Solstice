package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pact-marketplace/pact/backend/internal/app"
	"github.com/pact-marketplace/pact/backend/internal/blockchain"
	"github.com/pact-marketplace/pact/backend/internal/config"
	"github.com/pact-marketplace/pact/backend/internal/data"
	"github.com/pact-marketplace/pact/backend/internal/server"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, err := app.OpenDatabase(cfg.DatabaseDSN)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	chain := blockchain.NewRPCClient(blockchain.Config{RPCURL: cfg.SolanaRPCURL, ProgramID: cfg.SolanaProgramID})

	httpServer := &http.Server{
		Addr: cfg.HTTPAddr, Handler: server.New(logger, data.New(db), chain).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		logger.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}
}
