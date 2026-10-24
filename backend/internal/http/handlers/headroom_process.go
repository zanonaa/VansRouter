package handlers

// Espelho de src/lib/headroom/process.js: pilha de PID, log e extras.
import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	headroomDefaultPort   = 8787
	headroomStartupWait   = 8 * time.Second
	headroomGraceWait     = 2 * time.Second
	headroomRestartPoll   = 30
	headroomRestartPollMs = 100 * time.Millisecond
	headroomLogTailLines  = 200
	headroomInstallTail   = 15
)

// Headroom maneja o ciclo do proxy local (injetável para testes).
type Headroom struct {
	// DataDir é o mesmo DATA_DIR do app (config.Load).
	DataDir string
	// Client para o /health.
	Client *http.Client
	// StartupWait configurável; produção usa headroomStartupWait.
	StartupWait time.Duration
	// FindBinary/FindPython/Extras/Status/Probe podem ser substituídos por testes.
	FindBinary func() string
	FindPython func() string
	Probe      func(url string) bool
	Status     func(url string) map[string]any
	// Spawn substitui exec do binário (testes); default spawn discreto.
	Spawn func(binary string, args []string, logFile string) (int, error)
}

func (h *Headroom) dir() string         { return h.DataDir + "/headroom" }
func (h *Headroom) pidFile() string     { return h.dir() + "/proxy.pid" }
func (h *Headroom) logFile() string     { return h.dir() + "/proxy.log" }
func (h *Headroom) installFile() string { return h.dir() + "/install.log" }

func (h *Headroom) binary() string {
	if h.FindBinary != nil {
		return h.FindBinary()
	}
	return findHeadroomBinary()
}

func (h *Headroom) python() string {
	if h.FindPython != nil {
		return h.FindPython()
	}
	return findPython310()
}

func (h *Headroom) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: headroomHealthTimeout}
}

