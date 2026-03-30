package database

import "strings"

// DatabaseType identifies the backend driver to use.
type DatabaseType int

const (
	DatabaseSQLite DatabaseType = iota
	DatabaseTurso
)

// DatabaseConfig describes how to connect to the database.
type DatabaseConfig struct {
	URL       string // DSN or libsql:// URL
	AuthToken string // Optional Turso auth token
	LocalPath string // Optional local replica path
}

// DetectType returns the database type from DSN format.
func DetectType(dsn string) DatabaseType {
	trimmed := strings.TrimSpace(strings.ToLower(dsn))
	if strings.HasPrefix(trimmed, "libsql://") {
		return DatabaseTurso
	}
	return DatabaseSQLite
}
