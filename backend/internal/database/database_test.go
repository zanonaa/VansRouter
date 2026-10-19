package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(context.Background(), dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for pragma, want := range map[string]string{"journal_mode": "wal", "foreign_keys": "1", "busy_timeout": "5000", "synchronous": "1"} {
		var got string
		if err := db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s = %s; esperado %s", pragma, got, want)
		}
	}
	if _, err := db.Exec("CREATE TABLE exemplo (id INTEGER PRIMARY KEY, valor TEXT); INSERT INTO exemplo VALUES (1, 'preservado')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(context.Background(), dir, "FULL")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var got string
	if err := reopened.QueryRow("SELECT valor FROM exemplo WHERE id=1").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "preservado" {
		t.Fatal("registro alterado")
	}
}

func TestCorruptFileIsPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db", "data.sqlite")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	before := []byte("arquivo inválido, não substituir")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if db, err := Open(context.Background(), dir, "NORMAL"); err == nil {
		db.Close()
		t.Fatal("banco inválido aceito")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("arquivo inválido foi modificado")
	}
}
