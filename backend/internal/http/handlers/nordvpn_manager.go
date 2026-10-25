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
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// NordManager é o singleton ligado no main.
type NordManager struct {
	ops     sync.Mutex
	db      *sql.DB
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
	if err := persistNordClient(ctx, client); err != nil {
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
	nordManager.ops.Lock()
	defer nordManager.ops.Unlock()
	if len(request.Countries) == 0 || request.Replicas < 1 || request.Replicas > 10 || len(request.Countries)*request.Replicas > 10 {
		return nil, fmt.Errorf("nordvpn: select 1–10 tunnels total")
	}
	if nordManager.db == nil {
		return nil, fmt.Errorf("nordvpn: persistent store unavailable")
	}
	created := []map[string]any{}
	for _, country := range dedupeStrings(request.Countries) {
		country = strings.ToLower(strings.TrimSpace(country))
		if len(country) != 2 {
			return created, fmt.Errorf("nordvpn: invalid country")
		}
		for i := 1; i <= request.Replicas; i++ {
			id := fmt.Sprintf("nord-%s-%d", country, i)
			record := nordTunnelRecord{ID: id, Country: country}
			err := nordManager.db.QueryRowContext(ctx, `SELECT poolId FROM nordvpnTunnels WHERE id=?`, id).Scan(&record.PoolID)
			if err == sql.ErrNoRows {
				pool, e := nordManager.pools.Create(ctx, map[string]any{"name": fmt.Sprintf("NordVPN %s %d", strings.ToUpper(country), i), "proxyUrl": "socks5://127.0.0.1:1", "type": "socks5", "strictProxy": true, "isActive": false})
				if e != nil {
					return created, e
				}
				record.PoolID = ttsString(pool["id"])
				if _, e = nordManager.db.ExecContext(ctx, `INSERT INTO nordvpnTunnels(id,poolId,country) VALUES(?,?,?)`, id, record.PoolID, country); e != nil {
					_, _ = nordManager.pools.Delete(ctx, record.PoolID)
					return created, e
				}
			} else if err != nil {
				return created, err
			}
			nordManager.mu.RLock()
			existing := nordManager.tunnels[id]
			nordManager.mu.RUnlock()
			if existing != nil {
				status := existing.Status()
				status["poolId"] = record.PoolID
				created = append(created, status)
				continue
			}
			status, e := nordConnectRecord(ctx, record, "")
			if e != nil {
				return created, e
			}
			created = append(created, status)
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
	nordManager.ops.Lock()
	defer nordManager.ops.Unlock()
	if nordManager.db == nil {
		return nil, fmt.Errorf("nordvpn: persistent store unavailable")
	}
	var record nordTunnelRecord
	record.ID = id
	if err := nordManager.db.QueryRowContext(ctx, `SELECT poolId,country FROM nordvpnTunnels WHERE id=?`, id).Scan(&record.PoolID, &record.Country); err != nil {
		return nil, err
	}
	nordManager.mu.RLock()
	old := nordManager.tunnels[id]
	nordManager.mu.RUnlock()
	host := ""
	if old != nil {
		host = old.hostname
	}
	return nordConnectRecord(ctx, record, host)
}

func NordDelete(id string) bool {
	nordManager.ops.Lock()
	defer nordManager.ops.Unlock()
	if nordManager.db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var poolID string
	if err := nordManager.db.QueryRowContext(ctx, `SELECT poolId FROM nordvpnTunnels WHERE id=?`, id).Scan(&poolID); err != nil {
		return false
	}
	if _, err := nordManager.pools.Update(ctx, poolID, map[string]any{"isActive": false, "strictProxy": true, "lastError": "NordVPN tunnel deleted"}); err != nil {
		return false
	}
	if _, err := nordManager.db.ExecContext(ctx, `DELETE FROM nordvpnTunnels WHERE id=?`, id); err != nil {
		return false
	}
	nordManager.mu.Lock()
	t := nordManager.tunnels[id]
	delete(nordManager.tunnels, id)
	nordManager.mu.Unlock()
	if t != nil {
		t.Close()
	}
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
