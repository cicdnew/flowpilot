package database

import (
	"context"
	"fmt"

	"flowpilot/internal/models"
)

func (db *DB) CreateProxyRoutingPreset(ctx context.Context, preset models.ProxyRoutingPreset) error {
	_, err := db.conn.ExecContext(ctx, `INSERT INTO proxy_routing_presets (id, name, random_by_country, country, fallback, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		preset.ID, preset.Name, boolToInt(preset.RandomByCountry), preset.Country, preset.Fallback, preset.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert proxy routing preset %s: %w", preset.ID, err)
	}
	return nil
}

func (db *DB) ListProxyRoutingPresets(ctx context.Context) ([]models.ProxyRoutingPreset, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, name, random_by_country, country, fallback, created_at
		FROM proxy_routing_presets ORDER BY name ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query proxy routing presets: %w", err)
	}
	defer rows.Close()

	var presets []models.ProxyRoutingPreset
	for rows.Next() {
		var preset models.ProxyRoutingPreset
		var randomByCountry int
		if err := rows.Scan(&preset.ID, &preset.Name, &randomByCountry, &preset.Country, &preset.Fallback, &preset.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan proxy routing preset: %w", err)
		}
		preset.RandomByCountry = randomByCountry != 0
		presets = append(presets, preset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxy routing presets: %w", err)
	}
	return presets, nil
}

func (db *DB) DeleteProxyRoutingPreset(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM proxy_routing_presets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete proxy routing preset %s: %w", id, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for proxy routing preset %s: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("proxy routing preset %s not found", id)
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
