package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func specific(c map[string]any) map[string]any {
	v, _ := c["providerSpecificData"].(map[string]any)
	return v
}
func sameIdentity(old, data map[string]any) bool {
	incoming, existing := specific(data), specific(old)
	if data["provider"] == "cursor" && data["authType"] == "oauth" && jsonTruthy(incoming["machineId"]) {
		return old["provider"] == "cursor" && existing["machineId"] == incoming["machineId"]
	}
	if data["authType"] == "oauth" && jsonTruthy(data["email"]) {
		if old["authType"] != "oauth" || old["email"] != data["email"] {
			return false
		}
		iw, ew := incoming["chatgptAccountId"], existing["chatgptAccountId"]
		if data["provider"] == "codex" {
			return jsonTruthy(iw) && jsonTruthy(ew) && iw == ew
		}
		if jsonTruthy(iw) && jsonTruthy(ew) {
			return iw == ew
		}
		if jsonTruthy(iw) || jsonTruthy(ew) {
			return false
		}
		iu, eu := incoming["username"], existing["username"]
		if jsonTruthy(iu) && jsonTruthy(eu) {
			return iu == eu
		}
		return !jsonTruthy(iu) && !jsonTruthy(eu)
	}
	return data["authType"] == "apikey" && jsonTruthy(data["name"]) && old["authType"] == "apikey" && old["name"] == data["name"]
}

var optionalConnectionFields = []string{"displayName", "email", "globalPriority", "defaultModel", "accessToken", "refreshToken", "expiresAt", "tokenType", "scope", "projectId", "apiKey", "testStatus", "lastTested", "lastError", "lastErrorAt", "rateLimitedUntil", "expiresIn", "errorCode", "consecutiveUseCount", "idToken", "lastRefreshAt"}

// Create reproduz createProviderConnection, incluindo upsert por identidade.
func (c Connections) Create(ctx context.Context, data map[string]any) (map[string]any, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT "+connectionColumns+" FROM providerConnections WHERE provider = ?", data["provider"])
	if err != nil {
		return nil, err
	}
	all := []map[string]any{}
	for rows.Next() {
		item, err := scanConnection(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for _, old := range all {
		if sameIdentity(old, data) {
			patch := healthPatch(old, data)
			for k, v := range patch {
				old[k] = v
			}
			old["updatedAt"] = now
			if err = writeConnection(ctx, tx, old); err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return old, nil
		}
	}
	name := data["name"]
	if !jsonTruthy(name) {
		name = nil
	}
	if name == nil && (data["authType"] == "oauth" || data["authType"] == "access_token") {
		name = data["email"]
		if !jsonTruthy(name) {
			name = fmt.Sprintf("Account %d", len(all)+1)
		}
		if data["provider"] == "github" {
			for _, v := range []any{specific(data)["githubLogin"], specific(data)["githubEmail"], data["email"], specific(data)["githubName"]} {
				if jsonTruthy(v) {
					name = v
					break
				}
			}
		}
	}
	priority := data["priority"]
	if !jsonTruthy(priority) {
		var maximum sql.NullFloat64
		if err = tx.QueryRowContext(ctx, "SELECT MAX(priority) FROM providerConnections WHERE provider = ?", data["provider"]).Scan(&maximum); err != nil {
			return nil, err
		}
		priority = maximum.Float64 + 1
	}
	authType := data["authType"]
	if !jsonTruthy(authType) {
		authType = "oauth"
	}
	active := data["isActive"]
	if _, ok := data["isActive"]; !ok {
		active = true
	}
	conn := map[string]any{"id": uuid.NewString(), "provider": data["provider"], "authType": authType, "name": name, "priority": priority, "isActive": active, "createdAt": now, "updatedAt": now}
	for _, key := range optionalConnectionFields {
		if v, ok := data[key]; ok && v != nil {
			conn[key] = v
		}
	}
	if value := specific(data); len(value) > 0 {
		conn["providerSpecificData"] = value
	}
	if value, ok := data["email"]; ok {
		conn["email"] = value
	}
	if err = writeConnection(ctx, tx, conn); err != nil {
		return nil, err
	}
	if jsonTruthy(data["priority"]) {
		if err = reorderConnections(ctx, tx, data["provider"]); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return conn, nil
}
