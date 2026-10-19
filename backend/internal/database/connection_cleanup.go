package database

import "context"

// Cleanup reproduz cleanupProviderConnections sem alterar timestamps ou
// prioridades. Conta campos removidos, não quantidade de connections.
func (c Connections) Cleanup(ctx context.Context) (int, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT "+connectionColumns+" FROM providerConnections")
	if err != nil {
		return 0, err
	}
	connections := []map[string]any{}
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		connections = append(connections, connection)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	fields := []string{"displayName", "email", "globalPriority", "defaultModel", "accessToken", "refreshToken", "expiresAt", "tokenType", "scope", "projectId", "apiKey", "testStatus", "lastTested", "lastError", "lastErrorAt", "rateLimitedUntil", "expiresIn", "consecutiveUseCount"}
	cleaned := 0
	for _, connection := range connections {
		dirty := false
		for _, field := range fields {
			if value, exists := connection[field]; exists && value == nil {
				delete(connection, field)
				cleaned++
				dirty = true
			}
		}
		if data, ok := connection["providerSpecificData"].(map[string]any); ok && len(data) == 0 {
			delete(connection, "providerSpecificData")
			cleaned++
			dirty = true
		}
		if dirty {
			if err = writeConnection(ctx, tx, connection); err != nil {
				return 0, err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return cleaned, nil
}
