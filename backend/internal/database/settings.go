package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ReadAuthSettings lê somente dados existentes. Não cria tabelas nem altera versões.
// Um banco novo permite o login inicial; tabelas da aplicação serão criadas pelas migrations.
func ReadAuthSettings(ctx context.Context, db *sql.DB) (map[string]any, error) {
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='settings'").Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return map[string]any{}, nil
	}
	var data string
	err := db.QueryRowContext(ctx, "SELECT data FROM settings WHERE id=1").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(data), &settings); err != nil || settings == nil {
		return nil, fmt.Errorf("configurações de autenticação inválidas")
	}
	return settings, nil
}

// Defaults reproduz DEFAULT_SETTINGS de settingsRepo.js, incluindo as
// variáveis de ambiente lidas na inicialização do processo.
func Defaults() map[string]any {
	headroomURL := os.Getenv("HEADROOM_URL")
	if headroomURL == "" {
		headroomURL = "http://localhost:8787"
	}
	return map[string]any{
		"cloudEnabled":                 false,
		"stickyRoundRobinLimit":        float64(3),
		"providerStrategies":           map[string]any{},
		"comboStrategy":                "fallback",
		"comboStickyRoundRobinLimit":   float64(1),
		"comboStrategies":              map[string]any{},
		"requireLogin":                 true,
		"requireApiKey":                os.Getenv("REQUIRE_API_KEY") == "true",
		"authMode":                     "password",
		"oidcIssuerUrl":                "",
		"oidcClientId":                 "",
		"oidcClientSecret":             "",
		"oidcScopes":                   "openid profile email",
		"oidcLoginLabel":               "Sign in with OIDC",
		"enableObservability":          true,
		"observabilityMaxRecords":      float64(1000),
		"observabilityBatchSize":       float64(20),
		"observabilityFlushIntervalMs": float64(5000),
		"observabilityMaxJsonSize":     float64(5),
		"outboundProxyEnabled":         false,
		"outboundProxyUrl":             "",
		"outboundNoProxy":              "",
		"rtkEnabled":                   true,
		"headroomEnabled":              false,
		"headroomUrl":                  headroomURL,
		"headroomCompressUserMessages": false,
		"headroomTimeoutMs":            float64(3000),
		"cavemanEnabled":               false,
		"cavemanLevel":                 "full",
		"ponytailEnabled":              false,
		"ponytailLevel":                "official",
	}
}

// MergeWithDefaults reproduz mergeWithDefaults: os defaults são a base, e o
// banco sobrescreve todas as chaves presentes (inclusive JSON null).
func MergeWithDefaults(raw map[string]any) map[string]any {
	merged := Defaults()
	for key, value := range raw {
		merged[key] = value
	}
	return merged
}
