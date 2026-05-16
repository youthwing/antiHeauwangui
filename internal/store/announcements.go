package store

import (
	"context"
	"database/sql"
	"time"
)

// Announcement is a single admin-authored notice surfaced on the user
// Dashboard. Level drives the visual tone (info / success / warning /
// critical); expires_at is optional — null means "show until deleted".
type Announcement struct {
	ID        int64
	Title     string
	Content   string
	Level     string // info | success | warning | critical
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListActiveAnnouncements returns announcements that are NOT expired,
// newest first. Used by the user-facing /announcements endpoint.
func (s *Store) ListActiveAnnouncements(ctx context.Context) ([]Announcement, error) {
	now := time.Now().Unix()
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, content, level, expires_at, created_at, updated_at
FROM announcements
WHERE expires_at IS NULL OR expires_at > ?
ORDER BY created_at DESC
`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnouncements(rows)
}

// ListAllAnnouncements returns every announcement (admin view), newest first.
func (s *Store) ListAllAnnouncements(ctx context.Context) ([]Announcement, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, content, level, expires_at, created_at, updated_at
FROM announcements
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnouncements(rows)
}

// GetAnnouncement fetches one by id.
func (s *Store) GetAnnouncement(ctx context.Context, id int64) (*Announcement, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, title, content, level, expires_at, created_at, updated_at
FROM announcements WHERE id = ?
`, id)
	var a Announcement
	var exp sql.NullInt64
	var createdAt, updatedAt int64
	if err := row.Scan(&a.ID, &a.Title, &a.Content, &a.Level, &exp, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if exp.Valid {
		t := time.Unix(exp.Int64, 0)
		a.ExpiresAt = &t
	}
	a.CreatedAt = time.Unix(createdAt, 0)
	a.UpdatedAt = time.Unix(updatedAt, 0)
	return &a, nil
}

// CreateAnnouncement inserts a new announcement. ID is populated on return.
func (s *Store) CreateAnnouncement(ctx context.Context, a *Announcement) error {
	now := time.Now().Unix()
	a.CreatedAt = time.Unix(now, 0)
	a.UpdatedAt = a.CreatedAt
	var exp any
	if a.ExpiresAt != nil {
		exp = a.ExpiresAt.Unix()
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO announcements (title, content, level, expires_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
`, a.Title, a.Content, a.Level, exp, now, now)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// UpdateAnnouncement patches title/content/level/expires_at by id.
func (s *Store) UpdateAnnouncement(ctx context.Context, a *Announcement) error {
	now := time.Now().Unix()
	var exp any
	if a.ExpiresAt != nil {
		exp = a.ExpiresAt.Unix()
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE announcements
SET title = ?, content = ?, level = ?, expires_at = ?, updated_at = ?
WHERE id = ?
`, a.Title, a.Content, a.Level, exp, now, a.ID)
	return err
}

// DeleteAnnouncement removes one by id.
func (s *Store) DeleteAnnouncement(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM announcements WHERE id = ?`, id)
	return err
}

func scanAnnouncements(rows *sql.Rows) ([]Announcement, error) {
	out := make([]Announcement, 0)
	for rows.Next() {
		var a Announcement
		var exp sql.NullInt64
		var createdAt, updatedAt int64
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &a.Level, &exp, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if exp.Valid {
			t := time.Unix(exp.Int64, 0)
			a.ExpiresAt = &t
		}
		a.CreatedAt = time.Unix(createdAt, 0)
		a.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, a)
	}
	return out, rows.Err()
}
