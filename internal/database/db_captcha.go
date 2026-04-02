package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/models"
)

func (db *DB) CreateCaptchaConfig(ctx context.Context, c models.CaptchaConfig) error {
	encKey, err := crypto.Encrypt(c.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt captcha api key: %w", err)
	}

	enabled := 0
	if c.Enabled {
		enabled = 1
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO captcha_config (id, provider, api_key, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Provider, encKey, enabled, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert captcha config %s: %w", c.ID, err)
	}
	return nil
}

func (db *DB) GetCaptchaConfig(ctx context.Context, id string) (*models.CaptchaConfig, error) {
	row := db.readConn.QueryRowContext(ctx, `
		SELECT id, provider, api_key, enabled, created_at, updated_at
		FROM captcha_config WHERE id = ?`, id)

	c, err := db.scanCaptchaRow(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("captcha config %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get captcha config %s: %w", id, err)
	}
	return c, nil
}

func (db *DB) GetActiveCaptchaConfig(ctx context.Context) (*models.CaptchaConfig, error) {
	row := db.readConn.QueryRowContext(ctx, `
		SELECT id, provider, api_key, enabled, created_at, updated_at
		FROM captcha_config WHERE enabled = 1 LIMIT 1`)

	c, err := db.scanCaptchaRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active captcha config: %w", err)
	}
	return c, nil
}

func (db *DB) ListCaptchaConfigs(ctx context.Context) ([]models.CaptchaConfig, error) {
	rows, err := db.readConn.QueryContext(ctx, `
		SELECT id, provider, api_key, enabled, created_at, updated_at
		FROM captcha_config ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query captcha configs: %w", err)
	}
	defer rows.Close()

	var configs []models.CaptchaConfig
	for rows.Next() {
		var c models.CaptchaConfig
		var enabled int
		if err := rows.Scan(&c.ID, &c.Provider, &c.APIKey, &enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan captcha config row: %w", err)
		}
		c.Enabled = enabled != 0

		decKey, err := crypto.Decrypt(c.APIKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt captcha api key for %s: %w", c.ID, err)
		}
		c.APIKey = decKey
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate captcha configs: %w", err)
	}
	return configs, nil
}

func (db *DB) UpdateCaptchaConfig(ctx context.Context, c models.CaptchaConfig) error {
	encKey, err := crypto.Encrypt(c.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt captcha api key: %w", err)
	}

	enabled := 0
	if c.Enabled {
		enabled = 1
	}

	res, err := db.conn.ExecContext(ctx, `
		UPDATE captcha_config SET provider = ?, api_key = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		c.Provider, encKey, enabled, time.Now(), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update captcha config %s: %w", c.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result for captcha config %s: %w", c.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("captcha config %s not found", c.ID)
	}
	return nil
}

func (db *DB) DeleteCaptchaConfig(ctx context.Context, id string) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM captcha_config WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete captcha config %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result for captcha config %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("captcha config %s not found", id)
	}
	return nil
}

func (db *DB) scanCaptchaRow(row *sql.Row) (*models.CaptchaConfig, error) {
	var c models.CaptchaConfig
	var enabled int
	if err := row.Scan(&c.ID, &c.Provider, &c.APIKey, &enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Enabled = enabled != 0

	decKey, err := crypto.Decrypt(c.APIKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt captcha api key for %s: %w", c.ID, err)
	}
	c.APIKey = decKey
	return &c, nil
}
