package database

import (
	"context"
	"encoding/json"
	"testing"
)

func TestConnectionCleanup(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 INSERT INTO providerConnections VALUES('a','p','apikey','A',NULL,7,1,'{"accessToken":null,"refreshToken":"keep","providerSpecificData":{},"unknown":true,"idToken":null}','2026-01-01T00:00:00Z','2026-01-02T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	count, err := (Connections{DB: db}).Cleanup(ctx)
	if err != nil || count != 3 {
		t.Fatalf("cleanup: %d %v", count, err)
	}
	var raw, updated string
	var priority int
	if err = db.QueryRow(`SELECT data,updatedAt,priority FROM providerConnections WHERE id='a'`).Scan(&raw, &updated, &priority); err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"accessToken", "providerSpecificData"} {
		if _, exists := data[key]; exists {
			t.Fatalf("field retained: %s %v", key, data)
		}
	}
	if data["refreshToken"] != "keep" || data["unknown"] != true || updated != "2026-01-02T00:00:00Z" || priority != 7 {
		t.Fatalf("unexpected mutation: %v %s %d", data, updated, priority)
	}
	// idToken não faz parte da lista de limpeza da origem.
	if _, exists := data["idToken"]; !exists {
		t.Fatal("idToken should be retained")
	}
	// email é coluna estruturada NULL: a origem volta a contá-la em cada execução.
	count, err = (Connections{DB: db}).Cleanup(ctx)
	if err != nil || count != 1 {
		t.Fatalf("repeat semantics: %d %v", count, err)
	}
}
