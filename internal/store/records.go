package store

import (
	"context"
	"time"
)

// Record represents a single check-in attempt outcome.
type Record struct {
	ID         int64
	UserID     string
	UserName   string // joined; only filled by admin queries
	RuleID     int
	Status     string // success | already | exempt | failed | skipped
	Message    string
	OccurredAt time.Time
}

func (s *Store) AddRecord(ctx context.Context, r *Record) error {
	if r.OccurredAt.IsZero() {
		r.OccurredAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO sign_records(user_id, rule_id, status, message, occurred_at)
VALUES (?,?,?,?,?)
`, r.UserID, r.RuleID, r.Status, r.Message, r.OccurredAt.Unix())
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

// ListRecords returns the most recent records for a user (newest first).
func (s *Store) ListRecords(ctx context.Context, userID string, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, rule_id, status, message, occurred_at
FROM sign_records WHERE user_id = ? ORDER BY occurred_at DESC LIMIT ?
`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// ListRecordsBetween returns every record for a user with occurred_at in
// [from, to] (both inclusive at second resolution). Newest first. Used by
// the stats endpoint to compute streaks + monthly aggregates without
// hitting an arbitrary LIMIT.
func (s *Store) ListRecordsBetween(ctx context.Context, userID string, from, to time.Time) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, rule_id, status, message, occurred_at
FROM sign_records
WHERE user_id = ? AND occurred_at >= ? AND occurred_at <= ?
ORDER BY occurred_at DESC
`, userID, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// ListAllRecordsBetween (admin) returns every record across all users with
// occurred_at in [from, to]. Used by the CSV export.
func (s *Store) ListAllRecordsBetween(ctx context.Context, from, to time.Time) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.user_id, COALESCE(u.user_name,'') AS user_name,
       r.rule_id, r.status, r.message, r.occurred_at
FROM sign_records r LEFT JOIN users u ON u.user_id = r.user_id
WHERE r.occurred_at >= ? AND r.occurred_at <= ?
ORDER BY r.occurred_at DESC
`, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Record, 0)
	for rows.Next() {
		var r Record
		var ts int64
		if err := rows.Scan(&r.ID, &r.UserID, &r.UserName, &r.RuleID, &r.Status, &r.Message, &ts); err != nil {
			return nil, err
		}
		r.OccurredAt = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListAllRecords (admin) returns the most recent records across all users.
func (s *Store) ListAllRecords(ctx context.Context, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.user_id, COALESCE(u.user_name,'') AS user_name,
       r.rule_id, r.status, r.message, r.occurred_at
FROM sign_records r LEFT JOIN users u ON u.user_id = r.user_id
ORDER BY r.occurred_at DESC LIMIT ?
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Record, 0)
	for rows.Next() {
		var r Record
		var ts int64
		if err := rows.Scan(&r.ID, &r.UserID, &r.UserName, &r.RuleID, &r.Status, &r.Message, &ts); err != nil {
			return nil, err
		}
		r.OccurredAt = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountTodayRecords groups today's records by status for the admin dashboard.
func (s *Store) CountTodayRecords(ctx context.Context) (map[string]int, error) {
	today := time.Now()
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	rows, err := s.db.QueryContext(ctx, `
SELECT status, COUNT(*) FROM sign_records WHERE occurred_at >= ? GROUP BY status
`, start.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

func scanRecords(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]Record, error) {
	out := make([]Record, 0)
	for rows.Next() {
		var r Record
		var ts int64
		if err := rows.Scan(&r.ID, &r.UserID, &r.RuleID, &r.Status, &r.Message, &ts); err != nil {
			return nil, err
		}
		r.OccurredAt = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}
