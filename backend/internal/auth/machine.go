package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const cliAuthSalt = "9r-cli-auth"

// MachineID reproduz shared/utils/machineId.js para o token CLI (x-9r-cli-token).
// O token é derivado do mesmo arquivo DATA_DIR/machine-id e DATA_DIR/auth/cli-secret,
// garantindo que o servidor e o CLI vejam o mesmo valor.
type MachineID struct {
	dataDir string
	mu      sync.Mutex
	raw     string
	secret  string
}

func NewMachineID(dataDir string) *MachineID {
	return &MachineID{dataDir: dataDir}
}

func (m *MachineID) rawID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.raw != "" {
		return m.raw
	}
	// 1. arquivo persistido
	if data, err := os.ReadFile(filepath.Join(m.dataDir, "machine-id")); err == nil {
		if value := strings.TrimSpace(string(data)); value != "" {
			m.raw = value
			return m.raw
		}
	}
	// 2. identificadores do host
	for _, candidate := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(candidate); err == nil {
			if value := strings.TrimSpace(string(data)); value != "" {
				m.raw = value
				m.persistRaw()
				return m.raw
			}
		}
	}
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		m.raw = strings.TrimSpace(host)
		m.persistRaw()
		return m.raw
	}
	// 3. fallback aleatório
	m.raw = newUUID()
	m.persistRaw()
	return m.raw
}

func (m *MachineID) persistRaw() {
	_ = os.MkdirAll(m.dataDir, 0700)
	_ = os.WriteFile(filepath.Join(m.dataDir, "machine-id"), []byte(m.raw), 0600)
}

func (m *MachineID) cliSecret() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secret != "" {
		return m.secret
	}
	file := filepath.Join(m.dataDir, "auth", "cli-secret")
	if data, err := os.ReadFile(file); err == nil {
		if value := strings.TrimSpace(string(data)); value != "" {
			m.secret = value
			return m.secret
		}
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		// fallback pouco provável; mantém determinístico apenas em erro de entropia
		m.secret = "0000000000000000000000000000000000000000000000000000000000000000"
		return m.secret
	}
	m.secret = hex.EncodeToString(random)
	_ = os.MkdirAll(filepath.Dir(file), 0700)
	_ = os.WriteFile(file, []byte(m.secret), 0600)
	return m.secret
}

// CLIToken reproduz getConsistentMachineId('9r-cli-auth').
func (m *MachineID) CLIToken() string {
	raw := m.rawID()
	secret := m.cliSecret()
	sum := sha256.Sum256([]byte(raw + cliAuthSalt + secret))
	return hex.EncodeToString(sum[:])[:16]
}

func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
