package database

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestReadAuthSettings(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	settings, err := ReadAuthSettings(ctx, db)
	if err != nil || len(settings) != 0 {
		t.Fatal("banco novo deve retornar configuração vazia")
	}
	if _, err := db.Exec(`CREATE TABLE settings (id INTEGER PRIMARY KEY, data TEXT NOT NULL); INSERT INTO settings VALUES (1, '{"requireLogin":false,"password":"hash-ficticio","opcaoDesconhecida":123}')`); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadAuthSettings(ctx, db)
	if err != nil || settings["requireLogin"] != false || settings["password"] != "hash-ficticio" {
		t.Fatal("configuração existente não lida")
	}
	if _, err := db.Exec(`UPDATE settings SET data='invalido'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAuthSettings(ctx, db); err == nil {
		t.Fatal("configuração inválida aceita")
	}
}

func TestMergeWithDefaults(t *testing.T) {
	merged := MergeWithDefaults(nil)
	if merged["requireLogin"] != true || merged["authMode"] != "password" || merged["requireApiKey"] != false {
		t.Fatalf("defaults divergentes: %v", merged)
	}
	// O banco sobrescreve os defaults e preserva null.
	merged = MergeWithDefaults(map[string]any{"requireLogin": false, "comboStrategy": nil, "opcaoDesconhecida": float64(7)})
	if merged["requireLogin"] != false || merged["comboStrategy"] != nil || merged["opcaoDesconhecida"] != float64(7) {
		t.Fatalf("mesclagem divergente: %v", merged)
	}
	if merged["authMode"] != "password" {
		t.Fatal("default ausente após mesclagem")
	}
}

func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := ChangePassword(ctx, db, "hash-um"); err != nil {
		t.Fatalf("banco novo: %v", err)
	}
	settings, err := ReadAuthSettings(ctx, db)
	if err != nil || settings["password"] != "hash-um" {
		t.Fatalf("primeira gravação: %v %v", settings, err)
	}
	// Mesclagem preserva chaves desconhecidas existentes.
	if _, err := db.Exec(`UPDATE settings SET data='{"password":"hash-um","opcaoDesconhecida":123,"requireLogin":false}'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ChangePassword(ctx, db, "hash-dois"); err != nil {
		t.Fatal(err)
	}
	settings, err = ReadAuthSettings(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if settings["password"] != "hash-dois" || settings["opcaoDesconhecida"] != float64(123) || settings["requireLogin"] != false {
		t.Fatalf("mesclagem do troca de senha divergente: %v", settings)
	}
	var revision string
	if err := db.QueryRow(`SELECT value FROM _meta WHERE key='settings_revision'`).Scan(&revision); err != nil {
		t.Fatalf("revision ausente: %v", err)
	}
	if revision != "2" {
		t.Fatalf("revision esperado 2, veio %q", revision)
	}
	// settings_revision pré-existente é incrementado, não zerado; JSON inválido falha.
	if _, err := db.Exec(`UPDATE _meta SET value='5' WHERE key='settings_revision'; UPDATE settings SET data='invalido'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ChangePassword(ctx, db, "hash-três"); err == nil {
		t.Fatal("configuração inválida aceita")
	}
	data, _ := json.Marshal(settings)
	if !bytes.Contains(data, []byte("hash-dois")) {
		t.Fatal("testes inconsistentes")
	}
}
