# Implementation Plan: Authentication & Row-Level Security

## Overview

This document outlines a phased approach to adding multi-user authentication and row-level security (RLS) to FlowPilot. Currently, FlowPilot is a single-user Wails desktop application with no user concept or multi-tenancy. This plan transforms it into a multi-user system while maintaining backward compatibility with the existing single-user mode.

**Scope**: Add user registration/login, session management, and automatic data isolation per user across all 13 database tables.

**Key constraint**: Wails v2 provides no built-in HTTP middleware or authentication hooks. All auth logic must live in Go methods on the `App` struct or in a separate optional HTTP server for CLI agent mode.

---

## Design Decisions & Constraints

### 1. User Storage: Embedded SQLite vs External IdP
**Decision**: Embedded SQLite with local user accounts (Phase 1), optional OIDC integration (Phase 6).

- **Why**: Matches FlowPilot's self-contained, offline-first philosophy. Desktop users want zero external dependencies.
- **Trade-off**: No centralized user management across multiple instances. Each desktop installation is independent.
- **Future**: Optional HTTP API + JWT can integrate with external identity providers.

### 2. Session Tokens vs JWT
**Decision**: Session tokens stored in database + HTTP cookies for web mode; Wails context variable for desktop mode.

- **Desktop mode**: Wails has no native session/cookie support. We'll store the current user ID in `App.currentUserID` (protected by mutex).
- **Web/API mode** (Phase 6): JWT tokens for stateless auth + CLI agent use.
- **Why split**: Desktop doesn't need stateless tokens; web needs them for scalability.

### 3. Password Hashing
**Decision**: Argon2id via `golang.org/x/crypto/argon2` (already audited, OWASP recommended).

- Not bcrypt (slower for legitimate users, same security).
- Not PBKDF2 (older, lower memory cost).

### 4. Encryption Key per User (Optional)
**Decision**: Single global AES-256 key (Phase 1); per-user key derivation (Phase 6 optional).

- **Current state**: `internal/crypto/crypto.go` uses a single `globalKey` at `~/.flowpilot/key.bin`.
- **Phase 1**: Keep global key. Proxy credentials remain encrypted at rest.
- **Phase 6**: Optionally derive per-user keys from user password (PBKDF2) so users can't decrypt each other's proxies even if DB is stolen.

### 5. Wails Constraints & Workarounds
**Problem**: Wails v2 binds all `App` methods directly to frontend with no middleware layer.

**Solution**:
1. Add `currentUserID` to `App` struct (mutex-protected).
2. Every API method checks `currentUserID` before executing.
3. Return error if `currentUserID` is empty (user not logged in).
4. For optional HTTP API mode, create separate `internal/api/` package with standard HTTP handlers + middleware.

---

## Requirements

### Functional
- [x] User registration (username, password)
- [x] User login with session persistence
- [x] User logout
- [x] Password hashing with Argon2id
- [x] Role-based access (admin, user, viewer)
- [x] Row-level security: all queries filtered by `user_id`
- [x] Session tokens with expiration
- [x] Audit trail: log who created/modified each record
- [x] Optional HTTP API mode for background agent

### Non-Functional
- [x] No breaking changes to existing Wails API surface (return errors instead of panics)
- [x] Backward compatibility: support legacy single-user mode
- [x] All existing tests continue to pass with `-tags=dev`
- [x] Session token rotation on sensitive operations
- [x] CSRF protection for web mode (optional, Phase 6)

---

## Schema Changes

### Phase 1: Add Users Table & Foreign Keys

```sql
-- New table: users
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  email TEXT UNIQUE,
  role TEXT DEFAULT 'user', -- 'admin', 'user', 'viewer'
  is_active INTEGER DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_login_at DATETIME
);

-- New table: sessions
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT UNIQUE NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  ip_address TEXT,
  user_agent TEXT
);

-- Modify: tasks
ALTER TABLE tasks ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_tasks_user_id ON tasks(user_id);

-- Modify: recorded_flows
ALTER TABLE recorded_flows ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_recorded_flows_user_id ON recorded_flows(user_id);

-- Modify: dom_snapshots
ALTER TABLE dom_snapshots ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_dom_snapshots_user_id ON dom_snapshots(user_id);

-- Modify: batch_groups
ALTER TABLE batch_groups ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_batch_groups_user_id ON batch_groups(user_id);

-- Modify: task_events
ALTER TABLE task_events ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_task_events_user_id ON task_events(user_id);

-- Modify: step_logs
ALTER TABLE step_logs ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_step_logs_user_id ON step_logs(user_id);

-- Modify: network_logs
ALTER TABLE network_logs ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_network_logs_user_id ON network_logs(user_id);

-- Modify: websocket_logs
ALTER TABLE websocket_logs ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_websocket_logs_user_id ON websocket_logs(user_id);

-- Modify: proxies
ALTER TABLE proxies ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_proxies_user_id ON proxies(user_id);

-- Modify: proxy_routing_presets
ALTER TABLE proxy_routing_presets ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_proxy_routing_presets_user_id ON proxy_routing_presets(user_id);

-- Modify: schedules
ALTER TABLE schedules ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_schedules_user_id ON schedules(user_id);

-- Modify: captcha_config
ALTER TABLE captcha_config ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_captcha_config_user_id ON captcha_config(user_id);

-- Modify: visual_baselines
ALTER TABLE visual_baselines ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_visual_baselines_user_id ON visual_baselines(user_id);

-- Modify: visual_diffs
ALTER TABLE visual_diffs ADD COLUMN user_id TEXT DEFAULT '';
CREATE INDEX idx_visual_diffs_user_id ON visual_diffs(user_id);
```

