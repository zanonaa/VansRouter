package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func pxpipeTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func pxpipeFakeInstall(t *testing.T, dataDir, version string) {
	t.Helper()
	root := pxpipePackageRoot(dataDir)
	if err := os.MkdirAll(filepath.Join(root, "dist", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := "{}"
	if version != "" {
		pkg = `{"version":"` + version + `"}`
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pxpipeLibraryEntry(dataDir), []byte("export const transformAnthropicMessages = () => ({})"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPxpipeInstallInfoShape(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	info := pxpipeInstallInfo(dataDir)
	if info["installed"] != false || info["version"] != nil || info["path"] != nil {
		t.Fatalf("missing install %v", info)
	}
	pxpipeFakeInstall(t, dataDir, "1.2.3")
	info = pxpipeInstallInfo(dataDir)
	if info["installed"] != true || info["version"] != "1.2.3" {
		t.Fatalf("install %v", info)
	}
	if !strings.Contains(info["path"].(string), "pxpipe-proxy") {
		t.Fatalf("path %v", info["path"])
	}
	// package.json sem version → version null.
	dataDir2 := pxpipeTestDir(t)
	pxpipeFakeInstall(t, dataDir2, "")
	info = pxpipeInstallInfo(dataDir2)
	if info["installed"] != true || info["version"] != nil {
		t.Fatalf("no version %v", info)
	}
	// Entry faltando = não instalado.
	if err := os.Remove(pxpipeLibraryEntry(dataDir2)); err != nil {
		t.Fatal(err)
	}
	if installed, _ := pxpipeInstallInfo(dataDir2)["installed"].(bool); installed {
		t.Fatal("sem entry deveria estar desinstalado")
	}
}

func TestPxpipeRunInstallNpmMissing(t *testing.T) {
	if _, err := pxpipeRunInstall(context.Background(), pxpipeTestDir(t), func() string { return "" }); err == nil {
		t.Fatal("esperava NPM_NOT_FOUND")
	} else if coded, ok := err.(*pxpipeError); !ok || coded.Code != "NPM_NOT_FOUND" {
		t.Fatalf("code %T %v", err, err)
	} else if coded.Msg != "npm not found on PATH — Node.js/npm is required to install PXPIPE" {
		t.Fatalf("message %q", coded.Msg)
	}
}

func TestPxpipeInstallSerialized(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	var mu sync.Mutex
	runs := 0
	run := func(ctx context.Context, dir string) (map[string]any, error) {
		mu.Lock()
		runs++
		mu.Unlock()
		return map[string]any{"installed": true}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := pxpipeState.pxpipeInstall(context.Background(), dataDir, run); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// As 3 chamadas executam (serializadas, nunca sobrepostas).
	if runs != 3 {
		t.Fatalf("runs %d", runs)
	}
	if PxpipeIsInstalling() {
		t.Fatal("isInstalling residual")
	}
}

func TestPxpipeStatusShape(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	p := &Pxpipe{
		DataDir: dataDir,
		Read: func(context.Context) (map[string]any, error) {
			return map[string]any{"pxpipeEnabled": 1, "pxpipeAutoInstall": 0, "pxpipeMinChars": 600, "pxpipeTimeoutMs": 2000}, nil
		},
		Loader:     &PxpipeLoader{DataDir: dataDir},
		FindNpm:    func() string { return "/usr/bin/npm" },
		RunInstall: func(context.Context, string) (map[string]any, error) { return map[string]any{"installed": true}, nil },
	}
	w := httptest.NewRecorder()
	p.Status(w, httptest.NewRequest("GET", "/api/pxpipe/status", nil))
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["mode"] != "library" || payload["installed"] != false || payload["running"] != false || payload["npmAvailable"] != true {
		t.Fatalf("shape %v", payload)
	}
	if payload["enabled"] != true || payload["autoInstall"] != false || payload["minChars"] != float64(600) || payload["timeoutMs"] != float64(2000) {
		t.Fatalf("settings %v", payload)
	}
	// Chaves ausentes nas settings são omitidas (undefined do JS).
	p2 := &Pxpipe{DataDir: dataDir, Read: func(context.Context) (map[string]any, error) {
		return map[string]any{}, nil
	}, Loader: &PxpipeLoader{DataDir: dataDir}, FindNpm: func() string { return "" }}
	w = httptest.NewRecorder()
	p2.Status(w, httptest.NewRequest("GET", "/api/pxpipe/status", nil))
	body := w.Body.String()
	if strings.Contains(body, "minChars") || strings.Contains(body, "timeoutMs") {
		t.Fatalf("chaves ausentes %s", body)
	}
	if !strings.Contains(body, `"npmAvailable":false`) {
		t.Fatalf("npm %s", body)
	}
}

func TestPxpipeLoaderLifecycle(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	loader := &PxpipeLoader{DataDir: dataDir}
	// NOT_INSTALLED.
	err := loader.Load(context.Background())
	var coded *pxpipeError
	if !errors.As(err, &coded) || coded.Code != "NOT_INSTALLED" || coded.Msg != "PXPIPE is not installed" {
		t.Fatalf("load %v", err)
	}
	pxpipeFakeInstall(t, dataDir, "3.4.5")
	// Export check falha → message exata.
	loader.RunScript = func(context.Context, string) (string, error) {
		return "", errors.New("exit 1")
	}
	err = loader.Load(context.Background())
	if err == nil || err.Error() != "installed pxpipe package does not export transformAnthropicMessages" {
		t.Fatalf("export %v", err)
	}
	if loader.info().Loaded {
		t.Fatal("não deveria ter carregado")
	}
	// Sucesso: cache ligado e status running com loadedAt.
	loader.RunScript = func(context.Context, string) (string, error) { return "function", nil }
	start := time.Now()
	if err := loader.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	loaded := loader.info()
	if !loaded.Loaded || loaded.Version != "3.4.5" || loaded.LoadedAt < start.UnixMilli() {
		t.Fatalf("loaded %+v", loaded)
	}
	// Unload pela rota stop: stopped false (nunca carregou) e true depois.
	p := &Pxpipe{DataDir: dataDir, Loader: loader, Read: func(context.Context) (map[string]any, error) {
		return map[string]any{}, nil
	}, FindNpm: func() string { return "" }}
	w := httptest.NewRecorder()
	p.Stop(w, httptest.NewRequest("POST", "/api/pxpipe/stop", nil))
	if w.Code != 200 {
		t.Fatalf("stop %d", w.Code)
	}
	var payload struct {
		Stopped bool `json:"stopped"`
		Running bool `json:"running"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if !payload.Stopped || payload.Running {
		t.Fatalf("stop payload %+v", payload)
	}
	w = httptest.NewRecorder()
	p.Stop(w, httptest.NewRequest("POST", "/api/pxpipe/stop", nil))
	payload = struct {
		Stopped bool `json:"stopped"`
		Running bool `json:"running"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload.Stopped || payload.Running {
		t.Fatalf("stop duplo %+v", payload)
	}
}

func TestPxpipeSelfTestHook(t *testing.T) {
	loader := &PxpipeLoader{DataDir: pxpipeTestDir(t)}
	pxpipeFakeInstall(t, loader.DataDir, "1.0.0")
	loader.RunScript = func(context.Context, string) (string, error) {
		return `{"reason":"passthrough_unsupported"}`, nil
	}
	result, err := loader.SelfTest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || result["reason"] != "passthrough_unsupported" {
		t.Fatalf("selftest %v", result)
	}
	if ms, ok := result["durationMs"].(int64); !ok || ms < 0 {
		t.Fatalf("duration %v", result["durationMs"])
	}
}

func TestPxpipeHealthPipeline(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	p := &Pxpipe{
		DataDir: dataDir,
		Loader:  &PxpipeLoader{DataDir: dataDir},
		FindNpm: func() string { return "" },
	}
	// Não instalado: primeira checagem ok:false e erro literal.
	result := p.health(context.Background())
	if result["healthy"] != false || result["error"] != "pxpipe not installed" {
		t.Fatalf("health %v", result)
	}
	checks := result["checks"].([]map[string]any)
	if len(checks) != 1 || checks[0]["id"] != "installed" || checks[0]["ok"] != false {
		t.Fatalf("checks %v", checks)
	}
	// Instalado, export check ok, self-test ok: 3 checagens em ordem.
	pxpipeFakeInstall(t, dataDir, "9.9.9")
	p.Loader.RunScript = func(ctx context.Context, script string) (string, error) {
		if strings.Contains(script, "TextEncoder().encode") {
			return `{"reason":"compressed"}`, nil
		}
		return "function", nil
	}
	result = p.health(context.Background())
	if result["healthy"] != true || result["error"] != nil {
		t.Fatalf("health ok %v", result)
	}
	checks = result["checks"].([]map[string]any)
	if len(checks) != 3 {
		t.Fatalf("checks %v", checks)
	}
	ids := []string{}
	for _, check := range checks {
		ids = append(ids, check["id"].(string))
		if check["ok"] != true {
			t.Fatalf("check %v", check)
		}
	}
	if strings.Join(ids, ",") != "installed,module,transform" {
		t.Fatalf("ids %v", ids)
	}
	if !strings.Contains(checks[2]["detail"].(string), "ms (compressed)") {
		t.Fatalf("detail %v", checks[2]["detail"])
	}
	// Export check falha → module ok:false + erro Cannot load.
	p.Loader.RunScript = func(context.Context, string) (string, error) {
		return "", errors.New("boom-export")
	}
	result = p.health(context.Background())
	if result["error"] != "Cannot load module: installed pxpipe package does not export transformAnthropicMessages" {
		t.Fatalf("module fail %v", result)
	}
	// Self-test falha → Self-test failed.
	p.Loader.RunScript = func(ctx context.Context, script string) (string, error) {
		if strings.Contains(script, "TextEncoder().encode") {
			return "", errors.New("module exploded")
		}
		return "function", nil
	}
	result = p.health(context.Background())
	if result["error"] != "Self-test failed: module exploded" {
		t.Fatalf("selftest fail %v", result)
	}
}

func TestPxpipeInstallRoute(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	p := &Pxpipe{
		DataDir: dataDir,
		Loader:  &PxpipeLoader{DataDir: dataDir},
		FindNpm: func() string { return "/usr/bin/npm" },
		RunInstall: func(context.Context, string) (map[string]any, error) {
			pxpipeFakeInstall(t, dataDir, "5.6.7")
			return pxpipeInstallInfo(dataDir), nil
		},
	}
	w := httptest.NewRecorder()
	p.Install(w, httptest.NewRequest("POST", "/api/pxpipe/install", nil))
	if w.Code != 200 {
		t.Fatalf("install %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		Installed bool `json:"installed"`
		Version   any  `json:"version"`
		Health    struct {
			Healthy bool    `json:"healthy"`
			Error   *string `json:"error"`
		} `json:"health"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Installed || payload.Version != "5.6.7" {
		t.Fatalf("install payload %v", payload)
	}
	// NPM ausente → 500 com code.
	p2 := &Pxpipe{DataDir: pxpipeTestDir(t), FindNpm: func() string { return "" }}
	w = httptest.NewRecorder()
	p2.Install(w, httptest.NewRequest("POST", "/api/pxpipe/install", nil))
	if w.Code != 500 {
		t.Fatalf("npm missing %d %s", w.Code, w.Body.String())
	}
	var failure struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &failure)
	if failure.Code != "NPM_NOT_FOUND" {
		t.Fatalf("code %q", failure.Code)
	}
}

func TestPxpipeStartRoute(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	settings := map[string]any{}
	p := &Pxpipe{
		DataDir: dataDir,
		Read:    func(context.Context) (map[string]any, error) { return settings, nil },
		Loader: &PxpipeLoader{DataDir: dataDir, RunScript: func(context.Context, string) (string, error) {
			return "function", nil
		}},
		FindNpm: func() string { return "" },
	}
	// Não instalado + auto-install off → 409 literal.
	w := httptest.NewRecorder()
	p.Start(w, httptest.NewRequest("POST", "/api/pxpipe/start", nil))
	if w.Code != 409 {
		t.Fatalf("start %d %s", w.Code, w.Body.String())
	}
	var conflict struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &conflict)
	if conflict.Error != "PXPIPE is not installed" || conflict.Code != "NOT_INSTALLED" {
		t.Fatalf("conflict %+v", conflict)
	}
	// auto-install on → instala pelo hook e carrega.
	settings["pxpipeAutoInstall"] = true
	installed := false
	p.RunInstall = func(context.Context, string) (map[string]any, error) {
		installed = true
		pxpipeFakeInstall(t, dataDir, "2.0.0")
		return pxpipeInstallInfo(dataDir), nil
	}
	w = httptest.NewRecorder()
	p.Start(w, httptest.NewRequest("POST", "/api/pxpipe/start", nil))
	if w.Code != 200 || !installed {
		t.Fatalf("auto install %d %s", w.Code, w.Body.String())
	}
	var started map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &started)
	if started["running"] != true || started["version"] != "2.0.0" {
		t.Fatalf("started %v", started)
	}
	// Restart recarrega: unload + load ok.
	w = httptest.NewRecorder()
	p.Restart(w, httptest.NewRequest("POST", "/api/pxpipe/restart", nil))
	if w.Code != 200 {
		t.Fatalf("restart %d %s", w.Code, w.Body.String())
	}
	// Restart com export check falho → 500 {error}.
	p.Loader.RunScript = func(context.Context, string) (string, error) { return "", errors.New("x") }
	w = httptest.NewRecorder()
	p.Restart(w, httptest.NewRequest("POST", "/api/pxpipe/restart", nil))
	if w.Code != 500 {
		t.Fatalf("restart fail %d", w.Code)
	}
	var restartErr struct {
		Error string `json:"error"`
		Code  any    `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &restartErr)
	if restartErr.Error != "installed pxpipe package does not export transformAnthropicMessages" || restartErr.Code != nil {
		t.Fatalf("restart err %+v", restartErr)
	}
}

func TestPxpipeLogsRoute(t *testing.T) {
	dataDir := pxpipeTestDir(t)
	// events.jsonl.1 e atual com ordem embaralhada.
	if err := os.MkdirAll(pxpipeDir(dataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	rotated := "{\"ts\":10,\"applied\":true,\"tokensSavedEst\":7}\n{\"ts\":30,\"applied\":false,\"reason\":\"transform_error\"}\n"
	current := "{\"ts\":20,\"applied\":false}\n{\"ts\":5,\"applied\":true}\n"
	if err := os.WriteFile(pxpipeRotatedFile(dataDir), []byte(rotated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pxpipeEventsFile(dataDir), []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pxpipeInstallLogFile(dataDir), []byte(strings.Repeat("linha\n", 250)), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Pxpipe{DataDir: dataDir, Loader: &PxpipeLoader{DataDir: dataDir}}
	w := httptest.NewRecorder()
	p.Logs(w, httptest.NewRequest("GET", "/api/pxpipe/logs?limit=3", nil))
	if w.Code != 200 {
		t.Fatalf("logs %d", w.Code)
	}
	var payload struct {
		InstallLog string           `json:"installLog"`
		Events     []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	// Tail de 200 linhas e eventos em ordem inversa (mais novo primeiro).
	if strings.Count(payload.InstallLog, "\n") != 199 {
		t.Fatalf("tail %d", strings.Count(payload.InstallLog, "\n"))
	}
	tsOrder := []float64{}
	for _, ev := range payload.Events {
		tsOrder = append(tsOrder, ev["ts"].(float64))
	}
	if fmtTs(tsOrder) != "30,20,10" {
		t.Fatalf("ts %v (recortados os 3 mais novos: 20,10,5... limit=3 corta 30?)", tsOrder)
	}
}

func fmtTs(values []float64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatFloat(value, 'f', -1, 64))
	}
	return strings.Join(parts, ",")
}

func mustJSON(value map[string]any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
