package handlers

// Porta de src/app/api/oauth/cursor/auto-import/route.js.
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var cursorAccessTokenKeys = []string{"cursorAuth/accessToken", "cursorAuth/token"}
var cursorMachineIDKeys = []string{"storage.serviceMachineId", "storage.machineId", "telemetry.machineId"}

// cursorNormalize: desembrulha JSON string única (JS JSON.parse de um string).
func cursorNormalize(value string) string {
	var parsed string
	if err := json.Unmarshal([]byte(value), &parsed); err == nil && parsed != "" {
		return parsed
	}
	return value
}

// CursorAutoImport extrai o token do state.vscdb com hooks testáveis.
type CursorAutoImport struct {
	Home         string
	AppData      string
	LocalAppData string
	Platform     string
	// OpenDB abre a state.vscdb readonly (default: driver embutido).
	OpenDB func(path string) (*sql.DB, error)
	// IsInstalledLinux: `which cursor`/desktop (hook para testes).
	Installed bool
	// DesktopDir para o fallback de .desktop (default /usr/share/applications).
	DesktopDir string
}

func NewCursorAutoImport() *CursorAutoImport {
	home, _ := os.UserHomeDir()
	platform := runtime.GOOS
	if platform == "windows" {
		platform = "win32"
	}
	return &CursorAutoImport{Home: home, Platform: platform, AppData: os.Getenv("APPDATA"), LocalAppData: os.Getenv("LOCALAPPDATA")}
}

func (c *CursorAutoImport) home() string {
	if c.Home != "" {
		return c.Home
	}
	home, _ := os.UserHomeDir()
	return home
}

// candidatePaths espelha getCandidatePaths.
func (c *CursorAutoImport) candidatePaths() []string {
	home := c.home()
	if c.Platform == "darwin" {
		return []string{
			filepath.Join(home, "Library/Application Support/Cursor/User/globalStorage/state.vscdb"),
			filepath.Join(home, "Library/Application Support/Cursor - Insiders/User/globalStorage/state.vscdb"),
		}
	}
	if c.Platform == "win32" {
		appData := c.AppData
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		localAppData := c.LocalAppData
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return []string{
			filepath.Join(appData, "Cursor", "User", "globalStorage", "state.vscdb"),
			filepath.Join(appData, "Cursor - Insiders", "User", "globalStorage", "state.vscdb"),
			filepath.Join(localAppData, "Cursor", "User", "globalStorage", "state.vscdb"),
			filepath.Join(localAppData, "Programs", "Cursor", "User", "globalStorage", "state.vscdb"),
		}
	}
	return []string{
		filepath.Join(home, ".config/Cursor/User/globalStorage/state.vscdb"),
		filepath.Join(home, ".config/cursor/User/globalStorage/state.vscdb"),
	}
}

// resolveDbPath: primeiro candidato legível.
func (c *CursorAutoImport) resolveDbPath() string {
	for _, candidate := range c.candidatePaths() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// isCursorInstalledLinux: binário cursor no PATH ou .desktop presente.
func (c *CursorAutoImport) isCursorInstalledLinux() bool {
	if c.Installed {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "which", "cursor")
	if err := command.Run(); err == nil {
		return true
	}
	desktopDir := c.DesktopDir
	if desktopDir == "" {
		desktopDir = filepath.Join(c.home(), ".local/share/applications")
	}
	_, err := os.Stat(filepath.Join(desktopDir, "cursor.desktop"))
	return err == nil
}

// extractTokens espelha extractTokensViaBetterSqlite (driver embutido, ro).
func (c *CursorAutoImport) extractTokens(dbPath string) (accessToken, machineID string, err error) {
	db, err := c.openDB(dbPath)
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	for _, key := range cursorAccessTokenKeys {
		var raw string
		if err := db.QueryRow("SELECT value FROM itemTable WHERE key=? LIMIT 1", key).Scan(&raw); err == nil && raw != "" {
			accessToken = cursorNormalize(raw)
			break
		}
	}
	for _, key := range cursorMachineIDKeys {
		var raw string
		if err := db.QueryRow("SELECT value FROM itemTable WHERE key=? LIMIT 1", key).Scan(&raw); err == nil && raw != "" {
			machineID = cursorNormalize(raw)
			break
		}
	}
	return accessToken, machineID, nil
}

func (c *CursorAutoImport) openDB(dbPath string) (*sql.DB, error) {
	if c.OpenDB != nil {
		return c.OpenDB(dbPath)
	}
	return sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
}

// Get espelha a rota.
func (c *CursorAutoImport) Get(w http.ResponseWriter, r *http.Request) {
	dbPath := c.resolveDbPath()
	if dbPath == "" {
		nodeReply(w, 200, map[string]any{
			"found": false,
			"error": fmt.Sprintf("Cursor database not found. Checked locations:\n%s\n\nMake sure Cursor IDE is installed and opened at least once.", strings.Join(c.candidatePaths(), "\n")),
		})
		return
	}
	if c.Platform == "linux" && !c.isCursorInstalledLinux() {
		nodeReply(w, 200, map[string]any{
			"found": false,
			"error": "Cursor config files found but Cursor IDE does not appear to be installed. Skipping auto-import.",
		})
		return
	}
	// Estratégia 1: driver embutido.
	accessToken, machineID, err := c.extractTokens(dbPath)
	if err == nil && accessToken != "" && machineID != "" {
		nodeReply(w, 200, map[string]any{"found": true, "accessToken": accessToken, "machineId": machineID})
		return
	}
	// Estratégia 2: sqlite3 CLI (fallback; divergência registrada: falha como a origem).
	accessToken, machineID, cliErr := c.extractTokensViaCLI(dbPath)
	if cliErr == nil && accessToken != "" && machineID != "" {
		nodeReply(w, 200, map[string]any{"found": true, "accessToken": accessToken, "machineId": machineID})
		return
	}
	// Estratégia 3: manual.
	nodeReply(w, 200, map[string]any{"found": false, "windowsManual": true, "dbPath": dbPath})
}

// extractTokensViaCLI espelha extractTokensViaCLI.
func (c *CursorAutoImport) extractTokensViaCLI(dbPath string) (accessToken, machineID string, err error) {
	queryKeys := func(keys []string) string {
		for _, key := range keys {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			command := exec.CommandContext(ctx, "sqlite3", dbPath, fmt.Sprintf("SELECT value FROM itemTable WHERE key='%s' LIMIT 1;", key))
			hideHeadroomWindow(command)
			out, queryErr := command.Output()
			cancel()
			if raw := strings.TrimSpace(string(out)); queryErr == nil && raw != "" {
				return cursorNormalize(raw)
			}
		}
		return ""
	}
	return queryKeys(cursorAccessTokenKeys), queryKeys(cursorMachineIDKeys), nil
}
