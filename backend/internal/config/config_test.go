package config

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{"padrões", map[string]string{}, false},
		{"porta inválida", map[string]string{"PORT": "0"}, true},
		{"porta textual", map[string]string{"PORT": "abc"}, true},
		{"synchronous inválido", map[string]string{"SQLITE_SYNCHRONOUS": "NORMAL; DROP TABLE settings"}, true},
		{"temporário produção", map[string]string{"NODE_ENV": "production", "DATA_DIR": "/tmp/9router-data-test"}, true},
		{"temporário autorizado", map[string]string{"NODE_ENV": "production", "DATA_DIR": "/tmp/9router-data-test", "DATA_DIR_ALLOW_TEMP": "1"}, false},
		{"configuração explícita", map[string]string{"PORT": "20228", "DATA_DIR": "/dados/router", "SQLITE_SYNCHRONOUS": "full"}, false},
		{"spa inexistente", map[string]string{"SPA_DIR": "/caminho/que/nao/existe"}, true},
		{"spa em arquivo", map[string]string{"SPA_DIR": "/etc/hosts"}, true},
		{"segredo placeholder em produção", map[string]string{"NODE_ENV": "production", "JWT_SECRET": "change-me"}, true},
		{"segredo placeholder api key em produção", map[string]string{"NODE_ENV": "production", "API_KEY_SECRET": "ChangeMe"}, true},
		{"segredo forte em produção", map[string]string{"NODE_ENV": "production", "JWT_SECRET": "um-segredo-real-longo"}, false},
		{"segredo placeholder fora de produção", map[string]string{"JWT_SECRET": "changeme"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := load(func(k string) string { return tt.env[k] }, func() (string, error) { return "/home/operador", nil }, "linux")
			if (err != nil) != tt.wantErr {
				t.Fatalf("erro = %v", err)
			}
			if err == nil && tt.name == "padrões" && (cfg.Address != ":20128" || cfg.DataDir != filepath.Join("/home/operador", ".9router") || cfg.Synchronous != "NORMAL") {
				t.Fatalf("configuração inesperada: %+v", cfg)
			}
		})
	}
}
