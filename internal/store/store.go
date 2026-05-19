package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

// Store is the SQLite-backed multi-tenant data layer.
type Store struct {
	db *sql.DB
	c  *Crypto
}

// Open creates (or opens) the database at path and runs migrations.
func Open(path string, c *Crypto) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		return nil, err
	}
	s := &Store{db: db, c: c}
	if err := s.migrate(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	const baseSchema = `
CREATE TABLE IF NOT EXISTS users (
  user_id          TEXT PRIMARY KEY,
  user_name        TEXT,
  user_number      TEXT,
  user_section     TEXT,
  user_class       TEXT,
  token_enc        BLOB NOT NULL,
  token_exp        INTEGER NOT NULL,
  auto_sign        INTEGER NOT NULL DEFAULT 1,
  is_disabled      INTEGER NOT NULL DEFAULT 0,
  invite_code      TEXT,
  lat              REAL DEFAULT 0,
  lng              REAL DEFAULT 0,
  address          TEXT DEFAULT '',
  city             TEXT DEFAULT '',
  road             TEXT DEFAULT '',
  poi              TEXT DEFAULT '',
  device_model     TEXT DEFAULT 'iPhone',
  device_system    TEXT DEFAULT 'iOS',
  trigger_minute   INTEGER NOT NULL DEFAULT 2,
  jitter_sec       INTEGER NOT NULL DEFAULT 180,
  retry_count      INTEGER NOT NULL DEFAULT 3,
  retry_gap_min    INTEGER NOT NULL DEFAULT 5,
  saved_locations  TEXT NOT NULL DEFAULT '[]',
  pin_hash         BLOB,
  user_avatar_url  TEXT NOT NULL DEFAULT '',
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS invite_codes (
  code           TEXT PRIMARY KEY,
  bound_user_id  TEXT,
  bound_at       INTEGER,
  note           TEXT NOT NULL DEFAULT '',
  disabled       INTEGER NOT NULL DEFAULT 0,
  created_at     INTEGER NOT NULL,
  created_by     TEXT NOT NULL DEFAULT 'admin'
);
CREATE INDEX IF NOT EXISTS idx_codes_bound ON invite_codes(bound_user_id);

CREATE TABLE IF NOT EXISTS sign_records (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      TEXT NOT NULL,
  rule_id      INTEGER NOT NULL,
  status       TEXT NOT NULL,
  message      TEXT,
  occurred_at  INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_records_user_time ON sign_records(user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_time ON sign_records(occurred_at DESC);

CREATE TABLE IF NOT EXISTS web_sessions (
  session_id   TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL,
  is_admin     INTEGER NOT NULL DEFAULT 0,
  expires_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON web_sessions(user_id);

CREATE TABLE IF NOT EXISTS site_access_codes (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  code_hash     TEXT NOT NULL UNIQUE,
  created_by    TEXT NOT NULL DEFAULT 'admin',
  created_at    INTEGER NOT NULL,
  expires_at    INTEGER NOT NULL,
  used_at       INTEGER,
  used_by_ip    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_site_access_codes_expires ON site_access_codes(expires_at);

CREATE TABLE IF NOT EXISTS site_gate_passes (
  pass_id     TEXT PRIMARY KEY,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_site_gate_passes_expires ON site_gate_passes(expires_at);

CREATE TABLE IF NOT EXISTS system_config (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS dorm_locations (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  name                TEXT NOT NULL UNIQUE,
  latitude            REAL NOT NULL,
  longitude           REAL NOT NULL,
  address             TEXT DEFAULT '',
  city                TEXT DEFAULT '',
  road                TEXT DEFAULT '',
  poi                 TEXT DEFAULT '',
  note                TEXT DEFAULT '',
  send_address_fields INTEGER NOT NULL DEFAULT 0,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);
`
	if _, err := s.db.ExecContext(ctx, baseSchema); err != nil {
		return err
	}

	// Idempotent column additions for forward upgrades.
	upgrades := []struct{ table, name, def string }{
		{"users", "invite_code", "TEXT"},
		{"users", "is_disabled", "INTEGER NOT NULL DEFAULT 0"},
		{"users", "trigger_minute", "INTEGER NOT NULL DEFAULT 2"},
		{"users", "jitter_sec", "INTEGER NOT NULL DEFAULT 180"},
		{"users", "retry_count", "INTEGER NOT NULL DEFAULT 3"},
		{"users", "retry_gap_min", "INTEGER NOT NULL DEFAULT 5"},
		{"users", "saved_locations", "TEXT NOT NULL DEFAULT '[]'"},
		{"users", "pin_hash", "BLOB"},
		{"users", "dorm_id", "INTEGER"},
		{"users", "send_address_fields", "INTEGER NOT NULL DEFAULT 0"},
		{"users", "user_avatar_url", "TEXT NOT NULL DEFAULT ''"},
		{"users", "notify_email", "TEXT NOT NULL DEFAULT ''"},
		{"users", "notify_enabled", "INTEGER NOT NULL DEFAULT 0"},
		// 7-bit bitmask, bit 0 = Monday … bit 6 = Sunday. 127 (= 0b1111111) means
		// every day, which preserves legacy behaviour for users created before
		// this column existed.
		{"users", "sign_days", "INTEGER NOT NULL DEFAULT 127"},
		// Guest mode: temporary users created by admin, sign on specific
		// dates (sign_dates JSON list), no PIN, auto-deleted after expiry.
		{"users", "is_guest", "INTEGER NOT NULL DEFAULT 0"},
		{"users", "guest_label", "TEXT NOT NULL DEFAULT ''"},
		{"users", "sign_dates", "TEXT NOT NULL DEFAULT '[]'"},
		{"users", "expires_at", "INTEGER"},
		// Server酱 per-user push channel — independent from email, can be
		// enabled together or alone. Empty key means disabled regardless
		// of the flag.
		{"users", "server_chan_key", "TEXT NOT NULL DEFAULT ''"},
		{"users", "server_chan_enabled", "INTEGER NOT NULL DEFAULT 0"},
		// Last time a token-expiry warning was sent for the current token.
		// Reset to 0 on UpdateToken so each token cycle warns at most once.
		{"users", "token_warned_at", "INTEGER NOT NULL DEFAULT 0"},
		// User-driven "I won't be on campus" skip list. JSON array of
		// YYYY-MM-DD strings. Scheduler treats any date in the list as
		// "skip silently" — no sign attempt, no failure record.
		{"users", "skip_dates", "TEXT NOT NULL DEFAULT '[]'"},
		{"dorm_locations", "send_address_fields", "INTEGER NOT NULL DEFAULT 0"},
		{"web_sessions", "is_admin", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, u := range upgrades {
		_, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", u.table, u.name, u.def))
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate add %s.%s: %w", u.table, u.name, err)
		}
	}

	// Announcements: admin-authored notices shown on the user Dashboard.
	// Own table (not key-value config) so we can keep many rows, each with
	// its own expiry / level / created_at.
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS announcements (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  title       TEXT NOT NULL,
  content     TEXT NOT NULL,
  level       TEXT NOT NULL DEFAULT 'info',
  expires_at  INTEGER,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
)`); err != nil {
		return fmt.Errorf("create announcements: %w", err)
	}
	return nil
}
