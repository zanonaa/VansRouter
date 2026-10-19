package database

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// dataDirForSecrets é configurado em main.go via SetDataDir.
var dataDirForSecrets string

// SetDataDir configura o diretório usado para segredos de API key.
func SetDataDir(dir string) { dataDirForSecrets = dir }

func resolvedDataDir() string {
	if dataDirForSecrets != "" {
		return dataDirForSecrets
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".9router")
}
func ensureApiKeysTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS apiKeys (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL,
		name TEXT,
		machineId TEXT,
		isActive INTEGER DEFAULT 1,
		createdAt TEXT NOT NULL,
		allowedProviders TEXT,
		allowedCombos TEXT,
		allowedKinds TEXT
	)`)
	return err
}

func parsePermList(raw sql.NullString) any {
	if !raw.Valid || raw.String == "" {
		// DB NULL → null (all allowed); "[]" → [] (none).
		if !raw.Valid {
			return nil
		}
	}
	trimmed := strings.TrimSpace(raw.String)
	if trimmed == "" {
		if !raw.Valid {
			return nil
		}
		return nil
	}
	if trimmed == "null" {
		return nil
	}
	var out any
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil
	}
	if out == nil {
		return nil
	}
	if arr, ok := out.([]any); ok {
		return arr
	}
	return nil
}

// rowToApiKey converte uma linha do banco para o mapa usado pela API.
func rowToApiKey(row *sql.Rows) (map[string]any, error) {
	// placeholder; implementado via scan manual abaixo
	return nil, errors.New("não utilizado")
}

func scanApiKey(rows *sql.Rows) (map[string]any, error) {
	var id, key, createdAt string
	var name, machineId sql.NullString
	var isActive sql.NullInt64
	var allowedProviders, allowedCombos, allowedKinds sql.NullString
	if err := rows.Scan(&id, &key, &name, &machineId, &isActive, &createdAt, &allowedProviders, &allowedCombos, &allowedKinds); err != nil {
		return nil, err
	}
	active := true
	if isActive.Valid {
		active = isActive.Int64 == 1
	}
	out := map[string]any{
		"id":        id,
		"key":       key,
		"name":      nullString(name),
		"machineId": nullString(machineId),
		"isActive":  active,
		"createdAt": createdAt,
	}
	// parsePermList retorna nil (all) ou [] (none) ou [x].
	out["allowedProviders"] = parsePermList(allowedProviders)
	out["allowedCombos"] = parsePermList(allowedCombos)
	out["allowedKinds"] = parsePermList(allowedKinds)
	return out, nil
}

func nullString(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}

func GetApiKeys(ctx context.Context, db *sql.DB) ([]map[string]any, error) {
	if err := ensureApiKeysTable(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, key, name, machineId, isActive, createdAt, allowedProviders, allowedCombos, allowedKinds FROM apiKeys ORDER BY createdAt ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		item, err := scanApiKey(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func GetApiKeyById(ctx context.Context, db *sql.DB, id string) (map[string]any, error) {
	if err := ensureApiKeysTable(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, key, name, machineId, isActive, createdAt, allowedProviders, allowedCombos, allowedKinds FROM apiKeys WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	item, err := scanApiKey(rows)
	if err != nil {
		return nil, err
	}
	return item, rows.Err()
}

func CreateApiKey(ctx context.Context, db *sql.DB, name string, machineId string) (map[string]any, error) {
	if machineId == "" {
		return nil, errors.New("machineId is required")
	}
	if err := ensureApiKeysTable(ctx, db); err != nil {
		return nil, err
	}
	// geração do key segue o utilitário compartilhado (Go reimplementa).
	keyId := generateKeyId()
	crc := generateCrc(machineId, keyId)
	key := fmt.Sprintf("sk-%s-%s-%s", machineId, keyId, crc)
	id := uuid.NewString()
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `INSERT INTO apiKeys(id, key, name, machineId, isActive, createdAt, allowedProviders, allowedCombos, allowedKinds) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, key, name, machineId, 1, createdAt, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":        id,
		"key":       key,
		"name":      name,
		"machineId": machineId,
		"isActive":  true,
		"createdAt": createdAt,
	}, nil
}