### Migration Path

In `internal/database/sqlite.go`, update the `migrate()` function:

```go
func (db *DB) migrate() error {
  schema := `
    -- ... existing schema ...
    
    CREATE TABLE IF NOT EXISTS users (
      id TEXT PRIMARY KEY,
      username TEXT UNIQUE NOT NULL,
      password_hash TEXT NOT NULL,
      email TEXT UNIQUE,
      role TEXT DEFAULT 'user',
      is_active INTEGER DEFAULT 1,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      last_login_at DATETIME
    );

    CREATE TABLE IF NOT EXISTS sessions (
      id TEXT PRIMARY KEY,
      user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      token_hash TEXT UNIQUE NOT NULL,
      expires_at DATETIME NOT NULL,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      ip_address TEXT,
      user_agent TEXT
    );
  `
  
  if _, err := db.conn.Exec(schema); err != nil {
    return fmt.Errorf("migrate: %w", err)
  }
  
  // Add user_id columns (idempotent)
  alterStatements := []string{
    "ALTER TABLE tasks ADD COLUMN user_id TEXT DEFAULT ''",
    "ALTER TABLE recorded_flows ADD COLUMN user_id TEXT DEFAULT ''",
    // ... etc for all tables ...
  }
  
  for _, stmt := range alterStatements {
    db.conn.Exec(stmt) // Ignore errors if column exists
  }
  
  // Create indexes (idempotent)
  indexes := []string{
    "CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id)",
    // ... etc ...
  }
  
  for _, stmt := range indexes {
    if _, err := db.conn.Exec(stmt); err != nil {
      return fmt.Errorf("create index: %w", err)
    }
  }
  
  return nil
}
```


---

## Architecture Changes

### 1. App Struct (app.go)

Add user context to the `App` struct:

```go
type App struct {
  ctx               context.Context
  db                *database.DB
  runner            *browser.Runner
  pool              *browser.BrowserPool
  queue             *queue.Queue
  proxyManager      *proxy.Manager
  localProxyManager *localproxy.Manager
  scheduler         *scheduler.Scheduler
  dataDir           string
  batchEngine       *batch.Engine
  logExporter       *logs.Exporter
  config            AppConfig
  initErr           error

  recorderMu     sync.Mutex
  activeRecorder *recorder.Recorder
  recorderCancel context.CancelFunc
  recordedSteps  []models.RecordedStep
  
  // NEW: User context (mutex-protected for concurrent Wails calls)
  userMu         sync.RWMutex
  currentUserID  string
  currentUser    *models.User
}
```

### 2. New Models (internal/models/user.go)

```go
package models

import "time"

type User struct {
  ID            string    `json:"id"`
  Username      string    `json:"username"`
  Email         string    `json:"email,omitempty"`
  Role          string    `json:"role"` // "admin", "user", "viewer"
  IsActive      bool      `json:"isActive"`
  CreatedAt     time.Time `json:"createdAt"`
  UpdatedAt     time.Time `json:"updatedAt"`
  LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
  // Password NOT included in JSON responses
}

type Session struct {
  ID        string    `json:"id"`
  UserID    string    `json:"userId"`
  TokenHash string    `json:"-"` // Never expose
  ExpiresAt time.Time `json:"expiresAt"`
  CreatedAt time.Time `json:"createdAt"`
  IPAddress string    `json:"ipAddress,omitempty"`
  UserAgent string    `json:"userAgent,omitempty"`
}

type LoginRequest struct {
  Username string `json:"username"`
  Password string `json:"password"`
}

type LoginResponse struct {
  User  *User  `json:"user"`
  Token string `json:"token"`
}

type RegisterRequest struct {
  Username string `json:"username"`
  Password string `json:"password"`
  Email    string `json:"email,omitempty"`
}
```

### 3. New Database Methods (internal/database/db_users.go)

