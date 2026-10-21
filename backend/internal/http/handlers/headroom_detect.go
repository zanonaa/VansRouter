package handlers

// Espelho de src/lib/headroom/detect.js: binário, python, extras e status.
import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	headroomExtras        = []string{"code", "ml"}
	headroomExtraMarkers  = map[string][]string{"code": {"tree-sitter", "tree-sitter-language-pack"}, "ml": {"torch", "huggingface-hub"}}
	headroomPipTimeout    = 8 * time.Second
	headroomHealthTimeout = 1500 * time.Millisecond
	headroomLoopbackHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "[::1]": true, "0.0.0.0": true}
	headroomPythonNames   = []string{"python3.13", "python3.12", "python3.11", "python3.10", "python3", "python"}
)

func defaultHeadroomURL() string {
	if value := os.Getenv("HEADROOM_URL"); value != "" {
		return value
	}
	return "http://localhost:8787"
}

// headroomExtendedPath replica EXTRA_BINS + PATH (separador da plataforma).
func headroomExtendedPath() string {
	bins := extraBinDirs()
	windows := runtime.GOOS == "windows"
	extra := make([]string, 0, len(bins)+1)
	if windows {
		local := os.Getenv("LOCALAPPDATA")
		app := os.Getenv("APPDATA")
		extra = append(extra,
			filepath.Join(local, "Programs", "Python", "Python313", "Scripts"),
			filepath.Join(local, "Programs", "Python", "Python312", "Scripts"),
			filepath.Join(local, "Programs", "Python", "Python311", "Scripts"),
			filepath.Join(local, "Programs", "Python", "Python310", "Scripts"),
			filepath.Join(app, "Python", "Python313", "Scripts"),
		)
	} else {
		extra = append(extra, bins...)
	}
	if value := os.Getenv("PATH"); value != "" {
		extra = append(extra, value)
	}
	return strings.Join(extra, string(filepath.ListSeparator))
}

func extraBinDirs() []string {
	bins := []string{"/usr/local/bin", "/opt/homebrew/bin",
		"/Library/Frameworks/Python.framework/Versions/3.13/bin",
		"/Library/Frameworks/Python.framework/Versions/3.12/bin",
		"/Library/Frameworks/Python.framework/Versions/3.11/bin",
		"/Library/Frameworks/Python.framework/Versions/3.10/bin",
	}
	if home, err := os.UserHomeDir(); err == nil {
		bins = append(bins, filepath.Join(home, ".local/bin"))
	}
	return append(bins, "/usr/bin", "/bin")
}

// runHeadroomCommand = execSync/execFile com PATH estendido e stdio ignorado.
func runHeadroomCommand(name string, args []string, timeout time.Duration) (string, bool) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "PATH="+headroomExtendedPath())
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	cmd.Stdin = nil
	if timeout > 0 {
		timer := time.AfterFunc(timeout, func() { _ = cmd.Process.Kill() })
		defer timer.Stop()
	}
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return out.String(), true
}

// findHeadroomBinary detecta o CLI `headroom` com which/where.
func findHeadroomBinary() string {
	whichCmd := "which"
	if runtime.GOOS == "windows" {
		whichCmd = "where"
	}
	out, ok := runHeadroomCommand(whichCmd, []string{"headroom"}, 0)
	if !ok {
		return ""
	}
	line := strings.SplitN(strings.TrimSpace(out), "\n", 2)[0]
	return strings.TrimSpace(line)
}

// buildPythonCandidates explora o python vizinho do binário, depois bins e PATH.
func buildPythonCandidates(binary string) []string {
	var list []string
	windows := runtime.GOOS == "windows"
	if binary != "" {
		dir := filepath.Dir(binary)
		names := []string{"python3", "python3.13", "python"}
		if windows {
			names = []string{"python.exe", "python3.exe"}
		}
		for _, name := range names {
			list = append(list, filepath.Join(dir, name))
		}
	}
	var dirs []string
	if windows {
		local := os.Getenv("LOCALAPPDATA")
		app := os.Getenv("APPDATA")
		dirs = []string{
			filepath.Join(local, "Programs", "Python", "Python313", "Scripts"),
			filepath.Join(local, "Programs", "Python", "Python312", "Scripts"),
			filepath.Join(local, "Programs", "Python", "Python311", "Scripts"),
			filepath.Join(local, "Programs", "Python", "Python310", "Scripts"),
			filepath.Join(app, "Python", "Python313", "Scripts"),
		}
	} else {
		dirs = extraBinDirs()
	}
	for _, dir := range dirs {
		for _, name := range headroomPythonNames {
			if windows {
				name += ".exe"
			}
			list = append(list, filepath.Join(dir, name))
		}
	}
	return append(list, headroomPythonNames...)
}

