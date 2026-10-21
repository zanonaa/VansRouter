package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// headroomHarness injeta binário falso, spawn real (sleep) para pid vivo e
// settings de teste. Devolve também o pid do filho para cleanup.
func headroomHarness(t *testing.T, binary string) (*Headroom, *HeadroomRoutes, *[]string) {
	t.Helper()
	spawns := &[]string{}
	child := cmdWithPID(t)
	headroom := &Headroom{
		DataDir:     t.TempDir(),
		StartupWait: 150 * time.Millisecond,
		FindBinary:  func() string { return binary },
		FindPython:  func() string { return "" },
		Status: func(url string) map[string]any {
			return map[string]any{"installed": binary != "", "running": false, "python": "",
				"localUrl": true, "canStart": binary != "", "version": nil, "extras": map[string]bool{"code": false, "ml": false}}
		},
		Spawn: func(binaryPath string, args []string, logFile string) (int, error) {
			*spawns = append(*spawns, strings.Join(args, " "))
			return child(), nil
		},
	}
	routes := &HeadroomRoutes{H: headroom, Read: func(context.Context) (map[string]any, error) {
		return map[string]any{"headroomCodeAware": true, "headroomKompress": false, "headroomUrl": "http://localhost:8899"}, nil
	}}
	return headroom, routes, spawns
}

func cmdWithPID(t *testing.T) func() int {
	t.Helper()
	var pid int
	return func() int {
		if pid != 0 {
			return pid
		}
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		pid = cmd.Process.Pid
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return pid
	}
}

func TestHeadroomGateAndLifecycle(t *testing.T) {
	_, routes, spawns := headroomHarness(t, "/tmp/opencode/fake-headroom/fake-headroom")

	w := httptest.NewRecorder()
	routes.Start(w, httptest.NewRequest("POST", "/api/headroom/start", nil))
	if w.Code != 200 {
		t.Fatalf("start %d %s", w.Code, w.Body.String())
	}
	if len(*spawns) != 1 {
		t.Fatalf("spawn %v", *spawns)
	}
	if (*spawns)[0] != "proxy --port 8899 --code-aware --disable-kompress" {
		t.Fatalf("args %v", (*spawns)[0])
	}
	w = httptest.NewRecorder()
	routes.Start(w, httptest.NewRequest("POST", "/api/headroom/start", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"alreadyRunning":true`) {
		t.Fatalf("second start %d %s", w.Code, w.Body.String())
	}
	if len(*spawns) != 1 {
		t.Fatal("segundo start spawnou de novo")
	}
	w = httptest.NewRecorder()
	routes.Stop(w, httptest.NewRequest("POST", "/api/headroom/stop", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"stopped":true`) {
		t.Fatalf("stop %d %s", w.Code, w.Body.String())
	}
	var stop map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &stop)
	w = httptest.NewRecorder()
	routes.Stop(w, httptest.NewRequest("POST", "/api/headroom/stop", nil))
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"reason":"not_running"`) {
		t.Fatalf("second stop %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	routes.Get(w, httptest.NewRequest("GET", "/api/headroom/status", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "http://localhost:8899") || !strings.Contains(w.Body.String(), `"managedPid":null`) {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestHeadroomExternalURLGate(t *testing.T) {
	_, routes, spawns := headroomHarness(t, "/bin/fake")
	routes.Read = func(context.Context) (map[string]any, error) {
		return map[string]any{"headroomUrl": "https://headroom.remote.example"}, nil
	}
	w := httptest.NewRecorder()
	routes.Start(w, httptest.NewRequest("POST", "/api/headroom/start", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "EXTERNAL_PROXY") || !strings.Contains(w.Body.String(), "External Headroom proxies must be started outside 9Router") {
		t.Fatalf("external gate %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	routes.Restart(w, httptest.NewRequest("POST", "/api/headroom/restart", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "EXTERNAL_PROXY") {
		t.Fatalf("external restart %d %s", w.Code, w.Body.String())
	}
	if len(*spawns) != 0 {
		t.Fatal("spawn com URL externa")
	}
}

func TestHeadroomNotInstalled(t *testing.T) {
	_, routes, _ := headroomHarness(t, "")
	w := httptest.NewRecorder()
	routes.Start(w, httptest.NewRequest("POST", "/api/headroom/start", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"NOT_INSTALLED"`) || !strings.Contains(w.Body.String(), "Headroom CLI not installed") {
		t.Fatalf("not installed %d %s", w.Code, w.Body.String())
	}
}

func TestHeadroomStatusReport(t *testing.T) {
	_, routes, _ := headroomHarness(t, "/tmp/fake-headroom/fake")
	w := httptest.NewRecorder()
	routes.Get(w, httptest.NewRequest("GET", "/api/headroom/status", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"canStart":true`) || !strings.Contains(w.Body.String(), `"installed":true`) {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestHeadroomExtrasContracts(t *testing.T) {
	headroom, routes, _ := headroomHarness(t, "/tmp/fake-headroom/fake")
	// python sintético válido extras: a validação acontece antes do pip.
	headroom.FindPython = func() string { return "/bin/true" }
	if len(headroomExtras) != 2 || headroomExtras[0] != "code" || headroomExtras[1] != "ml" {
		t.Fatal("HEADROOM_COMPRESSION_EXTRAS divergiu")
	}
	if _, markerOK := headroomExtraMarkers["code"]; !markerOK || headroomExtraMarkers["code"][0] != "tree-sitter" {
		t.Fatal("EXTRA_MARKERS divergiu")
	}
	w := httptest.NewRecorder()
	routes.ExtrasDelete(w, httptest.NewRequest("DELETE", "/api/headroom/extras", strings.NewReader(`{"extras":["nao-valida"]}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"INVALID_EXTRAS"`) || !strings.Contains(w.Body.String(), "No valid extras to remove") {
		t.Fatalf("invalid extras %d %s", w.Code, w.Body.String())
	}
	// GET log vazio antes de qualquer install.
	w = httptest.NewRecorder()
	routes.ExtrasGet(w, httptest.NewRequest("GET", "/api/headroom/extras?log=1", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"log":""`) {
		t.Fatalf("log tail %d %s", w.Code, w.Body.String())
	}
	// GET extras: available = HEADROOM_COMPRESSION_EXTRAS.
	w = httptest.NewRecorder()
	routes.ExtrasGet(w, httptest.NewRequest("GET", "/api/headroom/extras", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"available":["code","ml"]`) {
		t.Fatalf("extras listing %d %s", w.Code, w.Body.String())
	}
	// NO_PYTHON → 400 em DELETE (rota original).
	failingPython := &Headroom{DataDir: headroom.DataDir, FindPython: func() string { return "" }}
	routesFailing := &HeadroomRoutes{H: failingPython}
	w = httptest.NewRecorder()
	routesFailing.ExtrasDelete(w, httptest.NewRequest("DELETE", "/api/headroom/extras", strings.NewReader(`{"extras":["code"]}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"NO_PYTHON"`) {
		t.Fatalf("no python %d %s", w.Code, w.Body.String())
	}
}
