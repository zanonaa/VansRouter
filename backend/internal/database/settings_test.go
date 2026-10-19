package database

import (
	"context"
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
