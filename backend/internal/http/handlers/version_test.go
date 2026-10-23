package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionEnvironmentOverride(t *testing.T) {
	t.Setenv("VANSROUTER_VERSION", "2.0.777")
	if got := NewVersion().Current; got != "2.0.777" {
		t.Fatalf("versão sobrescrita pelo ambiente: %q", got)
	}
}

func TestVersionCacheAndSkip(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = w.Write([]byte(`{"version":"2.0.0"}`)) }))
	defer server.Close()
	h := &Version{Current: "1.2.0", Client: server.Client(), URL: server.URL}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.Get(w, httptest.NewRequest("GET", "/", nil))
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		if result["hasUpdate"] != true || result["latestVersion"] != "2.0.0" {
			t.Fatal(result)
		}
	}
	if calls != 1 {
		t.Fatalf("cache calls: %d", calls)
	}
	t.Setenv("VANSROUTER_SKIP_UPDATE_CHECK", "1")
	w := httptest.NewRecorder()
	h.Get(w, httptest.NewRequest("GET", "/", nil))
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result["latestVersion"] != nil || result["hasUpdate"] != false {
		t.Fatal(result)
	}
}
