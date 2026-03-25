package database

import (
	"context"
	"fmt"
)

func (db *DB) UpdateProxyRateLimit(ctx context.Context, id string, maxRequestsPerMinute int) error {
	res, err := db.conn.ExecContext(ctx, `UPDATE proxies SET max_requests_per_minute = ? WHERE id = ?`, maxRequestsPerMinute, id)
	if err != nil {
		return fmt.Errorf("update proxy %s rate limit: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rate-limit update for proxy %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("proxy %s not found", id)
	}
	return nil
}
