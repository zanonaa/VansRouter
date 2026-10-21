package handlers

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var pidWindowsPattern = regexp.MustCompile(`^"?(\d+)"?`)

// Espelho de UPDATER_CONFIG e do appUpdater.js para POST /api/version/update.
const (
	updaterNPMPackageName = "vansrouter"
	updaterExitDelayMS    = 500
	updaterStatusPort     = 20129
	updaterLogTailLines   = 8
	updaterInstallRetries = 3
	updaterInstallRetryMS = 5000
	updaterLingerMS       = 30000
	updaterWaitMinMS      = 5000
	updaterWaitMaxMS      = 20000
	updaterWaitCheckMS    = 500
	updaterAppPort        = 20128
	updaterKillTimeout    = 5 * time.Second
	updaterProcessWait    = 1500 * time.Millisecond
)

// VersionUpdate mantém hooks para testes; produção usa os defaults reais.
type VersionUpdate struct {
	IsProduction  func() bool
	KillProcesses func() int
	Sleep         func(time.Duration)
	SpawnUpdater  func()
}

func NewVersionUpdate() *VersionUpdate { return &VersionUpdate{} }

func (h *VersionUpdate) production() bool {
	if h.IsProduction != nil {
		return h.IsProduction()
	}
	return os.Getenv("NODE_ENV") == "production"
}

func (h *VersionUpdate) Post(w http.ResponseWriter, r *http.Request) {
	if !h.production() {
		nodeReply(w, 403, map[string]any{"success": false, "message": "Update is only available in production build (vansrouter CLI)"})
		return
	}
	killed := 0
	if h.KillProcesses != nil {
		killed = h.KillProcesses()
	} else {
		killed = defaultKillAppProcesses()
	}
	if sleepFor := time.Duration(0); killed > 0 {
		sleepFor = updaterProcessWait
		if h.Sleep != nil {
			h.Sleep(sleepFor)
		} else {
			time.Sleep(sleepFor)
		}
	}
	if h.SpawnUpdater != nil {
		h.SpawnUpdater()
	} else {
		defaultSpawnUpdaterAndExit(updaterNPMPackageName)
	}
	nodeReply(w, 200, map[string]any{"success": true, "message": "Updater started. This app will exit shortly."})
}

// collectAppPIDs removido: collectAppPids(currentPID int) é a implementação.

func collectAppPids(currentPID int) []string {
	var pids []string
	if runtime.GOOS == "windows" {
		output, err := windowsProcessList()
		if err != nil {
			return pids
		}
		for _, line := range strings.Split(output, "\n")[1:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lower := strings.ToLower(line)
			isApp := strings.Contains(lower, "9router") ||
				strings.Contains(lower, "next-server") ||
				strings.Contains(lower, "\\bin\\app\\") ||
				strings.Contains(lower, "/bin/app/") ||
				strings.Contains(lower, "cli.js")
			if !isApp {
				continue
			}
			if match := pidWindowsPattern.FindStringSubmatch(strings.TrimSpace(line)); len(match) > 1 {
				if value, err := strconv.Atoi(match[1]); err == nil && value != currentPID {
					pids = append(pids, match[1])
				}
			}
		}
		trayOutput, err := windowsProcessCommand("Get-Process tray_windows_release -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id")
		if err == nil {
			for _, line := range strings.Split(trayOutput, "\n") {
				if value, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
					pids = append(pids, strconv.Itoa(value))
				}
			}
		}
		return pids
	}
	output, err := runQuiet("ps aux 2>/dev/null")
	if err != nil {
		return pids
	}
	for _, line := range strings.Split(output, "\n") {
		isApp := strings.Contains(line, "9router") ||
			strings.Contains(line, "next-server") ||
			strings.Contains(line, "/bin/app/") ||
			strings.Contains(line, "tray_darwin") ||
			strings.Contains(line, "tray_linux")
		if !isApp {
			continue
		}
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) < 2 {
			continue
		}
		if value, err := strconv.Atoi(parts[1]); err == nil && value != currentPID {
			pids = append(pids, parts[1])
		}
	}
	return pids
}

func windowsProcessList() (string, error) {
	cmd := exec.Command("powershell", "-NonInteractive", "-WindowStyle", "Hidden", "-Command",
		`Get-WmiObject Win32_Process -Filter 'Name="node.exe"' | Select-Object ProcessId,CommandLine | ConvertTo-Csv -NoTypeInformation`)
	return runCommand(cmd)
}