func UpdateApiKey(ctx context.Context, db *sql.DB, id string, data map[string]any) (map[string]any, error) {
	if err := ensureApiKeysTable(ctx, db); err != nil {
		return nil, err
	}
	existing, err := GetApiKeyById(ctx, db, id)
	if err != nil || existing == nil {
		return nil, err
	}
	merged := map[string]any{}
	for k, v := range existing {
		merged[k] = v
	}
	if v, ok := data["isActive"]; ok {
		merged["isActive"] = v
	}
	if v, ok := data["name"]; ok && v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			merged["name"] = strings.TrimSpace(s)
		}
	}
	for _, field := range []string{"allowedProviders", "allowedCombos", "allowedKinds"} {
		if v, ok := data[field]; ok {
			if v == nil {
				merged[field] = nil
			} else if arr, ok := v.([]any); ok {
				merged[field] = arr
			} else {
				merged[field] = nil
			}
		}
	}
	active := 0
	if b, ok := merged["isActive"].(bool); ok && b {
		active = 1
	}
	providers := serializePermList(merged["allowedProviders"])
	combos := serializePermList(merged["allowedCombos"])
	kinds := serializePermList(merged["allowedKinds"])
	_, err = db.ExecContext(ctx, `UPDATE apiKeys SET key = ?, name = ?, machineId = ?, isActive = ?, allowedProviders = ?, allowedCombos = ?, allowedKinds = ? WHERE id = ?`,
		merged["key"], merged["name"], merged["machineId"], active, providers, combos, kinds, id)
	if err != nil {
		return nil, err
	}
	merged["isActive"] = active == 1
	return merged, nil
}

func DeleteApiKey(ctx context.Context, db *sql.DB, id string) (bool, error) {
	if err := ensureApiKeysTable(ctx, db); err != nil {
		return false, err
	}
	res, err := db.ExecContext(ctx, `DELETE FROM apiKeys WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func ValidateApiKey(ctx context.Context, db *sql.DB, key string) (map[string]any, error) {
	if err := ensureApiKeysTable(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id, key, name, machineId, isActive, createdAt, allowedProviders, allowedCombos, allowedKinds FROM apiKeys WHERE key = ?`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	item, err := scanApiKey(rows)
	if err != nil {
		return nil, err
	}
	if isActive, ok := item["isActive"].(bool); ok && !isActive {
		return nil, nil
	}
	return item, rows.Err()
}

func serializePermList(v any) any {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return nil
	}
	return string(b)
}

// generateCrc reproduz o CRC de 8 chars HMAC-SHA256.
func generateCrc(machineId, keyId string) string {
	secret := apiKeySecret()
	mac := hmacSHA256(secret, machineId+keyId)
	return hexEncode(mac)[:8]
}

// generateKeyId reproduz o gerador de 6 chars do utilitário JS.
func generateKeyId() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "aaaaaa"
	}
	out := make([]byte, 6)
	for i := 0; i < 6; i++ {
		out[i] = chars[int(b[i])%len(chars)]
	}
	return string(out)
}

// apiKeySecret reproduz getApiKeySecret() (DATA_DIR/auth/api-key-secret).
func apiKeySecret() string {
	dataDir := resolvedDataDir()
	file := filepath.Join(dataDir, "auth", "api-key-secret")
	if data, err := os.ReadFile(file); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000000000000000000000000000000000000000000000000000"
	}
	secret := hex.EncodeToString(b)
	_ = os.MkdirAll(filepath.Dir(file), 0700)
	_ = os.WriteFile(file, []byte(secret), 0600)
	return secret
}

func hmacSHA256(secret, data string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}