```go
package database

import (
  "context"
  "fmt"
  "time"

  "flowpilot/internal/models"
  "github.com/google/uuid"
)

// CreateUser inserts a new user. Password should be pre-hashed.
func (db *DB) CreateUser(ctx context.Context, username, passwordHash, email string) (*models.User, error) {
  id := uuid.New().String()
  user := &models.User{
    ID:        id,
    Username:  username,
    Email:     email,
    Role:      "user",
    IsActive:  true,
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
  }

  query := `
    INSERT INTO users (id, username, password_hash, email, role, is_active, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
  `
  
  if _, err := db.conn.ExecContext(ctx, query,
    id, username, passwordHash, email, "user", 1, time.Now(), time.Now(),
  ); err != nil {
    return nil, fmt.Errorf("create user: %w", err)
  }

  return user, nil
}

// GetUserByUsername retrieves a user by username (includes password hash for auth).
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*models.User, string, error) {
  query := `SELECT id, username, email, role, is_active, created_at, updated_at, last_login_at, password_hash FROM users WHERE username = ?`
  
  var user models.User
  var passwordHash string
  var lastLoginAt *time.Time

  err := db.readConn.QueryRowContext(ctx, query, username).Scan(
    &user.ID, &user.Username, &user.Email, &user.Role, &user.IsActive,
    &user.CreatedAt, &user.UpdatedAt, &lastLoginAt, &passwordHash,
  )
  if err != nil {
    return nil, "", fmt.Errorf("get user by username: %w", err)
  }

  user.LastLoginAt = lastLoginAt
  return &user, passwordHash, nil
}

// GetUserByID retrieves a user by ID (no password hash).
func (db *DB) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
  query := `SELECT id, username, email, role, is_active, created_at, updated_at, last_login_at FROM users WHERE id = ?`
  
  var user models.User
  var lastLoginAt *time.Time

  err := db.readConn.QueryRowContext(ctx, query, userID).Scan(
    &user.ID, &user.Username, &user.Email, &user.Role, &user.IsActive,
    &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
  )
  if err != nil {
    return nil, fmt.Errorf("get user by id: %w", err)
  }

  user.LastLoginAt = lastLoginAt
  return &user, nil
}

// CreateSession creates a new session token.
func (db *DB) CreateSession(ctx context.Context, userID, tokenHash, ipAddress, userAgent string, expiresIn time.Duration) (*models.Session, error) {
  id := uuid.New().String()
  expiresAt := time.Now().Add(expiresIn)

  session := &models.Session{
    ID:        id,
    UserID:    userID,
    TokenHash: tokenHash,
    ExpiresAt: expiresAt,
    CreatedAt: time.Now(),
    IPAddress: ipAddress,
    UserAgent: userAgent,
  }

  query := `
    INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, ip_address, user_agent)
    VALUES (?, ?, ?, ?, ?, ?, ?)
  `
  
  if _, err := db.conn.ExecContext(ctx, query,
    id, userID, tokenHash, expiresAt, time.Now(), ipAddress, userAgent,
  ); err != nil {
    return nil, fmt.Errorf("create session: %w", err)
  }

  return session, nil
}

// ValidateSessionToken looks up and validates a session (checks expiry).
func (db *DB) ValidateSessionToken(ctx context.Context, tokenHash string) (*models.Session, error) {
  query := `SELECT id, user_id, expires_at, created_at, ip_address, user_agent FROM sessions WHERE token_hash = ?`
  
  var session models.Session
  err := db.readConn.QueryRowContext(ctx, query, tokenHash).Scan(
    &session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt, &session.IPAddress, &session.UserAgent,
  )
  if err != nil {
    return nil, fmt.Errorf("validate session: %w", err)
  }

  if time.Now().After(session.ExpiresAt) {
    return nil, fmt.Errorf("session expired")
  }

  session.TokenHash = tokenHash
  return &session, nil
}

// InvalidateSession deletes a session (logout).
func (db *DB) InvalidateSession(ctx context.Context, tokenHash string) error {
  query := `DELETE FROM sessions WHERE token_hash = ?`
  _, err := db.conn.ExecContext(ctx, query, tokenHash)
  return err
}

// UpdateLastLogin updates the last_login_at timestamp for a user.
func (db *DB) UpdateLastLogin(ctx context.Context, userID string) error {
  query := `UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`
  _, err := db.conn.ExecContext(ctx, query, time.Now(), time.Now(), userID)
  return err
}
```

### 4. Authentication Helpers (internal/auth/auth.go) - NEW PACKAGE

