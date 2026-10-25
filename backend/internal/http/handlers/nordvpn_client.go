package handlers

// Porta do contrato OAuth + credentials da API do NordVPN, extraído do
// cliente oficial nordvpn-linux (core/authentication.go, core/urls.go):
//
//	POST /v1/users/oauth/login  (PKCE)  → redirect_uri + attempt
//	POST /v1/users/oauth/token           → token/renew_token
//	GET  /v1/users/services/credentials  → username/password/nordlynx_private_key
//	GET  /v1/users/tokens                → login por token de dashboard
//	GET  /v1/servers?filters[country_id] → hostname + wireguard public_key
//
// O login por token do dashboard (my.nordaccount.com → Manual setup) aceita
// contas criadas via Google: o token é gerado no navegador, onde o login
// social já aconteceu — o VansRouter nunca vê a senha.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const nordAPIBase = "https://api.nordvpn.com"

// NordClient conversa com a API pública do NordVPN. As chamadas autenticadas
// exigem o Bearer token obtido por OAuth (fluxo PKCE) ou pelo token de
// dashboard colado pelo usuário.
type NordClient struct {
	BaseURL string
	Client  *http.Client

	mu          sync.Mutex
	renewMu     sync.Mutex
	token       string
	renewToken  string
	expiresAt   time.Time
	credentials *NordServiceCredentials
}

// NordServiceCredentials é a resposta de /v1/users/services/credentials.
type NordServiceCredentials struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	NordlynxPrivateKey string `json:"nordlynx_private_key"`
}

type nordLoginResponse struct {
	UserID     int64  `json:"user_id"`
	Token      string `json:"token"`
	RenewToken string `json:"renew_token"`
	ExpiresAt  string `json:"expires_at"`
}

type nordOAuthLoginResponse struct {
	URI     string `json:"redirect_uri"`
	Attempt string `json:"attempt"`
}

// NordServer é o servidor WireGuard candidato (menor load primeiro).
type NordServer struct {
	ID       int64  `json:"id"`
	Hostname string `json:"hostname"`
	Load     int    `json:"load"`
}

func (c *NordClient) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return nordAPIBase
}

func (c *NordClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *NordClient) do(ctx context.Context, method, path string, body any, authenticated bool) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, reader)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	if path == "/v1/users/tokens/renew" {
		request.Header.Set("Idempotency-Key", uuid.NewString())
	}
	if authenticated {
		if err := c.RenewIfDue(ctx); err != nil {
			return nil, 0, err
		}
		c.mu.Lock()
		token := c.token
		c.mu.Unlock()
		if token == "" {
			return nil, 0, fmt.Errorf("nordvpn: not authenticated")
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(payload))
		if len(message) > 240 {
			message = message[:240]
		}
		return payload, response.StatusCode, fmt.Errorf("nordvpn %s %s: %d %s", method, path, response.StatusCode, message)
	}
	return payload, response.StatusCode, nil
}

// LoginToken autentica com o token de longa duração gerado no dashboard
// (funciona para contas Google: o token é criado no navegador logado).
// O token de dashboard é usado direto como Bearer; a validação é buscar as
// credenciais de serviço com ele.
func (c *NordClient) LoginToken(ctx context.Context, token string) error {
	c.mu.Lock()
	c.token = strings.TrimSpace(token)
	c.credentials = nil
	c.mu.Unlock()
	if _, err := c.Credentials(ctx); err != nil {
		c.mu.Lock()
		c.token = ""
		c.mu.Unlock()
		return fmt.Errorf("nordvpn: token inválido ou expirado (%s)", err.Error())
	}
	return nil
}

// LoginExchange completa o fluxo OAuth PKCE a partir do exchange_token
// recebido no navegador (após abrir o redirect_uri da chamada a LoginURI).
func (c *NordClient) LoginExchange(ctx context.Context, attempt, verifier, exchangeToken string) error {
	payload, _, err := c.do(ctx, http.MethodPost, "/v1/users/oauth/token", map[string]any{
		"attempt":        attempt,
		"verifier":       verifier,
		"exchange_token": exchangeToken,
	}, false)
	if err != nil {
		return err
	}
	var login nordLoginResponse
	if err := json.Unmarshal(payload, &login); err != nil {
		return err
	}
	if login.Token == "" {
		return fmt.Errorf("nordvpn: oauth sem token")
	}
	c.mu.Lock()
	c.token = login.Token
	c.renewToken = login.RenewToken
	c.expiresAt, _ = time.Parse(time.RFC3339, login.ExpiresAt)
	c.mu.Unlock()
	return nil
}

// NordPKCE gera o par verifier/challenge (RFC7636), como o cliente oficial.
func NordPKCE() (verifier, challenge string, err error) {
	raw := make([]byte, 24)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier = hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(verifier))
	challenge = hex.EncodeToString(digest[:])
	return verifier, challenge, nil
}

