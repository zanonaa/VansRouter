package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderBatchEmptyAndInvalidMode(t *testing.T) {
	probe := &ConnectionProbe{ListConnections: func(context.Context, string, *bool) ([]map[string]any, error) { return nil, nil }}
	for _, test := range []struct {
		body   string
		status int
	}{{`{}`, 400}, {`{"mode":"invalid"}`, 400}, {`{"mode":"all"}`, 200}} {
		w := httptest.NewRecorder()
		probe.Batch(w, httptest.NewRequest("POST", "/", strings.NewReader(test.body)))
		if w.Code != test.status {
			t.Fatalf("status %d body %s", w.Code, w.Body.String())
		}
		if test.status == 200 && !strings.Contains(w.Body.String(), `"total":0`) {
			t.Fatal(w.Body.String())
		}
	}
}
