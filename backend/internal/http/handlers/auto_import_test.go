package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func autoFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestKiroAutoImportPriorityAndArn(t *testing.T) {
	home := t.TempDir()
	k := &KiroAutoImport{Home: home}
	cache := filepath.Join(home, ".aws/sso/cache")
	autoFixture(t, filepath.Join(cache, "a.json"), `{"refreshToken":"aorAAAAAG-other"}`)
	autoFixture(t, filepath.Join(cache, "kiro-auth-token.json"), `{"refreshToken":"aorAAAAAG-main","clientIdHash":"registration","region":"eu-west-1"}`)
	autoFixture(t, filepath.Join(cache, "registration.json"), `{"clientId":"client","clientSecret":"secret"}`)
	autoFixture(t, k.profilePaths()[1], `{"arn":"arn:aws:codewhisperer:eu-west-1:123:profile/example"}`)
	w := httptest.NewRecorder()
	k.Get(w, httptest.NewRequest("GET", "/", nil))
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["refreshToken"] != "aorAAAAAG-main" || got["clientId"] != "client" || got["profileArn"] != "arn:aws:codewhisperer:us-east-1:123:profile/example" {
		t.Fatalf("unexpected result: %v", got)
	}
	for _, arn := range []string{"not-an-arn", "arn:aws:other:eu:123:p", "arn:aws:codewhisperer::123:p"} {
		if kiroNormalizeArn(arn) != arn {
			t.Fatal("unexpected ARN modification")
		}
	}
}

func TestCursorAutoImportSQLite(t *testing.T) {
	c := &CursorAutoImport{Home: t.TempDir(), Platform: "linux", Installed: true}
	path := c.candidatePaths()[0]
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE itemTable(key TEXT,value TEXT); INSERT INTO itemTable VALUES ('cursorAuth/token','"test-token"'),('telemetry.machineId','machine');`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	w := httptest.NewRecorder()
	c.Get(w, httptest.NewRequest("GET", "/", nil))
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["found"] != true || got["accessToken"] != "test-token" || got["machineId"] != "machine" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestZedAutoImportKeyringAndSystemID(t *testing.T) {
	z := &ZedAutoImport{Home: t.TempDir(), Platform: "linux", UUID: func() string { return "fallback" }}
	z.Run = func(context.Context, time.Duration, string, ...string) (string, string, error) {
		return "label = zed-github-account\nsecret = test-token\n", "attribute.username = 123", nil
	}
	path := z.globalDbPaths()[0]
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE kv_store(key TEXT,value TEXT); INSERT INTO kv_store VALUES ('system_id','stored-id');`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	got := z.Credentials(context.Background())
	if got["found"] != true || got["userId"] != "123" || got["systemId"] != "stored-id" {
		t.Fatalf("unexpected: %v", got)
	}
}
