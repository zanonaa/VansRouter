package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBulkAgainstOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/bulk-http-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input  any
		Status int
		Body   any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	h := Providers{CreateBulk: func(_ context.Context, items []map[string]any) ([]map[string]any, error) { return items, nil }, GetNode: func(_ context.Context, id string) (map[string]any, error) {
		if strings.HasSuffix(id, "-known") {
			return map[string]any{"prefix": "n", "apiType": "chat", "baseUrl": "https://node.test/v1", "name": "Node"}, nil
		}
		return nil, nil
	}}
	for i, tt := range cases {
		payload, _ := json.Marshal(tt.Input)
		w := httptest.NewRecorder()
		h.Bulk(w, httptest.NewRequest("POST", "/", strings.NewReader(string(payload))))
		var got any
		if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
			t.Fatalf("case %d: Go=%d %v original=%d %v", i, w.Code, got, tt.Status, tt.Body)
		}
	}
}
