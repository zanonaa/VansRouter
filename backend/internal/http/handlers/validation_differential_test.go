package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestValidationAgainstOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/validation-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input    map[string]any
		Statuses []int
		Data     any
		Text     string
		Status   int
		Body     any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for i, tt := range cases {
		func() {
			index := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if index >= len(tt.Statuses) {
					t.Error("unexpected upstream request")
					w.WriteHeader(500)
					return
				}
				status := tt.Statuses[index]
				index++
				w.WriteHeader(status)
				if tt.Data != nil {
					_ = json.NewEncoder(w).Encode(tt.Data)
				} else {
					_, _ = w.Write([]byte(tt.Text))
				}
			}))
			defer upstream.Close()
			if tt.Input["baseUrl"] == "http://example.test" {
				tt.Input["baseUrl"] = upstream.URL
			}
			payload, _ := json.Marshal(tt.Input)
			w := httptest.NewRecorder()
			(NodeValidation{Local: func(*http.Request) bool { return true }}).Validate(w, httptest.NewRequest("POST", "/", strings.NewReader(string(payload))))
			var got any
			if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
				t.Fatalf("case %d: Go=%d %v original=%d %v", i, w.Code, got, tt.Status, tt.Body)
			}
		}()
	}
}