// findPython310 = python >= 3.10 que enxerga headroom-ai; sem o pacote,
// o primeiro elegível.
func findPython310() string {
	var fallback string
	for _, candidate := range buildPythonCandidates(findHeadroomBinary()) {
		version, ok := runHeadroomCommand(candidate, nil, 0)
		if !ok {
			continue
		}
		match := headroomVersionPattern.FindStringSubmatch(version)
		if match == nil {
			continue
		}
		major, _ := strconv.Atoi(match[1])
		minor, _ := strconv.Atoi(match[2])
		if !(major > 3 || (major == 3 && minor >= 10)) {
			continue
		}
		if fallback == "" {
			fallback = candidate
		}
		if _, ok := runHeadroomCommand(candidate, []string{"-m", "pip", "show", "headroom-ai"}, headroomPipTimeout); ok {
			return candidate
		}
	}
	return fallback
}

var headroomVersionPattern = regexp.MustCompile(`(\d+)\.(\d+)`)

// probeProxyRunning: GET {base}/health com timeout de 1,5 s.
func probeProxyRunning(client *http.Client, value string) bool {
	base := strings.TrimSuffix(value, "/")
	req, err := http.NewRequest("GET", base+"/health", nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 300
}

func isLoopbackHeadroomURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return headroomLoopbackHosts[parsed.Hostname()]
}

// headroomStatus agrega installed/running/python/localUrl/canStart/vantagens.
func headroomStatus(client *http.Client, value string) map[string]any {
	binary := findHeadroomBinary()
	python := findPython310()
	installed := binary != ""
	running := probeProxyRunning(client, value)
	localURL := isLoopbackHeadroomURL(value)
	version := ""
	extras := map[string]bool{"code": false, "ml": false}
	if installed {
		_, version, extras = getInstalledHeadroomExtras(python)
	}
	return map[string]any{"installed": installed, "path": binary, "running": running, "python": python,
		"localUrl": localURL, "canStart": installed && localURL, "version": version, "extras": extras}
}

// getInstalledHeadroomExtras usa um único `pip list --format=json`.
func getInstalledHeadroomExtras(python string) (bool, string, map[string]bool) {
	if python == "" {
		return false, "", map[string]bool{"code": false, "ml": false}
	}
	out, ok := runHeadroomCommand(python, []string{"-m", "pip", "list", "--format=json", "--disable-pip-version-check"}, headroomPipTimeout)
	if !ok {
		return false, "", map[string]bool{"code": false, "ml": false}
	}
	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(out), &packages); err != nil {
		return false, "", map[string]bool{"code": false, "ml": false}
	}
	names := map[string]bool{}
	version := ""
	hasHeadroom := false
	for _, pkg := range packages {
		name := strings.ToLower(pkg.Name)
		names[name] = true
		if name == "headroom-ai" {
			hasHeadroom = true
			version = pkg.Version
		}
	}
	if !hasHeadroom {
		return false, "", map[string]bool{"code": false, "ml": false}
	}
	extras := map[string]bool{"code": false, "ml": false}
	for _, extra := range headroomExtras {
		for _, marker := range headroomExtraMarkers[extra] {
			if names[marker] {
				extras[extra] = true
			}
		}
	}
	return true, version, extras
}

func headroomPortFromURL(value string) int {
	for _, part := range strings.Split(value, ":") {
		if port, err := strconv.Atoi(part); err == nil && port > 0 && port < 65536 {
			return port
		}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return 0
	}
	if port <= 0 || port >= 65536 {
		return 0
	}
	return port
}
