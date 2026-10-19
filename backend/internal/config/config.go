// Package config carrega a configuração do servidor sem expor segredos.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	Address     string
	DataDir     string
	Synchronous string
}

func Load() (Config, error) {
	return load(os.Getenv, os.UserHomeDir, runtime.GOOS)
}

func load(env func(string) string, home func() (string, error), platform string) (Config, error) {
	port := env("PORT")
	if port == "" {
		port = "20128"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("PORT deve estar entre 1 e 65535")
	}
	dir := env("DATA_DIR")
	production := env("NODE_ENV") == "production" || env("pm_id") != ""
	normalized := strings.ToLower(strings.ReplaceAll(dir, "\\", "/"))
	temporary := strings.Contains(normalized, "smoke") || strings.HasPrefix(normalized, "/tmp/9router-data-") || (platform == "darwin" && strings.Contains(normalized, "/var/folders/"))
	if production && temporary && env("DATA_DIR_ALLOW_TEMP") != "1" {
		return Config{}, fmt.Errorf("DATA_DIR temporário exige DATA_DIR_ALLOW_TEMP=1 em produção; nenhum fallback silencioso foi aplicado")
	}
	if platform == "windows" && strings.HasPrefix(dir, "/") {
		return Config{}, fmt.Errorf("DATA_DIR deve ser um caminho Windows válido")
	}
	if dir == "" {
		base, err := home()
		if err != nil {
			return Config{}, fmt.Errorf("resolver diretório pessoal: %w", err)
		}
		dir = filepath.Join(base, ".9router")
		if platform == "windows" {
			appData := env("APPDATA")
			if appData == "" {
				appData = filepath.Join(base, "AppData", "Roaming")
			}
			dir = filepath.Join(appData, "9router")
		}
	}
	mode := strings.ToUpper(env("SQLITE_SYNCHRONOUS"))
	if mode == "" {
		mode = "NORMAL"
	}
	switch mode {
	case "OFF", "NORMAL", "FULL", "EXTRA":
	default:
		return Config{}, fmt.Errorf("SQLITE_SYNCHRONOUS inválido")
	}
	return Config{Address: ":" + port, DataDir: dir, Synchronous: mode}, nil
}
