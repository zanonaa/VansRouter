package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslatorFilesRoundTripAndAllowlist(t *testing.T) {
	handler := &TranslatorFiles{DataDir: t.TempDir()}
	for _, file := range []string{"../escape", "unknown.json", ""} {
		w := httptest.NewRecorder()
		handler.Load(w, httptest.NewRequest("GET", "/?file="+file, nil))
		if w.Code != 400 {
			t.Fatalf("invalid file status %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.Load(w, httptest.NewRequest("GET", "/?file=1_req_client.json", nil))
	if w.Code != 404 {
		t.Fatalf("missing status %d", w.Code)
	}
	w = httptest.NewRecorder()
	handler.Save(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"file":"1_req_client.json","content":"{\"model\":\"example\"}"}`)))
	if w.Code != 200 {
		t.Fatalf("save %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handler.Load(w, httptest.NewRequest("GET", "/?file=1_req_client.json", nil))
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["content"] != `{"model":"example"}` {
		t.Fatal("roundtrip failed")
	}
	w = httptest.NewRecorder()
	handler.Save(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"file":"1_req_client.json"}`)))
	if w.Code != 400 {
		t.Fatal("missing content accepted")
	}
}
