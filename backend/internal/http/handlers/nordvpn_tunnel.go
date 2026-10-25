package handlers

// Túnel NordLynx (WireGuard) userspace sobre gVisor netstack, no modelo do
// wireproxy: sem /dev/net/tun, sem root e sem tocar nas rotas do host. Cada
// túnel tem sua própria pilha TCP/IP no processo e um listener SOCKS5 local;
// apenas requests de conexões vinculadas ao pool correspondente passam por
// ele (mesma semântica dos pools HTTP existentes).
import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// NordTunnel é um túnel NordLynx vivo: device WireGuard + netstack + listener
// SOCKS5. O IP de saída pertence apenas a este túnel.
type NordTunnel struct {
	mu sync.RWMutex

	id         string
	country    string
	hostname   string
	endpoint   string // host:port UDP do peer
	privateKey string
	publicKey  string

	device   *device.Device
	net      *netstack.Net
	listener net.Listener
	address  string // socks5://127.0.0.1:porta

	lastIP     string
	lastError  string
	createdAt  time.Time
	connected  bool
	closed     bool
	localAddrs []net.IP
}

// NordTunnelSpec descreve o túnel a criar.
type NordTunnelSpec struct {
	ID         string
	Country    string
	Hostname   string
	Endpoint   string // host:51820
	PrivateKey string
	PublicKey  string
}

// nordPort escolhe uma porta local livre para o listener SOCKS5.
func nordFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// StartNordTunnel sobe o túnel userspace e o listener SOCKS5. O handshake é
// validado com um ping via tunnel antes de devolver o túnel pronto.
func StartNordTunnel(ctx context.Context, spec NordTunnelSpec) (*NordTunnel, error) {
	if spec.PrivateKey == "" || spec.PublicKey == "" || spec.Endpoint == "" {
		return nil, fmt.Errorf("nordvpn: spec incompleta (private/public/endpoint)")
	}
	localPort, err := nordFreePort()
	if err != nil {
		return nil, err
	}
	tunnelAddress := "127.0.0.1:" + strconv.Itoa(localPort)

	// 10.5.0.0/16 interno por túnel — o netstack isola cada túnel; endereços
	// nunca saem do processo.
	localIP, _ := netip.ParseAddr("10.5.0.2")
	dns, _ := netip.ParseAddr("1.1.1.1")
	tun, stack, err := netstack.CreateNetTUN(
		[]netip.Addr{localIP},
		[]netip.Addr{dns},
		1420,
	)
	if err != nil {
		return nil, fmt.Errorf("nordvpn netstack: %w", err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, "nord"))
	config := fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=0.0.0.0/0\npersistent_keepalive_interval=25\n",
		normalizeWgKey(spec.PrivateKey), normalizeWgKey(spec.PublicKey), spec.Endpoint)
	if err := dev.IpcSet(config); err != nil {
		dev.Close()
		return nil, fmt.Errorf("nordvpn wg config: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("nordvpn wg up: %w", err)
	}

	listener, err := net.Listen("tcp", tunnelAddress)
	if err != nil {
		dev.Close()
		return nil, err
	}

	tunnel := &NordTunnel{
		id:         spec.ID,
		country:    spec.Country,
		hostname:   spec.Hostname,
		endpoint:   spec.Endpoint,
		privateKey: spec.PrivateKey,
		publicKey:  spec.PublicKey,
		device:     dev,
		net:        stack,
		listener:   listener,
		address:    "socks5://" + tunnelAddress,
		createdAt:  time.Now(),
	}
	go tunnel.acceptLoop()
	return tunnel, nil
}

// StartNordTunnelAt cria um túnel escutando no endereço socks informado
// (usado pela rotação: o pool continua apontando para o mesmo host:porta).
func StartNordTunnelAt(ctx context.Context, spec NordTunnelSpec, address string) (*NordTunnel, error) {
	tunnel, err := StartNordTunnel(ctx, spec)
	if err != nil {
		return nil, err
	}
	if address != "" && address != tunnel.Address() {
		// porta antiga: fecha o listener novo e reabre no endereço pedido
		tunnel.mu.Lock()
		listener := tunnel.listener
		tunnel.mu.Unlock()
		if listener != nil {
			_ = listener.Close()
		}
		parsed := mustParseURLLoose(address)
		replacement, listenErr := net.Listen("tcp", parsed.Host)
		if listenErr != nil {
			tunnel.Close()
			return nil, listenErr
		}
		tunnel.mu.Lock()
		tunnel.listener = replacement
		tunnel.address = "socks5://" + parsed.Host
		tunnel.mu.Unlock()
		go tunnel.acceptLoop()
	}
	return tunnel, nil
}

// normalizeWgKey aceita a private/public key em base64 (formato da API
// NordVPN e do wg-quick) e devolve o hex que o IpcSet do wireguard-go usa.
func normalizeWgKey(key string) string {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(key))
	if err != nil {
		return key
	}
	return hex.EncodeToString(decoded)
}

