package database

import (
	"context"
	"testing"
)

func TestUpdateSettings(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Primeira escrita em banco novo.
	merged, err := UpdateSettings(ctx, db, map[string]any{"requireLogin": false, "authMode": "oidc", "theme": "dark"})
	if err != nil {
		t.Fatal(err)
	}
	if merged["requireLogin"] != false || merged["theme"] != "dark" {
		t.Fatalf("mesclagem inicial: %v", merged)
	}
	// Escrita incremental preserva chaves existentes.
	merged, err = UpdateSettings(ctx, db, map[string]any{"oidcIssuerUrl": "https://idp"})
	if err != nil {
		t.Fatal(err)
	}
	if merged["requireLogin"] != false || merged["oidcIssuerUrl"] != "https://idp" || merged["theme"] != "dark" {
		t.Fatalf("mesclagem incremental: %v", merged)
	}
	// Valor nil remove a chave (semântica do oidcClientSecret vazio).
	merged, err = UpdateSettings(ctx, db, map[string]any{"theme": nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := merged["theme"]; exists {
		t.Fatalf("nil deveria remover: %v", merged)
	}
	// Revision incrementa a cada escrita.
	var revision string
	if err := db.QueryRow(`SELECT value FROM _meta WHERE key='settings_revision'`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if revision != "3" {
		t.Fatalf("revision esperado 3, veio %q", revision)
	}
	// Persistência real.
	raw, err := ReadAuthSettings(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if raw["requireLogin"] != false || raw["authMode"] != "oidc" || raw["oidcIssuerUrl"] != "https://idp" {
		t.Fatalf("persistência divergente: %v", raw)
	}
}
