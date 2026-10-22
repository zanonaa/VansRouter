package handlers

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestModelPingChecksActualResponse(t *testing.T) {
	for _, test := range []struct {
		kind, body string
		ok         bool
	}{{"embedding", `{"data":[{"embedding":[]}]}`, true}, {"embedding", `{}`, false}, {"image", `{"data":[{"url":"image"}]}`, true}, {"llm", `{"choices":[{"message":{"content":"hello"}}]}`, true}, {"llm", `{}`, false}} {
		p := &ModelPing{BaseURL: "http://127.0.0.1:20128", Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "127.0.0.1:20128" {
				t.Fatal("non-local target")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}}
		result := p.Ping(context.Background(), "provider/model", test.kind)
		if result["ok"] != test.ok {
			t.Fatalf("kind %s result %v", test.kind, result)
		}
	}
}
func TestSilentTestWAV(t *testing.T) {
	wav := silentTestWAV()
	if string(wav[:4]) != "RIFF" || binary.LittleEndian.Uint32(wav[40:]) != 8000 || len(wav) != 8044 {
		t.Fatal("invalid WAV")
	}
}
