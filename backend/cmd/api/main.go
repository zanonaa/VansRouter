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
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
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
	if err := database.Migrate(context.Background(), db, cfg.DataDir); err != nil {
		return err
	}
	database.SetDataDir(cfg.DataDir)
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
		WriteSettings: func(ctx context.Context, updates map[string]any) (map[string]any, error) {
			raw, err := database.UpdateSettings(ctx, db, updates)
			if err != nil {
				return nil, err
			}
			return database.MergeWithDefaults(raw), nil
		},
		Machine:     auth.NewMachineID(cfg.DataDir),
		ListAPIKeys: func(ctx context.Context) ([]map[string]any, error) { return database.GetApiKeys(ctx, db) },
		GetAPIKey: func(ctx context.Context, id string) (map[string]any, error) {
			return database.GetApiKeyById(ctx, db, id)
		},
		CreateAPIKey: func(ctx context.Context, name, machineId string) (map[string]any, error) {
			return database.CreateApiKey(ctx, db, name, machineId)
		},
		UpdateAPIKey: func(ctx context.Context, id string, data map[string]any) (map[string]any, error) {
			return database.UpdateApiKey(ctx, db, id, data)
		},
		DeleteAPIKey: func(ctx context.Context, id string) (bool, error) { return database.DeleteApiKey(ctx, db, id) },
		ValidateAPIKey: func(ctx context.Context, key string) (map[string]any, error) {
			return database.ValidateApiKey(ctx, db, key)
		},
		Initial:           initial,
		InitialValid:      valid,
		ForceSecure:       os.Getenv("AUTH_COOKIE_SECURE") == "true",
		CheckConfigured:   os.Getenv("JWT_SECRET") != "",
		EnableRequestLogs: os.Getenv("ENABLE_REQUEST_LOGS") == "true",
		EnableTranslator:  os.Getenv("ENABLE_TRANSLATOR") == "true",
	}
	shutdownRequested := make(chan struct{}, 1)
	shutdownHandler := &handlers.Shutdown{Request: func() {
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
	}}
	server := &http.Server{
		Addr: cfg.Address,
		Handler: router.NewWithNodes(func(ctx context.Context) error {
			var ok int
			return db.QueryRowContext(ctx, "SELECT 1 AS ok").Scan(&ok)
		}, authentication, &handlers.Nodes{Store: database.Nodes{DB: db}, Mutations: database.Nodes{DB: db}, Connections: database.Connections{DB: db}}, &handlers.Providers{OAuth: &handlers.OAuthEngine{CreateConnection: (database.Connections{DB: db}).Create}, Media: &handlers.MediaVoices{Fetchers: map[string]handlers.VoiceFetcher{"gemini": handlers.FetchGeminiVoicesPublic, "edge-tts": func(ctx context.Context, apiKey string) ([]map[string]any, error) {
			return handlers.FetchEdgeVoicesPublic(ctx)
		}, "elevenlabs": func(ctx context.Context, apiKey string) ([]map[string]any, error) {
			return handlers.FetchElevenVoicesPublic(ctx, apiKey)
		}}}, Database: &handlers.DatabaseExport{DB: db, ValidCLI: authentication.ValidCLIToken, VerifyPassword: func(password string) bool {
			return authentication.VerifyDashboardPassword(context.Background(), password)
		}}, Overrides: &handlers.Overrides{Read: authentication.Settings, Write: authentication.WriteSettings}, Shutdown: shutdownHandler, Pricing: &handlers.Pricing{Store: database.Pricing{DB: db}}, Logs: &handlers.LogEndpoints{Store: database.Usage{DB: db}, Connections: database.Connections{DB: db}}, Usage: &handlers.UsageEndpoints{Usage: database.Usage{DB: db}, Nodes: database.Nodes{DB: db}}, Models: &handlers.ModelsEndpoints{Disabled: handlers.DisabledModels{DB: db}, Aliases: database.ModelAliases{DB: db}}, Aliases: &handlers.ModelAliases{Store: database.ModelAliases{DB: db}}, Combos: &handlers.Combos{Store: database.Combos{DB: db}}, CreateBulk: (database.Connections{DB: db}).CreateBulk, Pools: &handlers.ProxyPools{Store: database.ProxyPools{DB: db}, Connections: database.Connections{DB: db}}, GetConnection: (database.Connections{DB: db}).Get, UpdateConnection: (database.Connections{DB: db}).Update, DeleteConnection: (database.Connections{DB: db}).Delete, CreateConnection: (database.Connections{DB: db}).Create, GetNode: (database.Nodes{DB: db}).Get, GetPool: (database.ProxyPools{DB: db}).Get, Headroom: &handlers.HeadroomRoutes{H: &handlers.Headroom{DataDir: cfg.DataDir}, Read: authentication.Settings}}),
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
	go func() {
		select {
		case <-shutdownRequested:
			stop()
		case <-ctx.Done():
		}
	}()
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
