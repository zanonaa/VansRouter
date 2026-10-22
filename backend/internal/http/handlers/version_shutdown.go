package handlers

// Espelho de src/app/api/version/shutdown/route.js + src/shared/utils/runtime.js.
import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// VersionShutdown replica updateAndRestartCommand + spawn destacado + exit;
// hooks para testes sem sair do processo.
type VersionShutdown struct {
	Detect        func() string
	KillProcesses func() int
	Spawn         func(command string) bool
	Exit          func()
}

func NewVersionShutdown() *VersionShutdown { return &VersionShutdown{} }

// detectRuntime espelha os marcadores do runtime.js.
func detectRuntime() string {
	if os.Getenv("PM2_HOME_PATH") != "" || os.Getenv("PM2_JSON_PROCESSING") != "" || os.Getenv("pm2_env") != "" {
		return "pm2"
	}
	if os.Getenv("INVOCATION_ID") != "" {
		return "systemd"
	}
	if value, exists := os.LookupEnv("TMUX"); exists && value != "" {
		return "tmux"
	}
	if value, exists := os.LookupEnv("STY"); exists && value != "" {
		return "screen"
	}
	if detectDockerRuntime() {
		return "docker"
	}
	return "direct"
}

// detectDockerRuntime: /.dockerenv ou cgroup do PID 1.
func detectDockerRuntime() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	content, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(content)), "docker") ||
		strings.Contains(strings.ToLower(string(content)), "containerd") ||
		strings.Contains(strings.ToLower(string(content)), "kubepods") ||
		strings.Contains(strings.ToLower(string(content)), "buildkit")
}

// pm2ProcessName espelha readPm2ProcessName (pm2_env é JSON string).
func pm2ProcessName() string {
	env := os.Getenv("pm2_env")
	if env == "" {
		return ""
	}
	var parsed struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(env), &parsed) != nil {
		return ""
	}
	return parsed.Name
}

// systemdUnitName deriva do executável (readSystemdUnitName).
func systemdUnitName() string {
	if exe, err := os.Executable(); err == nil {
		if base := filepath.Base(exe); base != "" {
			return base + ".service"
		}
	}
	return ""
}

// updateAndRestartCommand retorna comando e viabilidade de auto-restart.
func updateAndRestartCommand(runtime, packageName string) (string, bool) {
	installCmd := "npm i -g " + packageName + "@latest --prefer-online"
	switch runtime {
	case "pm2":
		name := pm2ProcessName()
		if name == "" {
			name = "vansrouter"
		}
		return installCmd + " && pm2 restart " + name, true
	case "systemd":
		unit := systemdUnitName()
		if unit == "" {
			unit = packageName
		}
		return installCmd + " && sudo systemctl restart " + unit, true
	case "screen":
		return installCmd + " && screen -dmS vansrouter vansrouter", true
	case "tmux":
		return installCmd + " && tmux new-session -d -s vansrouter 'vansrouter'", true
	case "docker":
		return installCmd + " # NOTE: inside Docker — pull new image on host (docker compose pull && up -d) or use Watchtower", true
	default:
		// Foreground puro: usuário reinicia manualmente.
		return "", false
	}
}

func (h *VersionShutdown) runtime() string {
	if h.Detect != nil {
		return h.Detect()
	}
	return detectRuntime()
}

func (h *VersionShutdown) kill() int {
	if h.KillProcesses != nil {
		return h.KillProcesses()
	}
	return defaultKillAppProcesses()
}

func (h *VersionShutdown) exitSoon() {
	if h.Exit != nil {
		h.Exit()
		return
	}
	time.AfterFunc(1500*time.Millisecond, func() { os.Exit(0) })
}

func (h *VersionShutdown) spawnChild(command string) bool {
	if h.Spawn != nil {
		return h.Spawn(command)
	}
	return defaultSpawnShutdownChild(command)
}

func (h *VersionShutdown) Post(w http.ResponseWriter, r *http.Request) {
	runtime := h.runtime()
	body := map[string]any{}
	if input, err := poolInput(r); err == nil && input != nil {
		body = input
	}
	packageName := ttsStringOr(body["packageName"], "vansrouter")
	mode := "auto"
	if jsString(body["mode"]) == "manual" {
		mode = "manual"
	}
	// Best effort: processos irmãos que prendem locks de arquivo.
	_ = h.kill()
	updateCommand, hasCommand := updateAndRestartCommand(runtime, packageName)
	canAutoRestart := mode == "auto" && hasCommand
	if canAutoRestart {
		_ = h.spawnChild(updateCommand)
	}
	// Resposta sai antes do exit agendado de 1,5 s.
	h.exitSoon()
	response := map[string]any{
		"success": true, "runtime": runtime, "mode": mode, "autoRestart": canAutoRestart,
	}
	fallback := "npm i -g " + packageName + "@latest"
	if canAutoRestart {
		response["autoRestartCommand"] = updateCommand
		response["message"] = "Update scheduled. Server will exit in ~1.5s, then " + runtime + " will install v" + packageName + "@latest and restart automatically."
	} else {
		if hasCommand {
			response["installCommand"] = updateCommand
		} else {
			response["installCommand"] = nil
		}
		if runtime == "direct" {
			response["message"] = "Shutting down. Server is running in foreground — manually restart with: `" + packageName + "`"
		} else {
			shown := updateCommand
			if shown == "" {
				shown = fallback
			}
			response["message"] = "Shutting down. After exit, run this command to update: `" + shown + "`"
		}
	}
	nodeReply(w, 200, response)
}

// defaultSpawnShutdownChild: bash destacado, sem stdio, sem env de PM2.
func defaultSpawnShutdownChild(command string) bool {
	env := []string{}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PM2_HOME_PATH=") || strings.HasPrefix(entry, "pm2_env=") || strings.HasPrefix(entry, "PM2_JSON_PROCESSING=") {
			continue
		}
		env = append(env, entry)
	}
	cmd := exec.Command("bash", "-c", "sleep 2 && "+command)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { _ = cmd.Process.Release() }()
	return true
}

func ttsStringOr(value any, fallback string) string {
	if output := ttsString(value); output != "" {
		return output
	}
	return fallback
}
