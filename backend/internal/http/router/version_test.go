package router

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestVersionPublicSkip(t *testing.T) {
	t.Setenv("VANSROUTER_SKIP_UPDATE_CHECK", "1")
	auth, _ := wiredAuth(t)
	r := NewWithAuth(nil, auth)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/version", nil))
	var value map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &value)
	if w.Code != 200 || value["currentVersion"] != "1.2.0" || value["latestVersion"] != nil || value["hasUpdate"] != false {
		t.Fatal(w.Code, value)
	}
	for _, method := range []string{"HEAD", "OPTIONS"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/version", nil))
		if w.Body.Len() != 0 || (w.Code != 200 && w.Code != 204) {
			t.Fatal(method, w.Code)
		}
	}
}
