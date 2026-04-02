package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestListDueSchedulesSkipsUndecryptableRows(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	good := makeSchedule("sched-good", "Good Schedule")
	nextRun := now.Add(-time.Minute)
	good.NextRunAt = &nextRun
	if err := db.CreateSchedule(ctx, good); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	// Use valid base64 of random bytes that will fail AES-GCM decryption
	// (valid base64 but not a valid ciphertext for the test key).
	badCiphertext := base64.StdEncoding.EncodeToString([]byte("this-is-not-a-valid-aes-gcm-ciphertext-padding-x"))

	tagsJSON, err := json.Marshal([]string{"bad"})
	if err != nil {
		t.Fatalf("Marshal tags: %v", err)
	}
	_, err = db.Conn().ExecContext(ctx, `
		INSERT INTO schedules (id, name, cron_expr, flow_id, url, proxy_server, proxy_username, proxy_password, proxy_geo, proxy_protocol, priority, headless, tags, enabled, last_run_at, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"sched-bad", "Bad Schedule", "*/5 * * * *", "flow-bad", "https://example.com",
		"proxy.example.com:8080", badCiphertext, badCiphertext, "US", "http",
		1, 1, string(tagsJSON), 1, nil, nextRun, now, now,
	)
	if err != nil {
		t.Fatalf("insert bad schedule row: %v", err)
	}

	due, err := db.ListDueSchedules(ctx, now)
	if err != nil {
		t.Fatalf("ListDueSchedules: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 valid due schedule, got %d", len(due))
	}
	if due[0].ID != good.ID {
		t.Fatalf("expected %q, got %q", good.ID, due[0].ID)
	}
}
