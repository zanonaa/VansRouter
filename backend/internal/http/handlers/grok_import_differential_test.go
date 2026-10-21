package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGrokImportOriginalHandler(t *testing.T) {
	raw, err := os.ReadFile("testdata/grok-import-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input  any
		Status int
		Body   any
		Saved  any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for i, tt := range cases {
		saved := []map[string]any{}
		h := OAuthEngine{CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
			saved = append(saved, data)
			copy := map[string]any{"id": fmt.Sprintf("saved-%d", len(saved))}
			for k, v := range data {
				copy[k] = v
			}
			return copy, nil
		}}
		payload, _ := json.Marshal(tt.Input)
		w := httptest.NewRecorder()
		h.GrokBulkImport(w, httptest.NewRequest("POST", "/", strings.NewReader(string(payload))))
		var got any
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
			t.Fatalf("case %d: Go=%d %v original=%d %v", i, w.Code, got, tt.Status, tt.Body)
		}
		encoded, _ := json.Marshal(saved)
		var normalized any
		_ = json.Unmarshal(encoded, &normalized)
		if !reflect.DeepEqual(normalized, tt.Saved) {
			t.Fatalf("case %d saved: Go=%v original=%v", i, normalized, tt.Saved)
		}
	}
}
