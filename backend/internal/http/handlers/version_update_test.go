package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVersionUpdateGateAndFlow(t *testing.T) {
	updater := &VersionUpdate{
		IsProduction:  func() bool { return false },
		SpawnUpdater:  func() { t.Fatal("updater não pode rodar fora de produção") },
		KillProcesses: func() int { t.Fatal("processos não podem ser mortos fora de produção"); return 0 },
	}
	req := httptest.NewRequest("POST", "/api/version/update", nil)
	w := httptest.NewRecorder()
	updater.Post(w, req)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "Update is only available in production build (vansrouter CLI)") {
		t.Fatalf("gate %d %s", w.Code, w.Body.String())
	}

	order := []string{}
	var waited time.Duration
	updater = &VersionUpdate{
		IsProduction: func() bool { return true },
		KillProcesses: func() int {
			order = append(order, "kill")
			return 2
		},
		Sleep:        func(delay time.Duration) { waited = delay },
		SpawnUpdater: func() { order = append(order, "spawn") },
	}
	w = httptest.NewRecorder()
	updater.Post(w, req)
	if w.Code != 200 {
		t.Fatalf("success %d %s", w.Code, w.Body.String())
	}
	if order[0] != "kill" || order[1] != "spawn" || waited != updaterProcessWait {
		t.Fatalf("order %v wait %v", order, waited)
	}
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || response.Message != "Updater started. This app will exit shortly." {
		t.Fatalf("response %s", w.Body.String())
	}
}

func TestCollectAppPidsParsing(t *testing.T) {
	for _, pid := range collectAppPids(99999) {
		if _, err := strconv.Atoi(pid); err != nil {
			t.Fatalf("pid não numérico %q", pid)
		}
	}
}

func TestUpdaterEnvVars(t *testing.T) {
	if updaterNPMPackageName != "vansrouter" || updaterAppPort != 20128 || updaterExitDelayMS != 500 || updaterStatusPort != 20129 {
		t.Fatal("UPDATER_CONFIG divergiu da origem")
	}
}
