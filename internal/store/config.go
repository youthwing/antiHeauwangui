package store

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

// ---------- system_config key/value layer ----------
//
// All config keys live in the system_config table. We use a thin wrapper to
// keep encryption of sensitive values out of the call sites. A subset of keys
// is encrypted at rest using the same AES master key that protects user tokens.

var encryptedKeys = map[string]bool{
	"smtp.password": true,
}

// SetConfig sets a single key. If the key is in encryptedKeys, the value is
// AES-GCM encrypted before persistence (and stored as hex inside the TEXT cell
// for simplicity — keeps the API uniform).
func (s *Store) SetConfig(ctx context.Context, key, value string) error {
	stored := value
	if encryptedKeys[key] && value != "" {
		enc, err := s.c.Encrypt([]byte(value))
		if err != nil {
			return err
		}
		stored = "enc:" + hexEncode(enc)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO system_config(key, value, updated_at) VALUES(?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
`, key, stored, time.Now().Unix())
	return err
}

// GetConfig returns the decrypted value for a key, or empty if missing.
func (s *Store) GetConfig(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if encryptedKeys[key] && len(v) > 4 && v[:4] == "enc:" {
		raw, err := hexDecode(v[4:])
		if err != nil {
			return "", err
		}
		plain, err := s.c.Decrypt(raw)
		if err != nil {
			return "", err
		}
		return string(plain), nil
	}
	return v, nil
}

// GetConfigInt is a thin numeric helper.
func (s *Store) GetConfigInt(ctx context.Context, key string, def int) (int, error) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == "" {
		return def, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def, nil
	}
	return n, nil
}

// GetConfigBool returns true for "1" / "true" / "yes".
func (s *Store) GetConfigBool(ctx context.Context, key string) (bool, error) {
	v, err := s.GetConfig(ctx, key)
	if err != nil || v == "" {
		return false, err
	}
	return v == "1" || v == "true" || v == "yes", nil
}

// ---------- SMTP config (typed helper on top of system_config) ----------

const (
	cfgSMTPEnabled  = "smtp.enabled"
	cfgSMTPHost     = "smtp.host"
	cfgSMTPPort     = "smtp.port"
	cfgSMTPUsername = "smtp.username"
	cfgSMTPPassword = "smtp.password"
	cfgSMTPFrom     = "smtp.from"
	cfgSMTPAdminBcc = "smtp.admin_bcc"
)

// SMTPConfig is the full settings bundle. Password is plaintext in memory only.
type SMTPConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`     // optional display name "Name <addr>"
	AdminBcc string `json:"adminBcc"` // admin global bcc address
}

func (s *Store) GetSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	c := &SMTPConfig{}
	var err error
	c.Enabled, err = s.GetConfigBool(ctx, cfgSMTPEnabled)
	if err != nil {
		return nil, err
	}
	if c.Host, err = s.GetConfig(ctx, cfgSMTPHost); err != nil {
		return nil, err
	}
	if c.Host == "" {
		c.Host = "smtp.gmail.com"
	}
	c.Port, err = s.GetConfigInt(ctx, cfgSMTPPort, 587)
	if err != nil {
		return nil, err
	}
	if c.Username, err = s.GetConfig(ctx, cfgSMTPUsername); err != nil {
		return nil, err
	}
	if c.Password, err = s.GetConfig(ctx, cfgSMTPPassword); err != nil {
		return nil, err
	}
	if c.From, err = s.GetConfig(ctx, cfgSMTPFrom); err != nil {
		return nil, err
	}
	if c.AdminBcc, err = s.GetConfig(ctx, cfgSMTPAdminBcc); err != nil {
		return nil, err
	}
	return c, nil
}

// SetSMTPConfig persists all SMTP fields atomically. Pass an empty Password to
// keep the previously-stored password (typical "update form" semantics).
func (s *Store) SetSMTPConfig(ctx context.Context, c *SMTPConfig) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	set := func(key, val string) error {
		stored := val
		if encryptedKeys[key] && val != "" {
			enc, err := s.c.Encrypt([]byte(val))
			if err != nil {
				return err
			}
			stored = "enc:" + hexEncode(enc)
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO system_config(key, value, updated_at) VALUES(?,?,?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
`, key, stored, time.Now().Unix())
		return err
	}
	if c.Enabled {
		if err := set(cfgSMTPEnabled, "1"); err != nil {
			return err
		}
	} else {
		if err := set(cfgSMTPEnabled, "0"); err != nil {
			return err
		}
	}
	if err := set(cfgSMTPHost, c.Host); err != nil {
		return err
	}
	if err := set(cfgSMTPPort, strconv.Itoa(c.Port)); err != nil {
		return err
	}
	if err := set(cfgSMTPUsername, c.Username); err != nil {
		return err
	}
	// Only overwrite password if a new one is provided. Empty means "keep".
	if c.Password != "" {
		if err := set(cfgSMTPPassword, c.Password); err != nil {
			return err
		}
	}
	if err := set(cfgSMTPFrom, c.From); err != nil {
		return err
	}
	if err := set(cfgSMTPAdminBcc, c.AdminBcc); err != nil {
		return err
	}
	return tx.Commit()
}

// ---------- hex helpers (tiny, avoids importing encoding/hex everywhere) ----------

const hexAlphabet = "0123456789abcdef"

func hexEncode(b []byte) string {
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexAlphabet[v>>4]
		out[i*2+1] = hexAlphabet[v&0x0f]
	}
	return string(out)
}

func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errHexLen
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi, ok := hexNibble(s[i])
		if !ok {
			return nil, errHexChar
		}
		lo, ok := hexNibble(s[i+1])
		if !ok {
			return nil, errHexChar
		}
		out[i/2] = hi<<4 | lo
	}
	return out, nil
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

var (
	errHexLen  = sqlError("invalid hex length")
	errHexChar = sqlError("invalid hex character")
)

type sqlError string

func (e sqlError) Error() string { return string(e) }