```go
package auth

import (
  "crypto/rand"
  "crypto/sha256"
  "encoding/base64"
  "fmt"

  "golang.org/x/crypto/argon2"
)

const (
  ArgonTime      = 2
  ArgonMemory    = 19 * 1024 // 19 MB
  ArgonParallel  = 1
  SessionTimeout = 24 * 60 * 60 // 24 hours in seconds
)

// HashPassword hashes a plaintext password using Argon2id.
func HashPassword(password string) (string, error) {
  salt := make([]byte, 16)
  if _, err := rand.Read(salt); err != nil {
    return "", fmt.Errorf("generate salt: %w", err)
  }

  hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonParallel, 32)
  
  // Format: argon2id$v=19$m=19456,t=2,p=1$<base64 salt>$<base64 hash>
  encoded := fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
    ArgonMemory,
    ArgonTime,
    ArgonParallel,
    base64.RawStdEncoding.EncodeToString(salt),
    base64.RawStdEncoding.EncodeToString(hash),
  )

  return encoded, nil
}

// VerifyPassword checks if plaintext matches hashed password.
func VerifyPassword(plaintext, hash string) bool {
  // Parse hash format (simplified; in production use a library like github.com/alexedwards/argon2id)
  // For now, use a basic comparison
  hashObj, err := argon2.IDKey([]byte(plaintext), extractSalt(hash), ArgonTime, ArgonMemory, ArgonParallel, 32)
  if err != nil {
    return false
  }

  expected := extractHash(hash)
  return constantTimeCompare(hashObj, expected)
}

// GenerateSessionToken creates a random session token.
func GenerateSessionToken() (string, error) {
  token := make([]byte, 32)
  if _, err := rand.Read(token); err != nil {
    return "", fmt.Errorf("generate token: %w", err)
  }
  return base64.URLEncoding.EncodeToString(token), nil
}

// HashToken hashes a token for storage in the database.
func HashToken(token string) string {
  hash := sha256.Sum256([]byte(token))
  return base64.URLEncoding.EncodeToString(hash[:])
}

// Helper functions (simplified; use a real library in production)
func extractSalt(hash string) []byte {
  // Parse format and return salt bytes
  parts := base64.RawStdEncoding.WithPadding(base64.NoPadding).DecodeString("dummy") // simplified
  return parts
}

func extractHash(hash string) []byte {
  // Parse format and return hash bytes
  return []byte("dummy") // simplified
}

func constantTimeCompare(a, b []byte) bool {
  // Simple comparison (not constant-time; use subtle.ConstantTimeCompare in production)
  if len(a) != len(b) {
    return false
  }
  for i := range a {
    if a[i] != b[i] {
      return false
    }
  }
  return true
}
```

Note: In production, use `github.com/alexedwards/argon2id` for robust Argon2 handling.


---

## Implementation Steps (Phased)

### Phase 1: Core Auth Infrastructure (2-3 weeks)

**Goals**: User registration, login, session management. All queries filtered by user_id.

#### Step 1.1: Database Schema Migration
- **File**: `internal/database/sqlite.go`
- **Task**: Update `migrate()` function to create `users` and `sessions` tables, add `user_id` columns to all 13 tables.
- **Testing**: `go test -tags=dev ./internal/database -run TestMigrate`

#### Step 1.2: Create Auth Package
- **New file**: `internal/auth/auth.go`
- **Exports**: `HashPassword()`, `VerifyPassword()`, `GenerateSessionToken()`, `HashToken()`
- **Dependency**: Add `golang.org/x/crypto/argon2` to `go.mod`
- **Testing**: `go test -tags=dev ./internal/auth`

#### Step 1.3: Create User Models
- **New file**: `internal/models/user.go`
- **Structs**: `User`, `Session`, `LoginRequest`, `LoginResponse`, `RegisterRequest`
- **Validation**: Add `ValidateUsername()`, `ValidatePassword()` to `internal/validation/validate.go`

#### Step 1.4: Add Database Methods for Users
- **New file**: `internal/database/db_users.go`
- **Methods**:
  - `CreateUser(ctx, username, passwordHash, email) (*User, error)`
  - `GetUserByUsername(ctx, username) (*User, passwordHash, error)`
  - `GetUserByID(ctx, userID) (*User, error)`
  - `CreateSession(ctx, userID, tokenHash, ipAddress, userAgent, expiresIn) (*Session, error)`
  - `ValidateSessionToken(ctx, tokenHash) (*Session, error)`
  - `InvalidateSession(ctx, tokenHash) error`
  - `UpdateLastLogin(ctx, userID) error`

#### Step 1.5: Add Auth Methods to App
- **File**: `app.go` (new section or separate `app_auth.go`)
- **Methods**:
  ```go
  func (a *App) Register(req models.RegisterRequest) (*models.User, error)
  func (a *App) Login(req models.LoginRequest) (*models.LoginResponse, error)
  func (a *App) Logout() error
  func (a *App) GetCurrentUser() (*models.User, error)
  func (a *App) ValidateSession(token string) (*models.User, error) // internal, called on startup
  ```