func (h *Headroom) readPid() (int, bool) {
	raw, err := os.ReadFile(h.pidFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func (h *Headroom) writePid(pid int) {
	if err := os.MkdirAll(h.dir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(h.pidFile(), []byte(strconv.Itoa(pid)), 0o644)
}

func (h *Headroom) clearPid() { _ = os.Remove(h.pidFile()) }

// isPidAlive: probe não fatal equivalente a process.kill(pid, 0).
func isPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func (h *Headroom) getManagedPid() int {
	pid, ok := h.readPid()
	if !ok || !isPidAlive(pid) {
		return 0
	}
	return pid
}

func extrasProxyArgs(codeAware bool, disableKompress bool) []string {
	var args []string
	if codeAware {
		args = append(args, "--code-aware")
	}
	if disableKompress {
		args = append(args, "--disable-kompress")
	}
	return args
}

func (h *Headroom) defaultSpawn(binary string, args []string, logFile string) (int, error) {
	out, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, &headroomError{"SPAWN_FAILED", "Failed to spawn headroom proxy"}
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil
	hideHeadroomWindow(cmd)
	if err := cmd.Start(); err != nil {
		_ = out.Close()
		return 0, &headroomError{"SPAWN_FAILED", "Failed to spawn headroom proxy"}
	}
	go func() { _ = cmd.Process.Release() }()
	pid := cmd.Process.Pid
	_ = out.Close()
	return pid, nil
}

func (h *Headroom) startHeadroomProxy(port int, codeAware bool, disableKompress bool) (int, bool, error) {
	if port <= 0 || port >= 65536 {
		port = headroomDefaultPort
	}
	binary := h.binary()
	if binary == "" {
		return 0, false, &headroomError{"NOT_INSTALLED", "Headroom CLI not installed"}
	}
	if existing := h.getManagedPid(); existing != 0 {
		return existing, true, nil
	}
	if err := os.MkdirAll(h.dir(), 0o755); err != nil {
		return 0, false, &headroomError{"SPAWN_FAILED", "Failed to spawn headroom proxy"}
	}
	args := append([]string{"proxy", "--port", strconv.Itoa(port)}, extrasProxyArgs(codeAware, disableKompress)...)
	spawn := h.Spawn
	if spawn == nil {
		spawn = h.defaultSpawn
	}
	pid, err := spawn(binary, args, h.logFile())
	if err != nil {
		return 0, false, err
	}
	h.writePid(pid)
	startupWait := h.StartupWait
	if startupWait <= 0 {
		startupWait = headroomStartupWait
	}
	deadline := time.Now().Add(startupWait)
	for time.Now().Before(deadline) {
		if !isPidAlive(pid) {
			h.clearPid()
			return 0, false, &headroomError{"EARLY_EXIT", "headroom proxy exited early — see proxy.log"}
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Prazo atingido com o processo vivo = sucesso (startup wait da origem).
	return pid, false, nil
}

func (h *Headroom) stopHeadroomProxy() map[string]any {
	pid := h.getManagedPid()
	if pid == 0 {
		return map[string]any{"stopped": false, "reason": "not_running"}
	}
	if process, err := os.FindProcess(pid); err == nil {
		_ = process.Signal(syscall.SIGTERM)
		go func() {
			time.Sleep(headroomGraceWait)
			if isPidAlive(pid) {
				_ = process.Signal(syscall.SIGKILL)
			}
		}()
	}
	h.clearPid()
	return map[string]any{"stopped": true, "pid": pid}
}

func (h *Headroom) restartHeadroomProxy(port int, codeAware bool, disableKompress bool) (int, bool, error) {
	pid := h.getManagedPid()
	if pid != 0 {
		if process, err := os.FindProcess(pid); err == nil {
			_ = process.Signal(syscall.SIGTERM)
		}
		for i := 0; i < headroomRestartPoll && isPidAlive(pid); i++ {
			time.Sleep(headroomRestartPollMs)
		}
		if isPidAlive(pid) {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Signal(syscall.SIGKILL)
			}
			time.Sleep(300 * time.Millisecond)
		}
		h.clearPid()
	}
	return h.startHeadroomProxy(port, codeAware, disableKompress)
}

func (h *Headroom) managedStatus(url string) map[string]any {
	var aggregate map[string]any
	if h.Status != nil {
		aggregate = h.Status(url)
	} else {
		aggregate = headroomStatus(h.client(), url)
	}
	pid := h.getManagedPid()
	var managed any
	if pid != 0 {
		managed = pid
	}
	aggregate["managedPid"] = managed
	// A origem mescla `url` sobre o status agregado.
	aggregate["url"] = url
	return aggregate
}

// installHeadroomExtras roda pip install headroom-ai[proxy,...extras] com log.
func (h *Headroom) installHeadroomExtras(requested []string) (map[string]any, error) {
	accepted := []string{}
	for _, extra := range requested {
		for _, known := range headroomExtras {
			if extra == known {
				accepted = append(accepted, extra)
			}
		}
	}
	python := h.python()
	if python == "" {
		return nil, &headroomError{"NO_PYTHON", "Python >= 3.10 not found"}
	}
	if h.binary() == "" {
		return nil, &headroomError{"NOT_INSTALLED", "headroom-ai not installed (run `pip install headroom-ai[proxy]` first)"}
	}
	spec := fmt.Sprintf("headroom-ai[%s]", strings.Join(append([]string{"proxy"}, accepted...), ","))
	if err := os.MkdirAll(h.dir(), 0o755); err != nil {
		return nil, err
	}
	out, err := os.OpenFile(h.installFile(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	cmd := exec.Command(python, "-m", "pip", "install", "--upgrade", spec)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil
	hideHeadroomWindow(cmd)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			code := exitError.ExitCode()
			return nil, &headroomError{"INSTALL_FAILED", fmt.Sprintf("pip install exited with code=%d — see headroom/install.log", code)}
		}
		return nil, err
	}
	installed, version, headroomExtrasStatus := getInstalledHeadroomExtras(h.python())
	// A origem espalha status por último, sobrescrevendo "extras" com o
	// estado de markers; o retorno final não expõe a lista solicitada.
	result := map[string]any{"success": true, "code": 0, "spec": spec,
		"installed": installed, "version": version, "extras": headroomExtrasStatus}
	return result, nil
}

func (h *Headroom) uninstallHeadroomExtras(requested []string) (map[string]any, error) {
	accepted := []string{}
	for _, extra := range requested {
		for _, known := range headroomExtras {
			if extra == known {
				accepted = append(accepted, extra)
			}
		}
	}
	python := h.python()
	if python == "" {
		return nil, &headroomError{"NO_PYTHON", "Python >= 3.10 not found"}
	}
	packages := map[string]bool{}
	pkgs := []string{}
	for _, extra := range accepted {
		for _, marker := range headroomExtraMarkers[extra] {
			if !packages[marker] {
				packages[marker] = true
				pkgs = append(pkgs, marker)
			}
		}
	}
	if len(pkgs) == 0 {
		return nil, &headroomError{"INVALID_EXTRAS", "No valid extras to remove"}
	}
	if err := os.MkdirAll(h.dir(), 0o755); err != nil {
		return nil, err
	}
	out, err := os.OpenFile(h.installFile(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	cmd := exec.Command(python, append([]string{"-m", "pip", "uninstall", "-y"}, pkgs...)...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil
	hideHeadroomWindow(cmd)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, &headroomError{"UNINSTALL_FAILED", fmt.Sprintf("pip uninstall exited with code=%d — see headroom/install.log", exitError.ExitCode())}
		}
		return nil, err
	}
	installed, version, headroomExtrasStatus := getInstalledHeadroomExtras(h.python())
	return map[string]any{"success": true, "code": 0, "removed": pkgs,
		"installed": installed, "version": version, "extras": headroomExtrasStatus}, nil
}

// getLogTail devolve as últimas N linhas não vazias do arquivo.
func (h *Headroom) getLogTail(name string, max int) string {
	content, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	return strings.Join(lines, "\n")
}

type headroomError struct {
	Code    string
	Message string
}

func (e *headroomError) Error() string { return e.Message }

func hideHeadroomWindow(cmd *exec.Cmd) { /* unix: sem janela */ }
