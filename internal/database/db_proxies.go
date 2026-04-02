package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

func (db *DB) scanProxyRow(rows *sql.Rows) (*models.Proxy, error) {
	var p models.Proxy
	var lastChecked sql.NullTime

	err := rows.Scan(
		&p.ID, &p.Server, &p.Protocol, &p.Username, &p.Password,
		&p.Geo, &p.Status, &p.Latency, &p.SuccessRate, &p.TotalUsed, &p.MaxRequestsPerMinute,
		&lastChecked, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastChecked.Valid {
		p.LastChecked = &lastChecked.Time
	}

	if p.Username != "" {
		decUser, err := crypto.Decrypt(p.Username)
		if err != nil {
			return nil, fmt.Errorf("decrypt proxy username for %s: %w", p.ID, err)
		}
		p.Username = decUser
	}
	if p.Password != "" {
		decPass, err := crypto.Decrypt(p.Password)
		if err != nil {
			return nil, fmt.Errorf("decrypt proxy password for %s: %w", p.ID, err)
		}
		p.Password = decPass
	}

	return &p, nil
}

func (db *DB) CreateProxy(ctx context.Context, proxy models.Proxy) error {
	encUsername, err := crypto.Encrypt(proxy.Username)
	if err != nil {
		return fmt.Errorf("encrypt proxy username: %w", err)
	}
	encPassword, err := crypto.Encrypt(proxy.Password)
	if err != nil {
		return fmt.Errorf("encrypt proxy password: %w", err)
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO proxies (id, server, protocol, username, password, geo, status, max_requests_per_minute, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proxy.ID, proxy.Server, proxy.Protocol, encUsername, encPassword,
		proxy.Geo, proxy.Status, proxy.MaxRequestsPerMinute, proxy.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert proxy %s: %w", proxy.ID, err)
	}
	return nil
}

func (db *DB) ListProxies(ctx context.Context) ([]models.Proxy, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, max_requests_per_minute, last_checked, created_at
		FROM proxies ORDER BY success_rate DESC, latency ASC`)
	if err != nil {
		return nil, fmt.Errorf("query proxies: %w", err)
	}
	defer rows.Close()

	var proxies []models.Proxy
	for rows.Next() {
		p, err := db.scanProxyRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan proxy row: %w", err)
		}
		proxies = append(proxies, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxies: %w", err)
	}
	return proxies, nil
}

func (db *DB) ListProxiesBestEffort(ctx context.Context) ([]models.Proxy, int, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, max_requests_per_minute, last_checked, created_at
		FROM proxies ORDER BY success_rate DESC, latency ASC`)
	if err != nil {
		return nil, 0, fmt.Errorf("query proxies: %w", err)
	}
	defer rows.Close()

	var proxies []models.Proxy
	skipped := 0
	for rows.Next() {
		p, err := db.scanProxyRow(rows)
		if err != nil {
			skipped++
			log.Printf("skip invalid proxy row: %v", err)
			continue
		}
		proxies = append(proxies, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, skipped, fmt.Errorf("iterate proxies: %w", err)
	}
	return proxies, skipped, nil
}

func (db *DB) ListHealthyProxies(ctx context.Context) ([]models.Proxy, error) {
	rows, err := db.readConn.QueryContext(ctx, `SELECT id, server, protocol, username, password, geo, status, latency, success_rate, total_used, max_requests_per_minute, last_checked, created_at
		FROM proxies WHERE status = 'healthy' ORDER BY success_rate DESC, latency ASC`)
	if err != nil {
		return nil, fmt.Errorf("query healthy proxies: %w", err)
	}
	defer rows.Close()

	var proxies []models.Proxy
	for rows.Next() {
		p, err := db.scanProxyRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan proxy row: %w", err)
		}
		proxies = append(proxies, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proxies: %w", err)
	}
	return proxies, nil
}

func (db *DB) UpdateProxyHealth(ctx context.Context, id string, status models.ProxyStatus, latency int) error {
	now := time.Now()
	res, err := db.conn.ExecContext(ctx, `UPDATE proxies SET status = ?, latency = ?, last_checked = ? WHERE id = ?`,
		status, latency, now, id)
	if err != nil {
		return fmt.Errorf("update proxy %s health: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check health update for proxy %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("proxy %s not found", id)
	}
	return nil
}

func (db *DB) IncrementProxyUsage(ctx context.Context, id string, success bool) error {
	var err error
	if success {
		_, err = db.conn.ExecContext(ctx, `UPDATE proxies SET total_used = total_used + 1,
			success_rate = (success_rate * total_used + 1.0) / (total_used + 1) WHERE id = ?`, id)
	} else {
		_, err = db.conn.ExecContext(ctx, `UPDATE proxies SET total_used = total_used + 1,
			success_rate = (success_rate * total_used) / (total_used + 1) WHERE id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("increment proxy %s usage: %w", id, err)
	}
	return nil
}

func (db *DB) DeleteProxy(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM proxies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete proxy %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for proxy %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("proxy %s not found", id)
	}
	return nil
}