func windowsProcessCommand(command string) (string, error) {
	cmd := exec.Command("powershell", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", command)
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) (string, error) {
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// runQuiet espelha execSync com timeout 5s e ignorância de saída falha.
func runQuiet(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.WaitDelay = 0
	output, err := cmd.Output()
	return string(output), err
}

func defaultKillAppProcesses() int {
	pids := collectAppPids(kernelPID())
	isWindows := runtime.GOOS == "windows"
	for _, pid := range pids {
		var cmd *exec.Cmd
		if isWindows {
			cmd = exec.Command("taskkill", "/F", "/PID", pid)
		} else {
			cmd = exec.Command("sh", "-c", "kill -9 "+pid+" 2>/dev/null")
		}
		_ = cmd.Run()
	}
	return len(pids)
}

func kernelPID() int { return os.Getpid() }

// updaterDataDir espelha getDataDir (APPDATA\9router no Windows).
func updaterDataDir() string {
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		return dataDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "9router")
	}
	return filepath.Join(home, ".9router")
}

// resolveUpdaterPath espelha resolveBundledUpdaterPath.
func resolveUpdaterPath(cwd string) string {
	if OverridePath := os.Getenv("UPDATER_SCRIPT_PATH"); OverridePath != "" && fileExists(OverridePath) {
		return OverridePath
	}
	fromCwd := filepath.Join(cwd, "src", "lib", "updater", "updater.js")
	if fileExists(fromCwd) {
		return fromCwd
	}
	return filepath.Join(cwd, "..", "src", "lib", "updater", "updater.js")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ensureRuntimeUpdater copia o updater para o DATA_DIR (npm -g sobrescreve
// node_modules com segurança).
func ensureRuntimeUpdater(bundled string) string {
	if bundled == "" || !fileExists(bundled) {
		return bundled
	}
	runtimePath := filepath.Join(updaterDataDir(), "runtime", "updater", "updater.js")
	if existing := runtimePath; fileExists(existing) {
		if bundledSize(existing) == bundledSize(bundled) {
			return runtimePath
		}
	}
	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
		return bundled
	}
	if err := copyFile(bundled, runtimePath); err != nil {
		return bundled
	}
	return runtimePath
}

func bundledSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// defaultSpawnUpdaterAndExit espelha spawnUpdaterAndExit: node destacado com
// env UPDATER_* e saída do processo atual após exitDelayMs.
func defaultSpawnUpdaterAndExit(packageName string) {
	updater := ensureRuntimeUpdater(resolveUpdaterPath(mustCwd()))
	tray := os.Getenv("TRAY_MODE") == "1"
	npx := "npx"
	if runtime.GOOS == "windows" {
		npx = "npx.cmd"
	}
	relaunchArgs := []string{updaterNPMPackageName, "--skip-update"}
	if tray {
		relaunchArgs = append(relaunchArgs, "--tray")
	}
	cmd := exec.Command("node", updater)
	cmd.Env = append(os.Environ(),
		"UPDATER_PKG_NAME="+packageName,
		"UPDATER_PORT="+strconv.Itoa(updaterStatusPort),
		"UPDATER_TAIL_LINES="+strconv.Itoa(updaterLogTailLines),
		"UPDATER_RETRIES="+strconv.Itoa(updaterInstallRetries),
		"UPDATER_RETRY_DELAY_MS="+strconv.Itoa(updaterInstallRetryMS),
		"UPDATER_LINGER_MS="+strconv.Itoa(updaterLingerMS),
		"UPDATER_WAIT_MIN_MS="+strconv.Itoa(updaterWaitMinMS),
		"UPDATER_WAIT_MAX_MS="+strconv.Itoa(updaterWaitMaxMS),
		"UPDATER_WAIT_CHECK_MS="+strconv.Itoa(updaterWaitCheckMS),
		"UPDATER_APP_PORT="+strconv.Itoa(updaterAppPort),
		"UPDATER_RELAUNCH=1",
		"UPDATER_RELAUNCH_CMD="+npx,
		"UPDATER_RELAUNCH_ARGS="+jsonListMarshal(relaunchArgs),
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
	if cmd.Process != nil {
		go func() { _ = cmd.Process.Release() }()
	}
	time.AfterFunc(updaterExitDelayMS*time.Millisecond, func() { os.Exit(0) })
}

func jsonListMarshal(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
