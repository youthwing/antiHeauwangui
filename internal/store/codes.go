package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// InviteCode represents one row of invite_codes.
// BoundUserName is filled by admin-side queries via JOIN with users; empty otherwise.
type InviteCode struct {
	Code          string
	BoundUserID   *string
	BoundUserName string // populated only by ListAdminCodes
	BoundAt       *time.Time
	Note          string
	Disabled      bool
	CreatedAt     time.Time
	CreatedBy     string
}

// Used reports whether the code has been bound to a user.
func (c *InviteCode) Used() bool { return c.BoundUserID != nil && *c.BoundUserID != "" }

// NewCode generates a fresh random invite code in the form "XXX-XXX-XXXX".
func NewCode() (string, error) {
	a, err := randSeg(3)
	if err != nil {
		return "", err
	}
	b, err := randSeg(3)
	if err != nil {
		return "", err
	}
	c, err := randSeg(4)
	if err != nil {
		return "", err
	}
	return a + "-" + b + "-" + c, nil
}

// Excludes look-alike letters/digits.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randSeg(n int) (string, error) {
	buf := make([]byte, n)
	rb := make([]byte, n)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	for i, b := range rb {
		buf[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}
	return string(buf), nil
}

// CreateCode inserts a fresh code row. The code is generated server-side.
func (s *Store) CreateCode(ctx context.Context, note, createdBy string) (*InviteCode, error) {
	code, err := NewCode()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO invite_codes (code, note, disabled, created_at, created_by)
VALUES (?, ?, 0, ?, ?)
`, code, note, now, createdBy)
	if err != nil {
		return nil, err
	}
	return &InviteCode{
		Code: code, Note: note,
		CreatedAt: time.Unix(now, 0), CreatedBy: createdBy,
	}, nil
}

// CreateCodes generates `count` fresh codes in one transaction.
func (s *Store) CreateCodes(ctx context.Context, count int, note, createdBy string) ([]*InviteCode, error) {
	if count <= 0 || count > 100 {
		return nil, errors.New("count must be 1–100")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	out := make([]*InviteCode, 0, count)
	for i := 0; i < count; i++ {
		code, err := NewCode()
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO invite_codes (code, note, disabled, created_at, created_by)
VALUES (?, ?, 0, ?, ?)
`, code, note, now, createdBy); err != nil {
			return nil, err
		}
		out = append(out, &InviteCode{
			Code: code, Note: note,
			CreatedAt: time.Unix(now, 0), CreatedBy: createdBy,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCode returns the invite_codes row.
func (s *Store) GetCode(ctx context.Context, code string) (*InviteCode, error) {
	return s.scanCode(s.db.QueryRowContext(ctx, `
SELECT code, bound_user_id, bound_at, note, disabled, created_at, created_by
FROM invite_codes WHERE code = ?
`, code))
}

// BindCode atomically binds an unbound code to a user.
// Returns ErrConflict if the code is already bound to a different user, or ErrNotFound if missing.
func (s *Store) BindCode(ctx context.Context, code, userID string) (*InviteCode, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	cur, err := s.scanCode(tx.QueryRowContext(ctx, `
SELECT code, bound_user_id, bound_at, note, disabled, created_at, created_by
FROM invite_codes WHERE code = ?
`, code))
	if err != nil {
		return nil, err
	}
	if cur.Disabled {
		return nil, fmt.Errorf("%w: 邀请码已禁用", ErrConflict)
	}
	if cur.Used() && cur.BoundUserID != nil && *cur.BoundUserID != userID {
		return nil, fmt.Errorf("%w: 邀请码已绑定到另一个账号", ErrConflict)
	}
	if !cur.Used() {
		now := time.Now().Unix()
		if _, err := tx.ExecContext(ctx, `
UPDATE invite_codes SET bound_user_id = ?, bound_at = ? WHERE code = ?
`, userID, now, code); err != nil {
			return nil, err
		}
		cur.BoundUserID = &userID
		t := time.Unix(now, 0)
		cur.BoundAt = &t
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return cur, nil
}

type CodeFilter struct {
	Status string // "" | "used" | "unused"
	Search string // substring on code or note
	Limit  int
	Offset int
}

// ListAdminCodes returns codes joined with the bound user's name (when bound).
func (s *Store) ListAdminCodes(ctx context.Context, f CodeFilter) ([]*InviteCode, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	switch f.Status {
	case "used":
		where = append(where, "c.bound_user_id IS NOT NULL")
	case "unused":
		where = append(where, "c.bound_user_id IS NULL")
	}
	if f.Search != "" {
		where = append(where, "(c.code LIKE ? OR c.note LIKE ? OR u.user_name LIKE ? OR u.user_number LIKE ?)")
		args = append(args, "%"+f.Search+"%", "%"+f.Search+"%", "%"+f.Search+"%", "%"+f.Search+"%")
	}
	q := `
SELECT c.code, c.bound_user_id, c.bound_at, c.note, c.disabled, c.created_at, c.created_by,
       COALESCE(u.user_name, '') AS user_name
FROM invite_codes c
LEFT JOIN users u ON u.user_id = c.bound_user_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY c.created_at DESC LIMIT ? OFFSET ?
`
	args = append(args, f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*InviteCode, 0)
	for rows.Next() {
		var c InviteCode
		var boundUser sql.NullString
		var boundAt sql.NullInt64
		var createdAt int64
		var disabled int
		var userName string
		if err := rows.Scan(&c.Code, &boundUser, &boundAt, &c.Note,
			&disabled, &createdAt, &c.CreatedBy, &userName); err != nil {
			return nil, err
		}
		if boundUser.Valid {
			id := boundUser.String
			c.BoundUserID = &id
		}
		if boundAt.Valid {
			t := time.Unix(boundAt.Int64, 0)
			c.BoundAt = &t
		}
		c.Disabled = disabled != 0
		c.CreatedAt = time.Unix(createdAt, 0)
		c.BoundUserName = userName
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (s *Store) ListCodes(ctx context.Context, f CodeFilter) ([]*InviteCode, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	switch f.Status {
	case "used":
		where = append(where, "bound_user_id IS NOT NULL")
	case "unused":
		where = append(where, "bound_user_id IS NULL")
	}
	if f.Search != "" {
		where = append(where, "(code LIKE ? OR note LIKE ?)")
		args = append(args, "%"+f.Search+"%", "%"+f.Search+"%")
	}
	q := `
SELECT code, bound_user_id, bound_at, note, disabled, created_at, created_by
FROM invite_codes WHERE ` + strings.Join(where, " AND ") + `
ORDER BY created_at DESC LIMIT ? OFFSET ?
`
	args = append(args, f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*InviteCode, 0)
	for rows.Next() {
		c, err := s.scanCodeRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCode(ctx context.Context, code string, note *string, disabled *bool) error {
	sets := []string{}
	args := []any{}
	if note != nil {
		sets = append(sets, "note = ?")
		args = append(args, *note)
	}
	if disabled != nil {
		sets = append(sets, "disabled = ?")
		args = append(args, boolInt(*disabled))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, code)
	_, err := s.db.ExecContext(ctx, "UPDATE invite_codes SET "+strings.Join(sets, ", ")+" WHERE code = ?", args...)
	return err
}

func (s *Store) DeleteCode(ctx context.Context, code string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM invite_codes WHERE code = ? AND bound_user_id IS NULL`, code)
	return err
}

// CountCodes returns (total, used) counts.
func (s *Store) CountCodes(ctx context.Context) (total int, used int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invite_codes`).Scan(&total)
	if err != nil {
		return
	}
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invite_codes WHERE bound_user_id IS NOT NULL`).Scan(&used)
	return
}

func (s *Store) scanCode(row *sql.Row) (*InviteCode, error) {
	var c InviteCode
	var boundUser sql.NullString
	var boundAt sql.NullInt64
	var createdAt int64
	var disabled int
	err := row.Scan(&c.Code, &boundUser, &boundAt, &c.Note, &disabled, &createdAt, &c.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if boundUser.Valid {
		s := boundUser.String
		c.BoundUserID = &s
	}
	if boundAt.Valid {
		t := time.Unix(boundAt.Int64, 0)
		c.BoundAt = &t
	}
	c.Disabled = disabled != 0
	c.CreatedAt = time.Unix(createdAt, 0)
	return &c, nil
}

func (s *Store) scanCodeRow(rows *sql.Rows) (*InviteCode, error) {
	var c InviteCode
	var boundUser sql.NullString
	var boundAt sql.NullInt64
	var createdAt int64
	var disabled int
	if err := rows.Scan(&c.Code, &boundUser, &boundAt, &c.Note, &disabled, &createdAt, &c.CreatedBy); err != nil {
		return nil, err
	}
	if boundUser.Valid {
		s := boundUser.String
		c.BoundUserID = &s
	}
	if boundAt.Valid {
		t := time.Unix(boundAt.Int64, 0)
		c.BoundAt = &t
	}
	c.Disabled = disabled != 0
	c.CreatedAt = time.Unix(createdAt, 0)
	return &c, nil
}
