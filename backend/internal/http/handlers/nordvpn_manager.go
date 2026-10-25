package handlers

// NordManager mantém os túneis NordVPN vivos do processo: cria a partir de
// (país × réplicas), registra pools socks5 equivalentes, resolve health/IP
// e rotaciona servidor (re-handshake com outro hostname do país).
//
// O token de acesso fica em memória; persistimos apenas o necessário para
// reconstruir túneis depois de restart (endpoint + chave pública por túnel;
// a private key é recuperada de /services/credentials com o token salvo
// em settings — ver nordvpn_token em Settings).
import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// NordManager é o singleton ligado no main.
type NordManager struct {
	mu      sync.RWMutex
	client  *NordClient
	tunnels map[string]*NordTunnel
	pools   PoolStore
	token   string
}

var nordManager = &NordManager{tunnels: map[string]*NordTunnel{}}

// SetNordManager wire o manager com stores reais (chamado no main).
func SetNordManager(pools PoolStore, client *NordClient) {
	nordManager.mu.Lock()
	defer nordManager.mu.Unlock()
	nordManager.pools = pools
	if client != nil {
		nordManager.client = client
	}
}

// NordSetToken define o token de acesso (dashboard ou OAuth).
func NordSetToken(token string) error {
	nordManager.mu.Lock()
	defer nordManager.mu.Unlock()
	if strings.TrimSpace(token) == "" {
		nordManager.token = ""
		nordManager.client = nil
		return nil
	}
	nordManager.token = strings.TrimSpace(token)
	nordManager.client = &NordClient{}
	// valida o token buscando as credenciais
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	old := nordManager.client
	_ = old
	client := &NordClient{}
	if err := client.LoginToken(ctx, nordManager.token); err != nil {
		nordManager.client = nil
		return err
	}
	nordManager.client = client
	return nil
}

// NordCountries devolve os países disponíveis (código+nome) via API pública.
func NordCountries(ctx context.Context) ([]map[string]any, error) {
	client := nordManager.client
	if client == nil {
		client = &NordClient{}
	}
	payload, _, err := client.do(ctx, "GET", "/v1/servers/countries", nil, false)
	if err != nil {
		return nil, err
	}
	var countries []map[string]any
	if err := json.Unmarshal(payload, &countries); err != nil {
		return nil, err
	}
	sort.Slice(countries, func(i, j int) bool {
		return fmt.Sprint(countries[i]["name"]) < fmt.Sprint(countries[j]["name"])
	})
	return countries, nil
}

// NordCreateRequest é o payload de POST /api/nordvpn/tunnels.
type NordCreateRequest struct {
	Countries []string `json:"countries"`
	Replicas  int      `json:"replicas"`
}

// NordCreate cria um túnel por (país × réplica), registra o pool socks5 e
// valida o IP de saída. Réplicas do mesmo país usam servidores distintos.
func NordCreate(ctx context.Context, request NordCreateRequest) ([]map[string]any, error) {
	if len(request.Countries) == 0 {
		return nil, fmt.Errorf("nordvpn: informe ao menos um país")
	}
	if request.Replicas <= 0 {
		request.Replicas = 1
	}
	if request.Replicas > 10 {
		return nil, fmt.Errorf("nordvpn: máximo de 10 réplicas por país")
	}
	nordManager.mu.RLock()
	client := nordManager.client
	pools := nordManager.pools
	nordManager.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("nordvpn: token não configurado")
	}
	if pools == nil {
		return nil, fmt.Errorf("nordvpn: pool store não configurado")
	}
	credentials, err := client.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	created := []map[string]any{}
	for _, country := range request.Countries {
		country = strings.ToLower(strings.TrimSpace(country))
		if country == "" {
			continue
		}
		servers, err := client.ServersByCountry(ctx, country, request.Replicas*3)
		if err != nil {
			return created, err
		}
		online := []NordServer{}
		for _, server := range servers {
			if server.Hostname != "" {
				online = append(online, server)
			}
		}
		if len(online) < request.Replicas {
			return created, fmt.Errorf("nordvpn: %s tem apenas %d servidores online", country, len(online))
		}
		sort.SliceStable(online, func(i, j int) bool { return online[i].Load < online[j].Load })
		for index := 0; index < request.Replicas; index++ {
			server := online[index%len(online)]
			publicKey, err := client.PublicKey(ctx, server.Hostname)
			if err != nil {
				return created, err
			}
			spec := NordTunnelSpec{
				ID:         fmt.Sprintf("nord-%s-%d", country, index+1),
				Country:    country,
				Hostname:   server.Hostname,
				Endpoint:   server.Hostname + ":51820",
				PrivateKey: credentials.NordlynxPrivateKey,
				PublicKey:  publicKey,
			}
			tunnel, err := StartNordTunnel(ctx, spec)
			if err != nil {
				return created, fmt.Errorf("nordvpn %s: %w", spec.ID, err)
			}
			nordManager.mu.Lock()
			nordManager.tunnels[tunnel.id] = tunnel
			nordManager.mu.Unlock()
			// pool correspondente: nome estável, url socks5 do túnel
			pool, err := pools.Create(ctx, map[string]any{
				"name":     "NordVPN " + strings.ToUpper(country) + " " + fmt.Sprint(index+1),
				"proxyUrl": tunnel.Address(),
				"noProxy":  "",
				"isActive": true,
				"type":     "socks5",
			})
			if err != nil {
				tunnel.Close()
				nordManager.mu.Lock()
				delete(nordManager.tunnels, tunnel.id)
				nordManager.mu.Unlock()
				return created, err
			}
			status := tunnel.Status()
			status["poolId"] = pool["id"]
			created = append(created, status)
			// health assíncrono: valida handshake e captura o IP real
			go tunnel.CheckIP(context.Background())
		}
	}
	return created, nil
}

