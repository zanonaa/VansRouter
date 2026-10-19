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

	"github.com/zanonaa/VansRouter/backend/internal/auth"
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
	sessions, err := auth.LoadSessions(cfg.DataDir, os.Getenv("JWT_SECRET"))
	if err != nil {
		return err
	}
	initial, valid := auth.InitialPassword(os.Getenv("INITIAL_PASSWORD"), os.Getenv("NODE_ENV"))
	authentication := &auth.HTTP{
		Sessions: sessions,
		Limiter:  auth.NewLimiter(),
		Settings: func(ctx context.Context) (map[string]any, error) {
			raw, err := database.ReadAuthSettings(ctx, db)
			if err != nil {
				return nil, err
			}
			return database.MergeWithDefaults(raw), nil
		},
		ChangeP: func(ctx context.Context, hash string) (map[string]any, error) {
			return database.ChangePassword(ctx, db, hash)
		},
		Initial:           initial,
		InitialValid:      valid,
		ForceSecure:       os.Getenv("AUTH_COOKIE_SECURE") == "true",
		CheckConfigured:   os.Getenv("JWT_SECRET") != "",
		EnableRequestLogs: os.Getenv("ENABLE_REQUEST_LOGS") == "true",
		EnableTranslator:  os.Getenv("ENABLE_TRANSLATOR") == "true",
	}
	server := &http.Server{
		Addr: cfg.Address,
		Handler: router.NewWithAuth(func(ctx context.Context) error {
			var ok int
			return db.QueryRowContext(ctx, "SELECT 1 AS ok").Scan(&ok)
		}, authentication),
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
