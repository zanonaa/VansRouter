package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type voiceTransport func(*http.Request) (*http.Response, error)

func (f voiceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestVoiceFetchContracts(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: voiceTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host == "speech.platform.bing.com" {
			if r.URL.Query().Get("trustedclienttoken") != "6A5AA1D4EAFF4E9FB37E23D68491D6F4" {
				t.Error("missing trusted client token")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[]`))}, nil
		}
		if r.Header.Get("xi-api-key") != "dummy" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("eleven headers")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"voices":[]}`))}, nil
	})}
	if _, err := fetchElevenVoices(context.Background(), client, ""); err == nil || calls != 0 {
		t.Fatal("missing key called network")
	}
	if _, err := fetchElevenVoices(context.Background(), client, "dummy"); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchEdgeVoices(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	voices, err := FetchGeminiVoicesPublic(context.Background(), "")
	if err != nil || len(voices) != 30 || voices[0]["voice_id"] != "Zephyr" {
		t.Fatal(len(voices), err)
	}
	if voiceLanguage("pt") != "Portuguese" || voiceRegion("US") != "United States" {
		t.Fatal("display names")
	}
}
