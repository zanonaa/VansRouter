package database

import (
	"context"
	"database/sql"
	"math"
	"time"
)

// ProxyFitness porta proxyPoolFitnessRepo.js. until preserva a unidade da
// origem (timestamp numérico, normalmente milissegundos), sem converter segundos.
type ProxyFitness struct{ DB *sql.DB }
type Fitness struct {
	PoolID    string  `json:"poolId"`
	Scope     string  `json:"scope"`
	Until     float64 `json:"until"`
	Reason    string  `json:"reason"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

func scanFitness(row scanner) (*Fitness, error) {
	value := &Fitness{}
	var reason sql.NullString
	if err := row.Scan(&value.PoolID, &value.Scope, &value.Until, &reason, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return nil, err
	}
	value.Reason = reason.String
	return value, nil
}

const fitnessColumns = "poolId, scope, until, reason, createdAt, updatedAt"

func (f ProxyFitness) List(ctx context.Context, poolID *string) ([]Fitness, error) {
	query := "SELECT " + fitnessColumns + " FROM proxyPoolFitness"
	args := []any{}
	if poolID == nil {
		query += " ORDER BY poolId, scope"
	} else {
		query += " WHERE poolId = ? ORDER BY scope"
		args = append(args, *poolID)
	}
	rows, err := f.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Fitness{}
	for rows.Next() {
		item, err := scanFitness(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}
func (f ProxyFitness) Upsert(ctx context.Context, poolID, scope string, until float64, reason string) (*Fitness, error) {
	if poolID == "" || scope == "" || math.IsNaN(until) || math.IsInf(until, 0) {
		return nil, nil
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err := f.DB.ExecContext(ctx, `INSERT INTO proxyPoolFitness(poolId,scope,until,reason,createdAt,updatedAt) VALUES(?,?,?,?,?,?) ON CONFLICT(poolId,scope) DO UPDATE SET until=excluded.until,reason=excluded.reason,updatedAt=excluded.updatedAt`, poolID, scope, until, reason, now, now)
	if err != nil {
		return nil, err
	}
	return scanFitness(f.DB.QueryRowContext(ctx, "SELECT "+fitnessColumns+" FROM proxyPoolFitness WHERE poolId = ? AND scope = ?", poolID, scope))
}
func (f ProxyFitness) Delete(ctx context.Context, poolID, scope string) (int64, error) {
	return fitnessDelete(ctx, f.DB, "DELETE FROM proxyPoolFitness WHERE poolId = ? AND scope = ?", poolID, scope)
}
func (f ProxyFitness) DeleteByPool(ctx context.Context, poolID string) (int64, error) {
	return fitnessDelete(ctx, f.DB, "DELETE FROM proxyPoolFitness WHERE poolId = ?", poolID)
}
func (f ProxyFitness) Clear(ctx context.Context, provider string) (int64, error) {
	if provider == "" {
		return fitnessDelete(ctx, f.DB, "DELETE FROM proxyPoolFitness")
	}
	// Mantém LIKE e seus curingas como na origem, sem normalizar o provider.
	return fitnessDelete(ctx, f.DB, "DELETE FROM proxyPoolFitness WHERE scope = ? OR scope LIKE ?", provider+"::*", provider+"::%")
}
func fitnessDelete(ctx context.Context, db *sql.DB, query string, args ...any) (int64, error) {
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
