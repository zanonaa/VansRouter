package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func apiKeysDB(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return ctx, db
}

func TestCreateApiKeyFormat(t *testing.T) {
	ctx, db := apiKeysDB(t)
	machineId := "57309511d27c70bf"
	key, err := CreateApiKey(ctx, db, "chave-teste", machineId)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := key["key"].(string)
	if !strings.HasPrefix(raw, "sk-"+machineId+"-") {
		t.Fatalf("formato divergente: %q", raw)
	}
	parts := strings.Split(raw, "-")
	if len(parts) != 4 || len(parts[2]) != 6 || len(parts[3]) != 8 {
		t.Fatalf("estrutura divergente: %q", raw)
	}
	if key["name"] != "chave-teste" || key["isActive"] != true || key["machineId"] != machineId {
		t.Fatalf("campos divergentes: %v", key)
	}
	if key["allowedProviders"] != nil || key["allowedCombos"] != nil || key["allowedKinds"] != nil {
		t.Fatalf("permissões iniciais divergentes: %v", key)
	}
}

func TestApiKeyCRUDAndValidate(t *testing.T) {
	ctx, db := apiKeysDB(t)
	created, err := CreateApiKey(ctx, db, "original", "57309511d27c70bf")
	if err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)
	rawKey := created["key"].(string)

	keys, err := GetApiKeys(ctx, db)
	if err != nil || len(keys) != 1 {
		t.Fatalf("listagem: %v %v", keys, err)
	}
	got, err := GetApiKeyById(ctx, db, id)
	if err != nil || got == nil || got["key"] != rawKey {
		t.Fatalf("leitura por id: %v %v", got, err)
	}
	missing, err := GetApiKeyById(ctx, db, "id-inexistente")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatal("id inexistente deveria retornar nil")
	}

	// Validação: ativa ok, desativada nil, desconhecida nil.
	valid, err := ValidateApiKey(ctx, db, rawKey)
	if err != nil || valid == nil || valid["id"] != id {
		t.Fatalf("validação ativa: %v %v", valid, err)
	}
	if _, err := UpdateApiKey(ctx, db, id, map[string]any{"isActive": false}); err != nil {
		t.Fatal(err)
	}
	if v, _ := ValidateApiKey(ctx, db, rawKey); v != nil {
		t.Fatal("key desativada validada")
	}
	if v, _ := ValidateApiKey(ctx, db, "sk-desconhecida"); v != nil {
		t.Fatal("key desconhecida validada")
	}

	// Update de permissões: null → all, [] → none, [x] → lista.
	updated, err := UpdateApiKey(ctx, db, id, map[string]any{"isActive": true, "allowedProviders": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	providers, _ := updated["allowedProviders"].([]any)
	if len(providers) != 0 {
		t.Fatalf("[] deveria ser none: %v", updated["allowedProviders"])
	}
	updated, err = UpdateApiKey(ctx, db, id, map[string]any{"allowedProviders": []any{"openai", "anthropic"}})
	if err != nil {
		t.Fatal(err)
	}
	providers, _ = updated["allowedProviders"].([]any)
	if len(providers) != 2 {
		t.Fatalf("lista divergente: %v", updated["allowedProviders"])
	}
	updated, err = UpdateApiKey(ctx, db, id, map[string]any{"allowedProviders": nil})
	if err != nil {
		t.Fatal(err)
	}
	if updated["allowedProviders"] != nil {
		t.Fatalf("null deveria ser all: %v", updated["allowedProviders"])
	}
	// valor inválido (não-array) trata como null
	updated, err = UpdateApiKey(ctx, db, id, map[string]any{"allowedKinds": "invalido"})
	if err != nil {
		t.Fatal(err)
	}
	if updated["allowedKinds"] != nil {
		t.Fatalf("tipo inválido deveria virar null: %v", updated["allowedKinds"])
	}

	// Delete
	ok, err := DeleteApiKey(ctx, db, id)
	if err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, _ := DeleteApiKey(ctx, db, id); ok {
		t.Fatal("delete duplicado")
	}
	if keys, _ := GetApiKeys(ctx, db); len(keys) != 0 {
		t.Fatal("lista não vazia após delete")
	}
}

func TestApiKeyPersistenceOfPermLists(t *testing.T) {
	ctx, db := apiKeysDB(t)
	created, _ := CreateApiKey(ctx, db, "perm", "57309511d27c70bf")
	id := created["id"].(string)
	if _, err := UpdateApiKey(ctx, db, id, map[string]any{"allowedCombos": []any{"combo-a"}}); err != nil {
		t.Fatal(err)
	}
	got, err := GetApiKeyById(ctx, db, id)
	if err != nil {
		t.Fatal(err)
	}
	combos, _ := got["allowedCombos"].([]any)
	if len(combos) != 1 || combos[0] != "combo-a" {
		t.Fatalf("persistência de permissões divergente: %v", got["allowedCombos"])
	}
}