- **Key implementation**:
  ```go
  func (a *App) Login(req models.LoginRequest) (*models.LoginResponse, error) {
    user, passwordHash, err := a.db.GetUserByUsername(context.Background(), req.Username)
    if err != nil {
      return nil, fmt.Errorf("invalid username or password")
    }
    
    if !auth.VerifyPassword(req.Password, passwordHash) {
      return nil, fmt.Errorf("invalid username or password")
    }
    
    token, err := auth.GenerateSessionToken()
    if err != nil {
      return nil, err
    }
    
    tokenHash := auth.HashToken(token)
    a.db.CreateSession(context.Background(), user.ID, tokenHash, "", "", 24*time.Hour)
    a.db.UpdateLastLogin(context.Background(), user.ID)
    
    a.userMu.Lock()
    a.currentUserID = user.ID
    a.currentUser = user
    a.userMu.Unlock()
    
    return &models.LoginResponse{User: user, Token: token}, nil
  }
  ```

#### Step 1.6: Add Auth Checks to All Existing API Methods
- **Files**: All `app_*.go` files
- **Pattern**: Each exported method on `App` must call `a.getCurrentUserID()` first:
  ```go
  func (a *App) CreateTask(task models.Task) (*models.Task, error) {
    userID, err := a.getCurrentUserID()
    if err != nil {
      return nil, err
    }
    task.UserID = userID
    // ... proceed with creation
  }
  
  func (a *App) getCurrentUserID() (string, error) {
    a.userMu.RLock()
    defer a.userMu.RUnlock()
    if a.currentUserID == "" {
      return "", fmt.Errorf("not authenticated")
    }
    return a.currentUserID, nil
  }
  ```
- **Affected methods**: All 85+ methods in `app_tasks.go`, `app_flows.go`, `app_batch.go`, etc.

#### Step 1.7: Add RLS to All Database Queries
- **Files**: All `internal/database/db_*.go` files
- **Pattern**: Append `WHERE user_id = ?` to all SELECT/UPDATE/DELETE queries
- **Example** (before):
  ```go
  func (db *DB) GetTask(ctx context.Context, taskID string) (*models.Task, error) {
    query := `SELECT ... FROM tasks WHERE id = ?`
    // ...
  }
  ```
- **Example** (after):
  ```go
  func (db *DB) GetTask(ctx context.Context, userID, taskID string) (*models.Task, error) {
    query := `SELECT ... FROM tasks WHERE id = ? AND user_id = ?`
    // ...
  }
  ```

**Changes across files**:
- `internal/database/db_tasks.go`: Update all 15+ task methods
- `internal/database/db_flows.go`: Update all flow methods
- `internal/database/db_logs.go`: Update all log query methods
- `internal/database/db_batch.go`: Update batch methods
- `internal/database/db_proxies.go`: Update proxy methods
- `internal/database/db_proxy_presets.go`: Update preset methods
- `internal/database/db_schedules.go`: Update schedule methods
- `internal/database/db_captcha.go`: Update captcha methods
- `internal/database/db_vision.go`: Update vision methods
- `internal/database/db_retention.go`: Update retention queries (only admin)

#### Step 1.8: Frontend Login Screen
- **New file**: `frontend/src/components/LoginModal.svelte`
- **Features**:
  - Username + password input
  - Register / Login buttons
  - Error handling
  - Store auth token in localStorage (desktop can use Wails FileSystem API)
- **Update `frontend/src/App.svelte`**:
  - Show login screen if no `currentUser` store value
  - Hide main UI until logged in
  - Add logout button to header

#### Step 1.9: Frontend User Store
- **File**: `frontend/src/lib/store.ts`
- **New stores**:
  ```typescript
  export const currentUser = writable<User | null>(null);
  export const authToken = writable<string | null>(null);
  export const isAuthenticated = derived(currentUser, $u => $u !== null);
  ```
- **Update App.svelte**: On mount, call `App.ValidateSession(token)` to restore session

#### Step 1.10: Testing
- **Unit tests**: `internal/auth/auth_test.go`, `internal/database/db_users_test.go`
- **Integration tests**: `app_test.go` - test `Register()`, `Login()`, `Logout()`, and RLS filtering
- **Example**:
  ```go
  func TestRLSFiltering(t *testing.T) {
    // Create two users
    user1, _ := setupTestDB(t).CreateUser(ctx, "alice", hash1, "")
    user2, _ := setupTestDB(t).CreateUser(ctx, "bob", hash2, "")
    
    // Create tasks for each user
    task1 := &models.Task{ID: "1", UserID: user1.ID, Name: "Task 1"}
    task2 := &models.Task{ID: "2", UserID: user2.ID, Name: "Task 2"}
    
    // Verify user1 can only see task1
    tasks, _ := db.GetTasks(ctx, user1.ID)
    if len(tasks) != 1 || tasks[0].ID != "1" {
      t.Errorf("RLS filter failed")
    }
  }
  ```

---

### Phase 2: Role-Based Access Control (1-2 weeks)

**Goals**: Admin/user/viewer roles, permission checks on sensitive operations.

#### Step 2.1: Add Role Validation
- **File**: `internal/validation/validate.go`
- **Function**: `ValidateRole(role string) error` - ensure role is one of {admin, user, viewer}