// NordList devolve o status de todos os túneis.
func NordList() []map[string]any {
	nordManager.mu.RLock()
	defer nordManager.mu.RUnlock()
	list := []map[string]any{}
	for _, tunnel := range nordManager.tunnels {
		list = append(list, tunnel.Status())
	}
	sort.Slice(list, func(i, j int) bool {
		return fmt.Sprint(list[i]["id"]) < fmt.Sprint(list[j]["id"])
	})
	return list
}

// NordRotate troca o servidor de um túnel (novo IP) mantendo o pool.
func NordRotate(ctx context.Context, id string) (map[string]any, error) {
	nordManager.mu.RLock()
	client := nordManager.client
	pools := nordManager.pools
	tunnel := nordManager.tunnels[id]
	nordManager.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("nordvpn: token não configurado")
	}
	if tunnel == nil {
		return nil, fmt.Errorf("nordvpn: túnel %s não encontrado", id)
	}
	credentials, err := client.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	servers, err := client.ServersByCountry(ctx, tunnel.country, 12)
	if err != nil {
		return nil, err
	}
	nordManager.mu.RLock()
	current := nordManager.tunnels[id].hostname
	nordManager.mu.RUnlock()
	var candidate *NordServer
	for i := range servers {
		if servers[i].Hostname != current {
			candidate = &servers[i]
			break
		}
	}
	if candidate == nil {
		return nil, fmt.Errorf("nordvpn: sem servidor alternativo em %s", tunnel.country)
	}
	publicKey, err := client.PublicKey(ctx, candidate.Hostname)
	if err != nil {
		return nil, err
	}
	spec := NordTunnelSpec{
		ID:         tunnel.id,
		Country:    tunnel.country,
		Hostname:   candidate.Hostname,
		Endpoint:   candidate.Hostname + ":51820",
		PrivateKey: credentials.NordlynxPrivateKey,
		PublicKey:  publicKey,
	}
	// preserva o endereço socks (pool aponta pra ele)
	oldAddress := tunnel.Address()
	tunnel.Close()
	replacement, err := StartNordTunnelAt(ctx, spec, oldAddress)
	if err != nil {
		nordManager.mu.Lock()
		delete(nordManager.tunnels, id)
		nordManager.mu.Unlock()
		return nil, err
	}
	nordManager.mu.Lock()
	nordManager.tunnels[id] = replacement
	nordManager.mu.Unlock()
	go replacement.CheckIP(context.Background())
	if pools != nil {
		// atualiza pool caso a porta tenha mudado (não deveria)
		_ = pools
	}
	return replacement.Status(), nil
}

// NordDelete encerra um túnel (o pool permanece, desativado pelo usuário se
// quiser — túnel e pool têm ciclos de vida independentes).
func NordDelete(id string) bool {
	nordManager.mu.Lock()
	defer nordManager.mu.Unlock()
	tunnel, ok := nordManager.tunnels[id]
	if !ok {
		return false
	}
	tunnel.Close()
	delete(nordManager.tunnels, id)
	return true
}

// NordHealth valida IPs de todos os túneis em paralelo.
func NordHealth(ctx context.Context) []map[string]any {
	nordManager.mu.RLock()
	tunnels := make([]*NordTunnel, 0, len(nordManager.tunnels))
	for _, tunnel := range nordManager.tunnels {
		tunnels = append(tunnels, tunnel)
	}
	nordManager.mu.RUnlock()
	var wg sync.WaitGroup
	for _, tunnel := range tunnels {
		wg.Add(1)
		go func(t *NordTunnel) {
			defer wg.Done()
			_, _ = t.CheckIP(ctx)
		}(tunnel)
	}
	wg.Wait()
	return NordList()
}
