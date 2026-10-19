package database

import (
	"context"
	"testing"
)

func TestConnectionIdentityRules(t *testing.T) {
	conn := func(provider, auth, email, name, workspace, username, machine string) map[string]any {
		return map[string]any{"provider": provider, "authType": auth, "email": email, "name": name, "providerSpecificData": map[string]any{"chatgptAccountId": workspace, "username": username, "machineId": machine}}
	}
	for _, tt := range []struct {
		name    string
		old, in map[string]any
		want    bool
	}{
		{"apikey name", conn("p", "apikey", "", "A", "", "", ""), conn("p", "apikey", "", "A", "", "", ""), true},
		{"apikey different", conn("p", "apikey", "", "A", "", "", ""), conn("p", "apikey", "", "B", "", "", ""), false},
		{"access token never", conn("p", "access_token", "e", "A", "", "", ""), conn("p", "access_token", "e", "A", "", "", ""), false},
		{"oauth email", conn("p", "oauth", "e", "", "", "", ""), conn("p", "oauth", "e", "", "", "", ""), true},
		{"username mismatch", conn("p", "oauth", "e", "", "", "u", ""), conn("p", "oauth", "e", "", "", "v", ""), false},
		{"username only one", conn("p", "oauth", "e", "", "", "u", ""), conn("p", "oauth", "e", "", "", "", ""), false},
		{"workspace same", conn("p", "oauth", "e", "", "w", "", ""), conn("p", "oauth", "e", "", "w", "", ""), true},
		{"workspace one", conn("p", "oauth", "e", "", "w", "", ""), conn("p", "oauth", "e", "", "", "", ""), false},
		{"codex bare email distinct", conn("codex", "oauth", "e", "", "", "", ""), conn("codex", "oauth", "e", "", "", "", ""), false},
		{"codex workspace same", conn("codex", "oauth", "e", "", "w", "", ""), conn("codex", "oauth", "e", "", "w", "", ""), true},
		{"cursor machine precedence", conn("cursor", "oauth", "old", "", "", "", "m"), conn("cursor", "oauth", "new", "", "", "", "m"), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameIdentity(tt.old, tt.in); got != tt.want {
				t.Fatalf("identity: %v want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionCreateUpsert(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	repo := Connections{DB: db}
	first, err := repo.Create(ctx, map[string]any{"provider": "p", "authType": "apikey", "name": "A", "apiKey": "old", "lastError": "err", "testStatus": "error"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Create(ctx, map[string]any{"provider": "p", "authType": "apikey", "name": "B", "apiKey": "second"})
	if err != nil {
		t.Fatal(err)
	}
	if first["priority"] != float64(1) || second["priority"] != float64(2) {
		t.Fatalf("priorities: %v %v", first, second)
	}
	updated, err := repo.Create(ctx, map[string]any{"provider": "p", "authType": "apikey", "name": "A", "apiKey": "new", "testStatus": "active"})
	if err != nil {
		t.Fatal(err)
	}
	if updated["id"] != first["id"] || updated["createdAt"] != first["createdAt"] || updated["apiKey"] != "new" || updated["lastError"] != nil {
		t.Fatalf("upsert: %v", updated)
	}
	list, err := repo.List(ctx, "p", nil)
	if err != nil || len(list) != 2 {
		t.Fatalf("dedup: %v %v", list, err)
	}
	github, err := repo.Create(ctx, map[string]any{"provider": "github", "authType": "oauth", "email": "e", "providerSpecificData": map[string]any{"githubLogin": "login"}})
	if err != nil || github["name"] != "login" {
		t.Fatalf("github name: %v %v", github, err)
	}
	for i := 0; i < 2; i++ {
		if _, err = repo.Create(ctx, map[string]any{"provider": "codex", "authType": "oauth", "email": "e"}); err != nil {
			t.Fatal(err)
		}
	}
	list, err = repo.List(ctx, "codex", nil)
	if err != nil || len(list) != 2 {
		t.Fatalf("codex grants collapsed: %v %v", list, err)
	}
	// Uma escrita inválida não pode deixar nova linha parcial.
	if _, err = repo.Create(ctx, map[string]any{"provider": "p", "authType": "apikey", "name": "invalid", "providerSpecificData": map[string]any{"bad": make(chan int)}}); err == nil {
		t.Fatal("expected JSON error")
	}
	list, err = repo.List(ctx, "p", nil)
	if err != nil || len(list) != 2 {
		t.Fatalf("rollback: %v %v", list, err)
	}
}