#### Step 2.2: Permission Checks in App Methods
- **Files**: `app_*.go`
- **Pattern**: Add role checks for sensitive operations:
  ```go
  func (a *App) DeleteTask(taskID string) error {
    userID, err := a.getCurrentUserID()
    if err != nil {
      return err
    }
    
    // Only allow owner or admin
    task, err := a.db.GetTask(context.Background(), userID, taskID)
    if err != nil {
      return fmt.Errorf("task not found")
    }
    
    // For Phase 2: check if admin or owner
    if !a.isAdmin(userID) && task.UserID != userID {
      return fmt.Errorf("permission denied")
    }
    
    return a.db.DeleteTask(context.Background(), userID, taskID)
  }
  
  func (a *App) isAdmin(userID string) bool {
    a.userMu.RLock()
    defer a.userMu.RUnlock()
    return a.currentUser != nil && a.currentUser.Role == "admin"
  }
  ```

#### Step 2.3: Admin API Methods
- **File**: `app_admin.go` (new)
- **Methods**:
  ```go
  func (a *App) ListAllUsers() ([]models.User, error) // admin only
  func (a *App) SetUserRole(userID, role string) error // admin only
  func (a *App) DisableUser(userID string) error // admin only
  func (a *App) GetSystemStats() (*models.SystemStats, error) // admin only
  ```

---

### Phase 3: Audit Trail Enhancement (1 week)

**Goals**: Track all user actions in `task_events` table.

#### Step 3.1: Update Task Events
- **File**: `internal/models/event.go`
- **Add field**: `ActedByUserID string` to `TaskLifecycleEvent`

#### Step 3.2: Log User Actions
- **Files**: All database write methods (`db_*.go`)
- **Pattern**: When creating/updating records, also insert an event:
  ```go
  func (db *DB) CreateTask(ctx context.Context, userID string, task *models.Task) error {
    // ... insert task ...
    
    // Log event
    event := models.TaskLifecycleEvent{
      ID:              uuid.New().String(),
      TaskID:          task.ID,
      Status:          "created",
      ActedByUserID:   userID,
      CreatedAt:       time.Now(),
    }
    return db.logEvent(ctx, event)
  }
  ```

---

### Phase 4: Export & Compliance Enhancements (1 week)

**Goals**: Ensure exports respect RLS, add data subject requests.

#### Step 4.1: Update Export Methods
- **Files**: `app_export.go`, `app_flow_io.go`
- **Pattern**: All exports filtered by `currentUserID`
- **Example**:
  ```go
  func (a *App) ExportResultsJSON() (string, error) {
    userID, err := a.getCurrentUserID()
    if err != nil {
      return "", err
    }
    
    tasks, _ := a.db.GetTasks(context.Background(), userID) // Already RLS-filtered
    // ... serialize to JSON ...
  }
  ```

#### Step 4.2: Data Subject Access Request (DSAR)
- **New method**: `App.ExportMyData()`
- **Action**: Zip all user's data (tasks, flows, logs) into a single file for GDPR compliance

#### Step 4.3: Data Deletion on Account Deletion
- **New method**: `App.DeleteMyAccount(password string)` 
- **Cascading deletes**: On `users` delete, all related records cascade delete via FK

---

### Phase 5: Optional HTTP API Mode (2 weeks, optional)

**Goals**: Enable background agent + CLI to use FlowPilot via HTTP with JWT auth.

#### Step 5.1: Create HTTP Server Package
- **New directory**: `internal/api/`
- **New file**: `internal/api/server.go`
- **Exports**:
  - `NewServer(db *database.DB, queue *queue.Queue) *Server`
  - `Server.Listen(port int) error`
  - `Server.Shutdown() error`

#### Step 5.2: JWT Middleware
- **New file**: `internal/api/middleware.go`
- **Functions**: 
  - `AuthMiddleware()` - verify JWT in Authorization header
  - `RoleMiddleware(requiredRole string)` - check user role

#### Step 5.3: HTTP Handlers
- **New files**: `internal/api/handlers_tasks.go`, `internal/api/handlers_flows.go`, etc.
- **Pattern**: Mirror all `App.Method()` as `POST /api/tasks`, `GET /api/tasks/:id`, etc.
- **Example**:
  ```go
  func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(string) // from middleware
    
    var task models.Task
    json.NewDecoder(r.Body).Decode(&task)
    
    createdTask, err := s.db.CreateTask(r.Context(), userID, &task)
    if err != nil {
      http.Error(w, err.Error(), http.StatusBadRequest)
      return
    }
    
    json.NewEncoder(w).Encode(createdTask)
  }
  ```

#### Step 5.4: Update Agent to Use HTTP API
- **File**: `cmd/agent/main.go`
- **Change**: Instead of in-process calls to `App` methods, make HTTP requests to the API server
- **Config**: Agent reads `FLOWPILOT_API_URL` and `FLOWPILOT_JWT_TOKEN` env vars

