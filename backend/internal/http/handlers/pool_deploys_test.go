package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// deployPoolStore memória mínima para os deploys.
type deployPoolStore struct {
	mu     sync.Mutex
	pools  []map[string]any
	create func(map[string]any) map[string]any
}

func (s *deployPoolStore) List(context.Context, *bool, string) ([]map[string]any, error) {
	return s.pools, nil
}

func (s *deployPoolStore) Get(context.Context, string) (map[string]any, error) { return nil, nil }

func (s *deployPoolStore) Create(_ context.Context, data map[string]any) (map[string]any, error) {
	created := map[string]any{"id": "novo-pool"}
	for key, value := range data {
		created[key] = value
	}
	s.mu.Lock()
	s.pools = append(s.pools, created)
	s.mu.Unlock()
	return created, nil
}

func (s *deployPoolStore) Update(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, nil
}

func (s *deployPoolStore) Delete(context.Context, string) (map[string]any, error) { return nil, nil }

func TestCloudflareDeployContract(t *testing.T) {
	var requests []string
	doer := &deployHTTP{Do: func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Method+" "+request.URL.String())
		switch {
		case request.Method == "PUT" && strings.HasSuffix(request.URL.Path, "/workers/scripts/meu-relay"):
			if request.Header.Get("Authorization") != "Bearer cf-token" {
				t.Fatalf("auth %q", request.Header.Get("Authorization"))
			}
			if err := request.ParseMultipartForm(1 << 22); err != nil {
				t.Fatal(err)
			}
			file, _, err := request.FormFile("index.js")
			if err != nil {
				t.Fatal(err)
			}
			workerCode, _ := io.ReadAll(file)
			if !strings.Contains(string(workerCode), "export default {") || !strings.Contains(string(workerCode), "function assertTrustedTarget(rawUrl)") {
				t.Fatalf("worker inválido: %.200s", string(workerCode))
			}
			if strings.Contains(string(workerCode), "RELAY_TARGET_GUARD_SOURCE") {
				t.Fatal("placeholder vazado no worker")
			}
			return deployResponse(200, `{}`), nil
		case request.Method == "POST" && strings.HasSuffix(request.URL.Path, "/subdomain"):
			return deployResponse(200, `{}`), nil
		case request.Method == "GET" && strings.HasSuffix(request.URL.Path, "/workers/subdomain"):
			return deployResponse(200, `{"result":{"subdomain":"equipe"}}`), nil
		}
		t.Fatalf("request inesperada %s %s", request.Method, request.URL)
		return nil, nil
	}}
	store := &deployPoolStore{}
	deploys := &RelayDeploys{Store: store, HTTP: doer}
	w := httptest.NewRecorder()
	deploys.CloudflareDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/cloudflare-deploy", strings.NewReader(`{"accountId":"abc","apiToken":"cf-token","projectName":"meu-relay"}`)))
	if w.Code != 201 {
		t.Fatalf("deploy %d %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["deployUrl"] != "https://meu-relay.equipe.workers.dev" {
		t.Fatalf("deployUrl %v", payload["deployUrl"])
	}
	if len(requests) != 3 {
		t.Fatalf("requests %d", len(requests))
	}
	// Falha de upload: status propagado com a mensagem do Cloudflare.
	doer.Do = func(*http.Request) (*http.Response, error) {
		return deployResponse(403, `{"errors":[{"message":"not authorized"}]}`), nil
	}
	w = httptest.NewRecorder()
	deploys.CloudflareDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/cloudflare-deploy", strings.NewReader(`{"accountId":"abc","apiToken":"cf-token"}`)))
	if w.Code != 403 {
		t.Fatalf("upload erro %d %s", w.Code, w.Body.String())
	}
	var failed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "not authorized" {
		t.Fatalf("error %v", failed)
	}
	// Credenciais ausentes → 400 literal.
	w = httptest.NewRecorder()
	deploys.CloudflareDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/cloudflare-deploy", strings.NewReader(`{}`)))
	if w.Code != 400 {
		t.Fatalf("creds %d", w.Code)
	}
}

