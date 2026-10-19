package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/config"
	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/router"
)

func main() {
	if err := run(); err != nil {
		slog.Error("servidor encerrado com falha", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.Open(startup, cfg.DataDir, cfg.Synchronous)
	cancel()
	if err != nil {
		return err
	}
	defer db.Close()
	server := &http.Server{
		Addr: cfg.Address,
		Handler: router.New(func(ctx context.Context) error {
			var ok int
			return db.QueryRowContext(ctx, "SELECT 1 AS ok").Scan(&ok)
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		// Sem WriteTimeout global: futuros streams de IA podem ser longos.
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	slog.Info("servidor Go iniciado", "endereco", listener.Addr().String())
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
