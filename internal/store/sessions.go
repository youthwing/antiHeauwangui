package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

type Session struct {
	SessionID string
	UserID    string
	IsAdmin   bool
	ExpiresAt time.Time
}

// CreateSession issues a fresh random session ID bound to userID.
// If isAdmin is true, the session is tagged as an admin session.
func (s *Store) CreateSession(ctx context.Context, userID string, isAdmin bool, ttl time.Duration) (*Session, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	sid := hex.EncodeToString(b)
	exp := time.Now().Add(ttl)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO web_sessions(session_id, user_id, is_admin, expires_at) VALUES(?,?,?,?)`,
		sid, userID, boolInt(isAdmin), exp.Unix()); err != nil {
		return nil, err
	}
	return &Session{SessionID: sid, UserID: userID, IsAdmin: isAdmin, ExpiresAt: exp}, nil
}

// GetSession returns the session iff it exists and hasn't expired.
func (s *Store) GetSession(ctx context.Context, sid string) (*Session, error) {
	var sess Session
	var exp int64
	var isAdmin int
	err := s.db.QueryRowContext(ctx,
		`SELECT session_id, user_id, is_admin, expires_at FROM web_sessions WHERE session_id=?`,
		sid).Scan(&sess.SessionID, &sess.UserID, &isAdmin, &exp)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess.IsAdmin = isAdmin != 0
	sess.ExpiresAt = time.Unix(exp, 0)
	if time.Now().After(sess.ExpiresAt) {
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, sid string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE session_id=?`, sid)
	return err
}

// DeleteSessionsForUser invalidates all of a user's sessions (force logout).
func (s *Store) DeleteSessionsForUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE user_id = ? AND is_admin = 0`, userID)
	return err
}

func (s *Store) GCSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM web_sessions WHERE expires_at < ?`, time.Now().Unix())
	return err
}
