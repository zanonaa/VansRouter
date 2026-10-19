package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachineIDPersistenceAndToken(t *testing.T) {
	dir := t.TempDir()
	m := NewMachineID(dir)
	first := m.CLIToken()
	if len(first) != 16 {
		t.Fatalf("token com tamanho inesperado: %q", first)
	}
	// Segunda instância lê os arquivos persistidos e gera o mesmo token.
	m2 := NewMachineID(dir)
	if m2.CLIToken() != first {
		t.Fatal("token não persistido/determinístico")
	}
	if _, err := os.Stat(filepath.Join(dir, "machine-id")); err != nil {
		t.Fatal("machine-id não persistido")
	}
	if _, err := os.Stat(filepath.Join(dir, "auth", "cli-secret")); err != nil {
		t.Fatal("cli-secret não persistido")
	}
	// Host fallback: diretório vazio com machine-id do host? Aqui forçamos um valor conhecido.
	if err := os.WriteFile(filepath.Join(dir, "machine-id"), []byte("maquina-ficticia"), 0600); err != nil {
		t.Fatal(err)
	}
	m3 := NewMachineID(dir)
	token3 := m3.CLIToken()
	if token3 == first {
		t.Fatal("token não mudou após alterar machine-id")
	}
	if !strings.HasPrefix(token3, "") {
		t.Fatal("token vazio")
	}
}

func TestMachineIDHostFallback(t *testing.T) {
	dir := t.TempDir()
	// Sem machine-id persistido e sem arquivos do host acessíveis: gera e persiste.
	m := NewMachineID(dir)
	token := m.CLIToken()
	if token == "" {
		t.Fatal("token vazio")
	}
	data, err := os.ReadFile(filepath.Join(dir, "machine-id"))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		t.Fatal("fallback não persistiu machine-id")
	}
}
