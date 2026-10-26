package handlers

// Porta de src/lib/pxpipe/install.js.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	pxpipePackage    = "pxpipe-proxy"
	pxpipeVersion    = "0.15.1"
	pxpipeTimeoutRun = 5 * time.Minute
)

func pxpipeDir(dataDir string) string { return filepath.Join(dataDir, "pxpipe") }
func pxpipeInstallLogFile(dataDir string) string {
	return filepath.Join(pxpipeDir(dataDir), "install.log")
}
func pxpipePackageRoot(dataDir string) string {
	if root := strings.TrimSpace(os.Getenv("PXPIPE_PACKAGE_ROOT")); root != "" {
		return root
	}
	return filepath.Join(pxpipeDir(dataDir), "node_modules", pxpipePackage)
}
func pxpipeLibraryEntry(dataDir string) string {
	return filepath.Join(pxpipePackageRoot(dataDir), "dist", "core", "library.js")
}

// Mesma extensão de PATH do headroom/detect.js: ambientes empacotados não
// enxergam os diretórios de binários do Node.
func pxpipeExtendedPath() string {
	home := ""
	if value, err := os.UserHomeDir(); err == nil {
		home = value
	}
	var bins []string
	if runtime.GOOS == "windows" {
		bins = []string{os.Getenv("ProgramFiles") + "\\nodejs", os.Getenv("APPDATA") + "\\npm"}
	} else {
		bins = []string{"/usr/local/bin", "/opt/homebrew/bin", home + "/.local/bin", "/usr/bin", "/bin"}
	}
	bins = append(bins, os.Getenv("PATH"))
	joined := []string{}
	for _, bin := range bins {
		if bin != "" {
			joined = append(joined, bin)
		}
	}
	return strings.Join(joined, string(filepath.ListSeparator))
}

// pxpipeInstallInfo espelha getInstallInfo: instalado = entry da lib no disco.
func pxpipeInstallInfo(dataDir string) map[string]any {
	info := map[string]any{"installed": false, "version": nil, "path": nil}
	pkgJSON := filepath.Join(pxpipePackageRoot(dataDir), "package.json")
	if !fileExists(pkgJSON) || !fileExists(pxpipeLibraryEntry(dataDir)) {
		return info
	}
	raw, err := os.ReadFile(pkgJSON)
	if err != nil {
		return info
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &pkg) != nil {
		return info
	}
	info["installed"] = true
	if pkg.Version != "" {
		info["version"] = pkg.Version
	}
	info["path"] = pxpipePackageRoot(dataDir)
	return info
}

func pxpipeFindNpm() string {
	tool := "which"
	if runtime.GOOS == "windows" {
		tool = "where"
	}
	cmd := exec.Command(tool, "npm")
	cmd.Env = append(os.Environ(), "PATH="+pxpipeExtendedPath())
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	return strings.TrimSpace(first)
}

// pxpipeError preserva o code do JS (NPM_NOT_FOUND, NOT_INSTALLED, ...) para
// o corpo `{error, code}` das respostas 500.
type pxpipeError struct {
	Code string
	Msg  string
}

func (e *pxpipeError) Error() string { return e.Msg }

func pxpipeErrorCode(err error) any {
	if coded, ok := err.(*pxpipeError); ok {
		return coded.Code
	}
	return nil
}

// Serialização do JS (installInFlight): com mutex mantido durante a corrida,
// chamadas concorrentes aguardam; a reexecução subsequente é inócua, pois o
// pacote já estará instalado — divergência benigna documentada.
type pxpipeInstallState struct {
	mu         sync.Mutex
	installing bool
}

var pxpipeState = &pxpipeInstallState{}

func (s *pxpipeInstallState) isInstalling() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.installing
}

// pxpipeInstall serializa: só uma execução por vez.
func (s *pxpipeInstallState) pxpipeInstall(ctx context.Context, dataDir string, run func(context.Context, string) (map[string]any, error)) (map[string]any, error) {
	s.mu.Lock()
	s.installing = true
	defer s.mu.Unlock()
	defer func() { s.installing = false }()
	return run(ctx, dataDir)
}

// PxpipeIsInstalling espelha isInstalling().
func PxpipeIsInstalling() bool { return pxpipeState.isInstalling() }

// pxpipeRunInstall executa o npm com log anexado e timeout de 5 minutos.
func pxpipeRunInstall(ctx context.Context, dataDir string, findNpm func() string) (map[string]any, error) {
	if strings.TrimSpace(os.Getenv("PXPIPE_PACKAGE_ROOT")) != "" {
		info := pxpipeInstallInfo(dataDir)
		if info["installed"] != true {
			return nil, &pxpipeError{Code: "BUNDLED_PACKAGE_MISSING", Msg: "Bundled PXPIPE is unavailable; rebuild the image"}
		}
		return info, nil
	}
	npm := findNpm()
	if npm == "" {
		return nil, &pxpipeError{Code: "NPM_NOT_FOUND", Msg: "npm not found on PATH — Node.js/npm is required to install PXPIPE"}
	}
	if err := os.MkdirAll(pxpipeDir(dataDir), 0o755); err != nil {
		return nil, err
	}
	pkgJSON := filepath.Join(pxpipeDir(dataDir), "package.json")
	if !fileExists(pkgJSON) {
		payload := "{\n  \"name\": \"9router-pxpipe-host\",\n  \"private\": true\n}"
		if err := os.WriteFile(pkgJSON, []byte(payload), 0o644); err != nil {
			return nil, err
		}
	}
	log, err := os.OpenFile(pxpipeInstallLogFile(dataDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer log.Close()
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, _ = fmt.Fprintf(log, "\n[%s] npm install %s@%s\n", stamp, pxpipePackage, pxpipeVersion)
	cmd := exec.CommandContext(ctx, npm, "install", pxpipePackage+"@"+pxpipeVersion, "--no-audit", "--no-fund", "--omit=dev")
	cmd.Dir = pxpipeDir(dataDir)
	cmd.Env = append(os.Environ(), "PATH="+pxpipeExtendedPath())
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.AfterFunc(pxpipeTimeoutRun, func() {
		_ = cmd.Process.Kill()
	})
	err = <-done
	timer.Stop()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("npm install timed out after 5 minutes — see install.log")
		}
		if exitErr, ok := err.(*exec.ExitError); ok && !exitErr.Success() {
			return nil, fmt.Errorf("npm install exited with code %d — see install.log", exitErr.ExitCode())
		}
		return nil, err
	}
	info := pxpipeInstallInfo(dataDir)
	if installed, _ := info["installed"].(bool); !installed {
		return nil, fmt.Errorf("install finished but package is missing — see install.log")
	}
	return info, nil
}

// PxpipeInstallLogTail espelha getInstallLogTail (200 linhas por padrão).
func PxpipeInstallLogTail(dataDir string, maxLines int) string {
	raw, err := os.ReadFile(pxpipeInstallLogFile(dataDir))
	if err != nil {
		if maxLines <= 0 {
			maxLines = 200
		}
		return ""
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if maxLines <= 0 {
		maxLines = 200
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}