func TestDenoDeployContract(t *testing.T) {
	var appID string
	doer := &deployHTTP{Do: func(request *http.Request) (*http.Response, error) {
		path := request.URL.String()
		switch {
		case request.Method == "POST" && path == "https://api.deno.com/v2/apps":
			var body struct {
				Slug string `json:"slug"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body.Slug != "meu-relay" {
				t.Fatalf("slug %q", body.Slug)
			}
			appID = "app-1"
			return deployResponse(200, `{"id":"app-1"}`), nil
		case request.Method == "POST" && strings.HasSuffix(path, "/apps/app-1/deploy"):
			var body struct {
				Assets map[string]struct {
					Content string `json:"content"`
				} `json:"assets"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if !strings.Contains(body.Assets["main.ts"].Content, "Deno.serve(async (request) => {") || !strings.Contains(body.Assets["main.ts"].Content, "assertTrustedTarget") {
				t.Fatal("código deno inválido")
			}
			return deployResponse(200, `{"id":"rev-1","status":"succeeded"}`), nil
		}
		t.Fatalf("request inesperada %s %s", request.Method, path)
		return nil, nil
	}}
	deploys := &RelayDeploys{Store: &deployPoolStore{}, HTTP: doer}
	w := httptest.NewRecorder()
	deploys.DenoDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/deno-deploy", strings.NewReader(`{"denoToken":"tk","orgDomain":"equipe.deno.net","projectName":"meu-relay"}`)))
	if w.Code != 201 {
		t.Fatalf("deno %d %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload["deployUrl"] != "https://meu-relay.equipe.deno.net" {
		t.Fatalf("deployUrl %v", payload["deployUrl"])
	}
	if fmt.Sprint(appID) != "app-1" {
		t.Fatal("app não criado")
	}
	// 409 na criação.
	doer.Do = func(*http.Request) (*http.Response, error) {
		return deployResponse(409, `{"message":"conflict"}`), nil
	}
	w = httptest.NewRecorder()
	deploys.DenoDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/deno-deploy", strings.NewReader(`{"denoToken":"tk","orgDomain":"equipe.deno.net","projectName":"meu-relay"}`)))
	if w.Code != 409 {
		t.Fatalf("409 %d", w.Code)
	}
	var conflict map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &conflict)
	if conflict["error"] != `App "meu-relay" already exists. Choose a different name.` {
		t.Fatalf("conflict %v", conflict)
	}
	// Org/token vazios → 400 na ordem JS.
	w = httptest.NewRecorder()
	deploys.DenoDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/deno-deploy", strings.NewReader(`{"denoToken":"tk"}`)))
	if w.Code != 400 {
		t.Fatalf("org %d", w.Code)
	}
	w = httptest.NewRecorder()
	deploys.DenoDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/deno-deploy", strings.NewReader(`{"orgDomain":"x.deno.net"}`)))
	if w.Code != 400 {
		t.Fatalf("token %d", w.Code)
	}
}

func TestVercelDeployContract(t *testing.T) {
	var polls int
	doer := &deployHTTP{Do: func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == "POST" && request.URL.Path == "/v13/deployments":
			var body struct {
				Name  string `json:"name"`
				Files []struct {
					File string `json:"file"`
					Data string `json:"data"`
				} `json:"files"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body.Name != "meu-relay" || len(body.Files) != 3 {
				t.Fatalf("body %+v", body)
			}
			return deployResponse(200, `{"id":"d1","projectId":"p1","url":"meu-relay.vercel.app"}`), nil
		case request.Method == "PATCH" && strings.HasPrefix(request.URL.Path, "/v9/projects/"):
			return deployResponse(200, `{}`), nil
		case request.Method == "GET" && strings.HasPrefix(request.URL.Path, "/v13/deployments/"):
			polls++
			return deployResponse(200, `{"readyState":"READY","url":"meu-relay.vercel.app"}`), nil
		}
		t.Fatalf("request inesperada %s %s", request.Method, request.URL)
		return nil, nil
	}}
	deploys := &RelayDeploys{Store: &deployPoolStore{}, HTTP: doer}
	w := httptest.NewRecorder()
	deploys.VercelDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/vercel-deploy", strings.NewReader(`{"vercelToken":"tk","projectName":"meu-relay"}`)))
	if w.Code != 201 {
		t.Fatalf("vercel %d %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload["deployUrl"] != "https://meu-relay.vercel.app" {
		t.Fatalf("deployUrl %v", payload["deployUrl"])
	}
	if polls < 1 {
		t.Fatal("poll não rodou")
	}
	// Token ausente → 400 literal.
	w = httptest.NewRecorder()
	deploys.VercelDeploy(w, httptest.NewRequest("POST", "/api/proxy-pools/vercel-deploy", strings.NewReader(`{}`)))
	if w.Code != 400 {
		t.Fatalf("token %d", w.Code)
	}
}