// LoginURI inicia o fluxo OAuth devolvendo a URI para abrir no navegador.
func (c *NordClient) LoginURI(ctx context.Context, challenge string) (uri, attempt string, err error) {
	payload, _, err := c.do(ctx, http.MethodPost, "/v1/users/oauth/login", map[string]any{
		"challenge":      challenge,
		"preferred_flow": "login",
		"redirect_flow":  "default",
	}, false)
	if err != nil {
		return "", "", err
	}
	var login nordOAuthLoginResponse
	if err := json.Unmarshal(payload, &login); err != nil {
		return "", "", err
	}
	return login.URI, login.Attempt, nil
}

// Credentials devolve (com cache) as credenciais de serviço, incluindo a
// private key NordLynx usada pelo túnel WireGuard.
func (c *NordClient) Credentials(ctx context.Context) (*NordServiceCredentials, error) {
	c.mu.Lock()
	cached := c.credentials
	token := c.token
	c.mu.Unlock()
	if cached != nil && cached.NordlynxPrivateKey != "" {
		return cached, nil
	}
	if token == "" {
		return nil, fmt.Errorf("nordvpn: not authenticated")
	}
	payload, _, err := c.do(ctx, http.MethodGet, "/v1/users/services/credentials", nil, true)
	if err != nil {
		return nil, err
	}
	var credentials NordServiceCredentials
	if err := json.Unmarshal(payload, &credentials); err != nil {
		return nil, err
	}
	if credentials.NordlynxPrivateKey == "" {
		return nil, fmt.Errorf("nordvpn: resposta sem nordlynx_private_key")
	}
	c.mu.Lock()
	c.credentials = &credentials
	c.mu.Unlock()
	return &credentials, nil
}

// ServersByCountry devolve os servidores online de um país ordenados por
// menor load. countryCode é o ISO de duas letras (ex.: "us", "br").
func (c *NordClient) ServersByCountry(ctx context.Context, countryCode string, limit int) ([]NordServer, error) {
	country := strings.ToLower(strings.TrimSpace(countryCode))
	if len(country) != 2 {
		return nil, fmt.Errorf("nordvpn: país deve ser ISO 2 letras")
	}
	if limit <= 0 {
		limit = 10
	}
	path := fmt.Sprintf("/v1/servers?filters[country_code]=%s&limit=%d", url.QueryEscape(country), limit)
	payload, _, err := c.do(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}
	var servers []NordServer
	if err := json.Unmarshal(payload, &servers); err != nil {
		return nil, err
	}
	sort.SliceStable(servers, func(i, j int) bool { return servers[i].Load < servers[j].Load })
	return servers, nil
}

// PublicKey devolve a public key WireGuard do servidor (metadata
// wireguard_udp), necessária para montar o peer do túnel.
func (c *NordClient) PublicKey(ctx context.Context, hostname string) (string, error) {
	path := "/v1/servers?filters[hostname]=" + url.QueryEscape(hostname)
	payload, _, err := c.do(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return "", err
	}
	var servers []struct {
		Technologies []struct {
			Identifier string `json:"identifier"`
			Metadata   []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"metadata"`
		} `json:"technologies"`
	}
	if err := json.Unmarshal(payload, &servers); err != nil {
		return "", err
	}
	for _, server := range servers {
		for _, technology := range server.Technologies {
			if technology.Identifier != "wireguard_udp" {
				continue
			}
			for _, meta := range technology.Metadata {
				if meta.Name == "public_key" {
					return meta.Value, nil
				}
			}
		}
	}
	return "", fmt.Errorf("nordvpn: sem public_key wireguard para %s", hostname)
}

// Manual dashboard tokens do not have a renew token: never invent a refresh
// flow for them. OAuth refresh is serialized and rotated secrets are saved
// before becoming visible to other requests.
func (c *NordClient) RenewIfDue(ctx context.Context) error {
	c.renewMu.Lock()
	defer c.renewMu.Unlock()
	c.mu.Lock()
	renew, expires := c.renewToken, c.expiresAt
	c.mu.Unlock()
	if renew == "" || expires.IsZero() || time.Until(expires) > 5*time.Minute {
		return nil
	}
	payload, _, err := c.do(ctx, http.MethodPost, "/v1/users/tokens/renew", map[string]string{"renewToken": renew}, false)
	if err != nil {
		return fmt.Errorf("nordvpn: session renewal unavailable; reauthenticate if revoked")
	}
	var login nordLoginResponse
	if json.Unmarshal(payload, &login) != nil || login.Token == "" || login.RenewToken == "" {
		return fmt.Errorf("nordvpn: invalid renewal response")
	}
	expiry, err := time.Parse(time.RFC3339, login.ExpiresAt)
	if err != nil {
		return fmt.Errorf("nordvpn: invalid renewal expiry")
	}
	nordAuthPersistence.RLock()
	store := nordAuthPersistence.store
	nordAuthPersistence.RUnlock()
	if store != nil {
		if err = store.Save(ctx, nordAuthState{login.Token, login.RenewToken, expiry}); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.token = login.Token
	c.renewToken = login.RenewToken
	c.expiresAt = expiry
	c.credentials = nil
	c.mu.Unlock()
	return nil
}
