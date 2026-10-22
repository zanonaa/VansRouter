package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type kiroImportTransport func(*http.Request) (*http.Response, error)

func (f kiroImportTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKiroImportRefreshAndPersist(t *testing.T) {
	var saved map[string]any
	h := &OAuthEngine{CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "new"
		return data, nil
	}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "oidc.eu-west-1.amazonaws.com" {
			t.Fatalf("unexpected host: %s", r.URL.Host)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["grantType"] != "refresh_token" || body["refreshToken"] != "refresh" {
			t.Fatal("unexpected refresh body")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"accessToken":"access","refreshToken":"rotated","expiresIn":3600}`))}, nil
	})}}
	w := httptest.NewRecorder()
	h.KiroImport(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"refreshToken":" refresh ","clientId":"id","clientSecret":"secret","region":"eu-west-1"}`)))
	if w.Code != 200 || saved["refreshToken"] != "rotated" {
		t.Fatalf("status %d", w.Code)
	}
	data := saved["providerSpecificData"].(map[string]any)
	if data["authMethod"] != "idc" || data["provider"] != "Enterprise" {
		t.Fatal("wrong IDC metadata")
	}
}

func TestKiroCLIImportValidation(t *testing.T) {
	raw := map[string]any{"access_token": "access", "refresh_token": "refresh", "client_id": "client", "token_endpoint": "https://login.microsoftonline.com/tenant/oauth2/token", "profile_arn": "arn:profile", "scopes": []any{"openid", " profile "}}
	data, err := normalizeKiroExternalAuth(raw)
	if err != nil {
		t.Fatal(err)
	}
	metadata := data["providerSpecificData"].(map[string]any)
	if metadata["scope"] != "openid profile" || metadata["region"] != "us-east-1" {
		t.Fatal("wrong metadata")
	}
	for _, endpoint := range []string{"http://login.microsoftonline.com/token", "https://example.com/token", "https://login.microsoftonline.com.evil.invalid/token"} {
		raw["token_endpoint"] = endpoint
		if _, err := normalizeKiroExternalAuth(raw); err == nil {
			t.Fatal("untrusted endpoint accepted")
		}
	}
}
