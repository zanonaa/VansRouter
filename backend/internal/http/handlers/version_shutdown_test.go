package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestVersionShutdownDirectManual(t *testing.T) {
	exited := false
	shutdown := &VersionShutdown{
		Detect:        func() string { return "direct" },
		KillProcesses: func() int { return 1 },
		Exit:          func() { exited = true },
	}
	w := httptest.NewRecorder()
	shutdown.Post(w, httptest.NewRequest("POST", "/api/version/shutdown", strings.NewReader(`{"mode":"manual"}`)))
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Success            bool    `json:"success"`
		Runtime            string  `json:"runtime"`
		Mode               string  `json:"mode"`
		AutoRestart        bool    `json:"autoRestart"`
		AutoRestartCommand *string `json:"autoRestartCommand"`
		InstallCommand     *string `json:"installCommand"`
		Message            string  `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || response.Runtime != "direct" || response.Mode != "manual" || response.AutoRestart {
		t.Fatalf("shape %+v", response)
	}
	if response.AutoRestartCommand != nil || response.InstallCommand != nil {
		t.Fatalf("comandos %v %v", response.AutoRestartCommand, response.InstallCommand)
	}
	if !strings.Contains(response.Message, "manually restart with: `vansrouter`") {
		t.Fatalf("message %q", response.Message)
	}
	if !exited {
		t.Fatal("exit agendado não disparou")
	}
}

func TestVersionShutdownAutoRestartCommands(t *testing.T) {
	var spawned string
	spawns := 0
	shutdown := &VersionShutdown{
		KillProcesses: func() int { return 0 },
		Spawn:         func(command string) bool { spawned = command; spawns++; return true },
		Exit:          func() {},
	}
	run := func(detect func() string, body string) map[string]any {
		shutdown.Detect = detect
		w := httptest.NewRecorder()
		shutdown.Post(w, httptest.NewRequest("POST", "/api/version/shutdown", strings.NewReader(body)))
		var payload map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &payload)
		return payload
	}
	// pm2 em auto: nome do pm2_env e spawn destacado.
	t.Setenv("pm2_env", `{"name":"minha-app"}`)
	payload := run(func() string { return "pm2" }, `{}`)
	expected := "npm i -g vansrouter@latest --prefer-online && pm2 restart minha-app"
	if payload["autoRestartCommand"] != expected || payload["autoRestart"] != true || spawned != expected || spawns != 1 {
		t.Fatalf("pm2 %v spawned=%q", payload, spawned)
	}
	// systemd: unit derivada do binário do teste.
	payload = run(func() string { return "systemd" }, `{}`)
	command, _ := payload["autoRestartCommand"].(string)
	if !strings.Contains(command, "sudo systemctl restart ") || !strings.HasSuffix(command, ".service") {
		t.Fatalf("systemd %q", command)
	}
	// screen/tmux: sessão destacada.
	payload = run(func() string { return "screen" }, `{}`)
	if command, _ := payload["autoRestartCommand"].(string); !strings.Contains(command, "screen -dmS vansrouter vansrouter") {
		t.Fatalf("screen %q", command)
	}
	payload = run(func() string { return "tmux" }, `{}`)
	if command, _ := payload["autoRestartCommand"].(string); !strings.Contains(command, "tmux new-session -d -s vansrouter 'vansrouter'") {
		t.Fatalf("tmux %q", command)
	}
	// docker em auto: spawn roda e mensagem cita o runtime.
	payload = run(func() string { return "docker" }, `{}`)
	if payload["autoRestart"] != true {
		t.Fatalf("docker %v", payload)
	}
	if message, _ := payload["message"].(string); !strings.Contains(message, "then docker will install vvansrouter@latest") {
		t.Fatalf("docker message %q", message)
	}
	// docker/manual: installCommand com hint e nenhum spawn.
	spawns = 0
	payload = run(func() string { return "docker" }, `{"mode":"manual"}`)
	if payload["autoRestart"] != false {
		t.Fatalf("manual docker %v", payload)
	}
	if command, _ := payload["installCommand"].(string); !strings.Contains(command, "docker compose pull") {
		t.Fatalf("manual docker install %q", command)
	}
	if spawns != 0 {
		t.Fatal("spawn com mode manual")
	}
	// direct/auto: sem auto-restart, installCommand null e mensagem manual.
	payload = run(func() string { return "direct" }, `{}`)
	if payload["autoRestart"] != false {
		t.Fatalf("direct auto %v", payload)
	}
	if value, exists := payload["installCommand"]; !exists || value != nil {
		t.Fatalf("direct installCommand %v (exists=%v)", value, exists)
	}
	if message, _ := payload["message"].(string); !strings.Contains(message, "manually restart with: `vansrouter`") {
		t.Fatalf("direct message %q", message)
	}
}

func TestDetectRuntimeMarkers(t *testing.T) {
	if value, exists := os.LookupEnv("TMUX"); exists && value != "" {
		t.Skip("tmux ativo no ambiente de teste")
	}
	t.Setenv("PM2_HOME_PATH", "")
	t.Setenv("PM2_JSON_PROCESSING", "")
	t.Setenv("pm2_env", "")
	t.Setenv("INVOCATION_ID", "")
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	// A ordem pm2 > systemd > tmux > screen > docker > direct é marcada por env.
	t.Setenv("PM2_HOME_PATH", "/x")
	t.Setenv("TMUX", "1")
	if got := detectRuntime(); got != "pm2" {
		t.Fatal(got)
	}
	t.Setenv("PM2_HOME_PATH", "")
	t.Setenv("INVOCATION_ID", "x")
	if got := detectRuntime(); got != "systemd" {
		t.Fatal(got)
	}
	t.Setenv("INVOCATION_ID", "")
	t.Setenv("TMUX", "x")
	if got := detectRuntime(); got != "tmux" {
		t.Fatal(got)
	}
	t.Setenv("TMUX", "")
	t.Setenv("STY", "1")
	if got := detectRuntime(); got != "screen" {
		t.Fatal(got)
	}
	t.Setenv("STY", "")
	if got := detectRuntime(); got != "direct" && got != "docker" {
		t.Fatal(got)
	}
}
