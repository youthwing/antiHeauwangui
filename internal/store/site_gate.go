package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// SiteAccessCode is a one-time code used only to reveal the site. It does not
// log a user in or bind to an account.
type SiteAccessCode struct {
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewSiteAccessCode() (string, error) {
	a, err := randSeg(4)
	if err != nil {
		return "", err
	}
	b, err := randSeg(4)
	if err != nil {
		return "", err
	}
	c, err := randSeg(4)
	if err != nil {
		return "", err
	}
	return "AWG-" + a + "-" + b + "-" + c, nil
}

func normalizeSiteAccessCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	b.Grow(len(code))
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func siteAccessCodeHash(code string) string {
	sum := sha256.Sum256([]byte(normalizeSiteAccessCode(code)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) CreateSiteAccessCode(ctx context.Context, createdBy string, ttl time.Duration) (*SiteAccessCode, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	now := time.Now()
	exp := now.Add(ttl)
	for i := 0; i < 5; i++ {
		code, err := NewSiteAccessCode()
		if err != nil {
			return nil, err
		}
		_, err = s.db.ExecContext(ctx, `
INSERT INTO site_access_codes (code_hash, created_by, created_at, expires_at)
VALUES (?, ?, ?, ?)
`, siteAccessCodeHash(code), strings.TrimSpace(createdBy), now.Unix(), exp.Unix())
		if err == nil {
			return &SiteAccessCode{Code: code, CreatedAt: now, ExpiresAt: exp}, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, err
		}
	}
	return nil, ErrConflict
}

func (s *Store) ConsumeSiteAccessCode(ctx context.Context, code, ip string) error {
	hash := siteAccessCodeHash(code)
	if hash == siteAccessCodeHash("") {
		return ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int64
	var expiresAt int64
	var usedAt sql.NullInt64
	err = tx.QueryRowContext(ctx, `
SELECT id, expires_at, used_at
FROM site_access_codes
WHERE code_hash = ?
`, hash).Scan(&id, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	if usedAt.Valid || expiresAt < now {
		return ErrNotFound
	}
	res, err := tx.ExecContext(ctx, `
UPDATE site_access_codes
SET used_at = ?, used_by_ip = ?
WHERE id = ? AND used_at IS NULL
`, now, strings.TrimSpace(ip), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) CreateSiteGatePass(ctx context.Context, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	passID := hex.EncodeToString(b)
	now := time.Now()
	exp := now.Add(ttl)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO site_gate_passes (pass_id, created_at, expires_at)
VALUES (?, ?, ?)
`, passID, now.Unix(), exp.Unix())
	if err != nil {
		return "", time.Time{}, err
	}
	return passID, exp, nil
}

func (s *Store) HasSiteGatePass(ctx context.Context, passID string) (bool, error) {
	passID = strings.TrimSpace(passID)
	if passID == "" {
		return false, nil
	}
	var exp int64
	err := s.db.QueryRowContext(ctx, `
SELECT expires_at FROM site_gate_passes WHERE pass_id = ?
`, passID).Scan(&exp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Now().Unix() <= exp, nil
}

func (s *Store) GCSiteGatePasses(ctx context.Context) error {
	now := time.Now().Unix()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM site_gate_passes WHERE expires_at < ?`, now); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
DELETE FROM site_access_codes
WHERE expires_at < ? OR (used_at IS NOT NULL AND used_at < ?)
`, now, now-7*24*3600)
	return err
}