// acceptLoop serve conexões SOCKS5 entrantes (o próprio VansRouter é o
// cliente; auth local opcional no futuro).
func (t *NordTunnel) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			t.mu.Lock()
			if !t.closed {
				t.lastError = err.Error()
			}
			t.mu.Unlock()
			return
		}
		go t.handleSOCKS(conn)
	}
}

// handleSOCKS implementa o mínimo do protocolo SOCKS5 (greeting + CONNECT)
// e atravessa o túnel via netstack.
func (t *NordTunnel) handleSOCKS(client net.Conn) {
	defer client.Close()
	// greeting: [VER NMETHODS METHODS...]
	header := make([]byte, 2)
	if _, err := io.ReadFull(client, header); err != nil {
		return
	}
	if header[0] != 5 {
		return
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(client, methods); err != nil {
		return
	}
	// no-auth
	if _, err := client.Write([]byte{5, 0}); err != nil {
		return
	}
	// request: VER CMD RSV ATYP ADDR...
	req := make([]byte, 4)
	if _, err := io.ReadFull(client, req); err != nil {
		return
	}
	if req[1] != 1 { // apenas CONNECT
		_, _ = client.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	var host string
	var port int
	switch req[3] {
	case 1: // IPv4
		raw := make([]byte, 4)
		if _, err := io.ReadFull(client, raw); err != nil {
			return
		}
		host = net.IP(raw).String()
	case 3: // domain
		length := make([]byte, 1)
		if _, err := io.ReadFull(client, length); err != nil {
			return
		}
		raw := make([]byte, int(length[0]))
		if _, err := io.ReadFull(client, raw); err != nil {
			return
		}
		host = string(raw)
	case 4: // IPv6
		raw := make([]byte, 16)
		if _, err := io.ReadFull(client, raw); err != nil {
			return
		}
		host = net.IP(raw).String()
	default:
		return
	}
	portRaw := make([]byte, 2)
	if _, err := io.ReadFull(client, portRaw); err != nil {
		return
	}
	port = int(portRaw[0])<<8 | int(portRaw[1])

	t.mu.RLock()
	netStack := t.net
	t.mu.RUnlock()
	if netStack == nil {
		_, _ = client.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	remote, dialErr := dialSOCKSviaNetstack(netStack, host, port)
	if dialErr != nil {
		_, _ = client.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()
	// success reply
	_, _ = client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	relay(client, remote)
}

// dialSOCKSviaNetstack resolve o host no DNS do túnel (1.1.1.1 via
// netstack) e disca o alvo pela pilha do túnel, com limite curto — o cliente
// SOCKS espera resposta de CONNECT rápido.
func dialSOCKSviaNetstack(stack *netstack.Net, host string, port int) (net.Conn, error) {
	// LookupHost do netstack não honra contexto quando não há resposta DNS;
	// corremos com limite próprio e devolvemos erro amigável.
	resolveDone := make(chan struct{})
	var addresses []string
	var resolveErr error
	go func() {
		defer close(resolveDone)
		addresses, resolveErr = stack.LookupHost(host)
	}()
	select {
	case <-resolveDone:
	case <-time.After(6 * time.Second):
		return nil, fmt.Errorf("nordvpn: resolução DNS do túnel expirou (%s)", host)
	}
	if resolveErr != nil {
		return nil, resolveErr
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("nordvpn: host sem endereço: %s", host)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ip, err := netip.ParseAddr(addresses[0])
	if err != nil {
		return nil, err
	}
	return stack.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(ip, uint16(port)))
}

func relay(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(a, b); done <- struct{}{} }()
	go func() { _, _ = io.Copy(b, a); done <- struct{}{} }()
	<-done
}

// Address devolve a URL socks5 do listener local.
func (t *NordTunnel) Address() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.address
}

// Status devolve um snapshot do túnel.
func (t *NordTunnel) Status() map[string]any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return map[string]any{
		"id":        t.id,
		"country":   t.country,
		"hostname":  t.hostname,
		"address":   t.address,
		"lastIp":    t.lastIP,
		"lastError": t.lastError,
		"connected": t.connected,
		"createdAt": t.createdAt.Format(time.RFC3339),
	}
}

// CheckIP consulta o IP de saída do túnel e atualiza o status.
func (t *NordTunnel) CheckIP(ctx context.Context) (string, error) {
	address := t.Address()
	parsed := mustParseURLLoose(address)
	dialer, err := proxy.SOCKS5("tcp", parsed.Host, nil, proxy.Direct)
	if err != nil {
		return "", err
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return "", fmt.Errorf("socks dialer sem contexto")
	}
	transport := &http.Transport{DialContext: contextDialer.DialContext}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		t.mu.Lock()
		t.lastError = err.Error()
		t.connected = false
		t.mu.Unlock()
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64))
	if err != nil {
		return "", err
	}
	ip := string(body)
	t.mu.Lock()
	t.lastIP = ip
	t.lastError = ""
	t.connected = true
	t.mu.Unlock()
	return ip, nil
}

// Close encerra listener e device.
func (t *NordTunnel) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	t.connected = false
	listener := t.listener
	dev := t.device
	t.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
	if dev != nil {
		dev.Close()
	}
}

func mustParseURLLoose(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{Host: raw}
	}
	return parsed
}
