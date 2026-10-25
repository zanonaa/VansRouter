package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Cliente da API NordVPN contra um upstream de teste.

func nordTestClient(t *testing.T, handler http.HandlerFunc) (*NordClient, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &NordClient{BaseURL: server.URL}, server
}

func TestNordCredentialsCachesPrivateKey(t *testing.T) {
	client, _ := nordTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/services/credentials" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"username":             "svc",
			"password":             "pw",
			"nordlynx_private_key": "PKBASE64",
		})
	})
	client.mu.Lock()
	client.token = "tok"
	client.mu.Unlock()
	credentials, err := client.Credentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credentials.NordlynxPrivateKey != "PKBASE64" || credentials.Username != "svc" {
		t.Fatalf("credentials: %+v", credentials)
	}
	// segunda chamada serve do cache (sem bater no servidor)
	client.mu.Lock()
	client.token = "" // invalidaria requests novas
	client.mu.Unlock()
	again, err := client.Credentials(context.Background())
	if err != nil || again.NordlynxPrivateKey != "PKBASE64" {
		t.Fatalf("cache falhou: %v %+v", err, again)
	}
}

func TestNordServersByCountrySortsAndFilters(t *testing.T) {
	client, _ := nordTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filters[country_code]") != "us" {
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 2, "hostname": "us2.nordvpn.com", "load": 40},
			{"id": 1, "hostname": "us1.nordvpn.com", "load": 10},
		})
	})
	servers, err := client.ServersByCountry(context.Background(), "us", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 || servers[0].Hostname != "us1.nordvpn.com" {
		t.Fatalf("servers: %+v", servers)
	}
	if _, err := client.ServersByCountry(context.Background(), "usa", 1); err == nil {
		t.Fatal("país inválido deveria falhar")
	}
}

func TestNordPublicKeyFromWireguardMetadata(t *testing.T) {
	client, _ := nordTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filters[hostname]") != "us1.nordvpn.com" {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"technologies": []map[string]any{{
				"identifier": "wireguard_udp",
				"metadata": []map[string]any{
					{"name": "public_key", "value": "SERVERPUB"},
				},
			}},
		}})
	})
	key, err := client.PublicKey(context.Background(), "us1.nordvpn.com")
	if err != nil || key != "SERVERPUB" {
		t.Fatalf("public key: %q %v", key, err)
	}
}

func TestNordLoginTokenRejectsInvalid(t *testing.T) {
	client, _ := nordTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
	})
	if err := client.LoginToken(context.Background(), "bad"); err == nil {
		t.Fatal("token inválido deveria falhar")
	}
}

func TestNordLoginTokenAcceptsDashboardToken(t *testing.T) {
	client, _ := nordTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"username":             "svc",
			"password":             "pw",
			"nordlynx_private_key": "PK",
		})
	})
	if err := client.LoginToken(context.Background(), "good"); err != nil {
		t.Fatalf("token de dashboard válido recusado: %v", err)
	}
	credentials, err := client.Credentials(context.Background())
	if err != nil || credentials.NordlynxPrivateKey != "PK" {
		t.Fatalf("credentials pós-login: %v %+v", err, credentials)
	}
}

func TestNordPKCEShape(t *testing.T) {
	verifier, challenge, err := NordPKCE()
	if err != nil {
		t.Fatal(err)
	}
	// verifier: 48 hex (24 bytes), como o cliente Nord; challenge SHA256 hex.
	if len(verifier) != 48 || len(challenge) != 64 {
		t.Fatalf("shapes: %d %d", len(verifier), len(challenge))
	}
}

func TestNordLoginURIUsesOfficialContract(t *testing.T) {
	client, _ := nordTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["redirect_flow"] != "default" || body["preferred_flow"] != "login" || body["challenge"] != "challenge" {
			t.Errorf("invalid login contract: %v", body)
		}
		w.Write([]byte(`{"redirect_uri":"https://nordaccount.com/login","attempt":"attempt"}`))
	})
	uri, attempt, err := client.LoginURI(context.Background(), "challenge")
	if err != nil || uri == "" || attempt != "attempt" {
		t.Fatalf("login start failed: %v", err)
	}
}
