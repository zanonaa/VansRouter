package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsoleLogsBufferAndClear(t *testing.T) {
	b := NewConsoleBuffer()
	b.Append("one")
	b.Append("two")
	w := httptest.NewRecorder()
	b.Get(w, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(w.Body.String(), "one") {
		t.Fatal(w.Body.String())
	}
	b.Delete(w, httptest.NewRequest("DELETE", "/", nil))
	if len(b.Logs()) != 0 {
		t.Fatal("not cleared")
	}
}