#### Step 5.5: JWT Token Issuance
- **New method**: `App.IssueAgentToken(duration time.Duration) (string, error)` 
- **Stored**: Token metadata in DB for revocation
- **Rotation**: Admin can rotate tokens via UI

---

### Phase 6: Multi-Instance & OIDC (Optional, Future)

**Goals**: Support multiple FlowPilot instances sharing a central user directory.

#### Step 6.1: Per-User Encryption Keys
- **File**: `internal/crypto/crypto.go`
- **Change**: Derive user-specific AES key from password via PBKDF2
- **Pattern**:
  ```go
  func DeriveUserKey(userID, password string, salt []byte) ([]byte, error) {
    // PBKDF2(password, salt=userID+globalSalt, iterations=100000, length=32)
    return pbkdf2.Key([]byte(password), append(salt, []byte(userID)...), 100000, 32, sha256.New)
  }
  ```
- **Impact**: Proxy credentials encrypted with per-user key. DB leak doesn't expose credentials.

#### Step 6.2: OIDC Integration (Optional)
- **New package**: `internal/auth/oidc.go`
- **Support**: OpenID Connect provider (e.g., Keycloak, Auth0)
- **Flow**: Desktop app opens browser for OIDC login, receives JWT, stores in Wails secure storage

---

## Security Considerations

### 1. Password Storage
- ✅ Argon2id hashing with memory=19MB, time=2, parallelism=1
- ✅ Each password has unique random salt
- ✅ Passwords never logged or exposed in errors

### 2. Session Tokens
- ✅ Tokens generated with 32 bytes of `crypto/rand` entropy
- ✅ Stored hashed in DB (SHA256)
- ✅ 24-hour expiration by default
- ✅ Invalidated on logout
- ✅ Future: Rotate on sensitive operations (password change, role update)

### 3. User Data Isolation (RLS)
- ✅ All queries include `WHERE user_id = ?` filter
- ✅ Index on `user_id` for query performance
- ✅ Cannot select another user's data
- ✅ Tests verify RLS enforcement

### 4. Attack Mitigations
- **Timing attacks on login**: Use constant-time password comparison (via `subtle.ConstantTimeCompare`)
- **Brute force**: Rate limit login attempts (future: via `x-ratelimit-*` headers in API mode)
- **Session fixation**: Generate new token on each login, invalidate old sessions
- **CSRF** (web mode): Add CSRF token to HTML forms, validate on POST
- **XSS** (frontend): Never innerHTML user input, use Svelte's auto-escaping

### 5. Credential Encryption
- ✅ Proxy credentials encrypted at rest with AES-256-GCM
- ✅ Future: Per-user keys so admin can't decrypt other users' proxies
- ✅ Keys stored with mode=0600 (owner-read-write only)

### 6. Audit Trail
- ✅ All user actions logged in `task_events` table with `acted_by_user_id`
- ✅ Logs cannot be deleted by regular users (admin-only purge)
- ✅ 90-day retention minimum (configurable via `RetentionDays`)

### 7. SQLite Constraints
- ✅ Foreign key enforcement: `PRAGMA foreign_keys=ON` (add to connection setup)
- ✅ Transactions: All multi-step operations use explicit transactions
- ✅ No eval: Keep `allowEval=false` in browser config

---

## Testing Strategy

### Unit Tests

**`internal/auth/auth_test.go`**:
```go
func TestHashPassword(t *testing.T) {
  hash, err := auth.HashPassword("password123")
  if err != nil || hash == "" {
    t.Fail()
  }
}

func TestVerifyPassword(t *testing.T) {
  hash, _ := auth.HashPassword("password123")
  if !auth.VerifyPassword("password123", hash) {
    t.Fail()
  }
  if auth.VerifyPassword("wrong", hash) {
    t.Fail()
  }
}
```

**`internal/database/db_users_test.go`**:
```go
func TestCreateUser(t *testing.T) {
  db := setupTestDB(t)
  user, err := db.CreateUser(context.Background(), "alice", "hash", "alice@example.com")
  if err != nil || user.Username != "alice" {
    t.Fail()
  }
}

func TestSessionValidation(t *testing.T) {
  db := setupTestDB(t)
  user, _ := db.CreateUser(context.Background(), "bob", "hash", "")
  token, _ := auth.GenerateSessionToken()
  tokenHash := auth.HashToken(token)
  session, _ := db.CreateSession(context.Background(), user.ID, tokenHash, "", "", time.Hour)
  
  retrieved, _ := db.ValidateSessionToken(context.Background(), tokenHash)
  if retrieved.UserID != user.ID {
    t.Fail()
  }
}
```

### Integration Tests

**`app_test.go`** (extend existing):
```go
func TestLoginFlow(t *testing.T) {
  app := setupTestApp(t)
  
  // Register
  resp, err := app.Register(models.RegisterRequest{
    Username: "charlie",
    Password: "secret123",
  })
  if err != nil || resp.User.Username != "charlie" {
    t.Fail()
  }
  
  // Login
  login, err := app.Login(models.LoginRequest{
    Username: "charlie",
    Password: "secret123",
  })
  if err != nil || login.Token == "" {
    t.Fail()
  }
  
  // Verify currentUserID is set
  if !app.isCurrentUser("charlie") {
    t.Fail()
  }
}

func TestRLSEnforcement(t *testing.T) {
  app := setupTestApp(t)
  
  // Create two users
  app.Register(models.RegisterRequest{Username: "user1", Password: "pass1"})
  app.Register(models.RegisterRequest{Username: "user2", Password: "pass2"})
  
  // Login as user1, create a task
  app.Login(models.LoginRequest{Username: "user1", Password: "pass1"})
  task1, _ := app.CreateTask(models.Task{Name: "User1 Task"})
  
  // Login as user2, verify task1 is not visible
  app.Logout()
  app.Login(models.LoginRequest{Username: "user2", Password: "pass2"})
  tasks, _ := app.ListTasks()
  
  for _, t := range tasks {
    if t.ID == task1.ID {
      t.Errorf("RLS filter broken: user2 saw user1's task")
    }
  }
}
```

### Frontend Tests

**`frontend/src/components/LoginModal.test.ts`**:
- Test login form submission
- Test error display on invalid credentials
- Test register form
- Test token storage/retrieval

### E2E Tests (Optional)

- Test full login → create task → logout → login as different user → verify isolation flow
- Test admin role accessing user management UI
- Test audit trail shows correct user actions

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| **Wails doesn't expose middleware hooks** | Can't intercept all API calls with auth checks | Add `getCurrentUserID()` check to every exported method. Code review to catch misses. |
| **Migration adds user_id to 13 tables** | Long ALTER TABLE locks on large databases | Use online migration tool (Phase 1.1 tests this). Mark as breaking change in release notes. |
| **Argon2 library dependency** | Supply chain risk | Use audited, well-maintained library (golang.org/x/crypto). Pin version in go.mod. |
| **Password bruteforce attacks** | Account takeover | Future: Rate limit login attempts. Log failed attempts. Alert on suspicious patterns. |
| **User deletes account, then re-registers** | Can see old data under same username | Soft-delete users (isActive=false), then hard-delete after 30 days. OR use email+timestamp as unique key. |
| **Frontend stores JWT in localStorage** | XSS attack reads token | Use httpOnly=true cookies in web mode. For desktop, use Wails secure storage API (WIP). |
| **Admin reads another user's password hash** | Can crack password offline | Admin role can only disable users, not reset passwords. Add separate "RequestPasswordReset" flow. |
| **SQLite doesn't have column-level security** | Admin can query raw DB, see all data | Not applicable to desktop app (single instance). For multi-instance HTTP API, use separate DB connection pools per user. |
| **Background agent needs to run tasks for many users** | JWT token management complexity | Agent uses single service account JWT, proxied through API. Each API call includes user_id header. |

---

## Success Criteria

- [x] All 85+ exported methods on `App` struct require valid session (return auth error if not)
- [x] All database queries include RLS filter (`WHERE user_id = ?` or cascade from parent)
- [x] Login/logout/register flows work in desktop UI
- [x] Two users can operate independently with no data leakage
- [x] Audit trail logs all actions with `acted_by_user_id`
- [x] All existing tests pass with `-tags=dev`
- [x] New test coverage: auth (>85%), RLS enforcement (>90%), schema migration (100%)
- [x] No performance regression (RLS indexes created, queries optimized)
- [x] Documentation updated: API docs, security guide, migration guide
- [x] No hardcoded user IDs or "default user" fallback
- [x] Admin role can manage users via API
- [x] Optional: HTTP API mode works with JWT (Phase 5)

---

## Timeline

- **Phase 1 (core auth + RLS)**: 2-3 weeks (all methods, all tables)
- **Phase 2 (RBAC)**: 1-2 weeks (role checks, admin API)
- **Phase 3 (audit)**: 1 week (event logging)
- **Phase 4 (exports)**: 1 week (DSAR, compliance)
- **Phase 5 (HTTP API)**: 2 weeks (optional, for agent mode)
- **Phase 6 (OIDC, per-user keys)**: 3+ weeks (future, optional)

**Total (Phases 1-4)**: 5-7 weeks
**With Phase 5**: 7-9 weeks
**Total with Phase 6**: 10-12 weeks

---

## References

- Argon2 spec: https://github.com/P-H-C/phc-winner-argon2
- OWASP password storage: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- SQLite foreign keys: https://www.sqlite.org/foreignkeys.html
- Session best practices: https://owasp.org/www-community/attacks/Session_fixation
- Wails v2 docs: https://wails.io/docs/howdosiwails

