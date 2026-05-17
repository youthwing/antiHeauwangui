package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// SavedLocation is one entry in the user's saved location list (JSON-serialized).
type SavedLocation struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	Road      string  `json:"road"`
	Poi       string  `json:"poi"`
}

// User mirrors the users table; Token is plaintext in memory only.
type User struct {
	UserID        string
	UserName      string
	UserNumber    string
	UserSection   string
	UserClass     string
	UserAvatarURL string
	Token         string
	TokenExp      time.Time
	AutoSign     bool
	IsDisabled   bool
	InviteCode   string

	Lat          float64
	Lng          float64
	Address      string
	City         string
	Road         string
	Poi          string
	DeviceModel  string
	DeviceSystem string

	TriggerMinute  int
	JitterSec      int
	RetryCount     int
	RetryGapMin    int
	SavedLocations string // raw JSON; legacy, unused by current UI

	DormID            *int64 // nullable foreign key to dorm_locations.id
	SendAddressFields bool   // snapshotted from dorm at bind time

	NotifyEmail   string // user-facing email for sign-result notifications
	NotifyEnabled bool   // whether to send emails to this user

	// SignDays is a 7-bit bitmask of which weekdays to auto-sign on.
	// bit 0 = Monday … bit 6 = Sunday. 127 = every day.
	// 0 effectively disables auto-sign even when AutoSign==true.
	SignDays int

	// Guest fields: when IsGuest==true, this is an admin-managed temporary
	// account. Schedule is driven by SignDates (JSON array of "YYYY-MM-DD")
	// instead of SignDays. The account is auto-deleted after ExpiresAt.
	// Guests have no PIN and never receive sign-result emails — only admin
	// BCC sees their outcomes.
	IsGuest    bool       // is_guest
	GuestLabel string     // guest_label — admin-visible nickname
	SignDates  string     // sign_dates — raw JSON "[YYYY-MM-DD, ...]"
	ExpiresAt  *time.Time // expires_at — nil = no auto-cleanup

	// Server酱 push channel — independent from email. Empty key disables.
	ServerChanKey     string
	ServerChanEnabled bool

	// TokenWarnedAt is the unix timestamp of the last token-expiry warning
	// we sent for the *current* token. Reset to 0 on UpdateToken so a fresh
	// token starts the warning cycle over.
	TokenWarnedAt int64

	// SkipDates is a JSON array of "YYYY-MM-DD" strings the user has marked
	// "I won't be on campus" — scheduler skips those dates silently. Used
	// for the Dashboard "今晚不在校" button to avoid the谎报位置 risk.
	SkipDates string

	PinHash []byte // bcrypt hash of the 4–6 digit login PIN; nil = no PIN set

	CreatedAt time.Time
	UpdatedAt time.Time
}

const userColumns = `user_id, user_name, user_number, user_section, user_class,
  token_enc, token_exp, auto_sign, is_disabled, invite_code,
  lat, lng, address, city, road, poi,
  device_model, device_system,
  trigger_minute, jitter_sec, retry_count, retry_gap_min, saved_locations,
  pin_hash, dorm_id, send_address_fields, user_avatar_url,
  notify_email, notify_enabled, sign_days,
  is_guest, guest_label, sign_dates, expires_at,
  server_chan_key, server_chan_enabled, token_warned_at,
  skip_dates,
  created_at, updated_at`

// UpsertUser inserts a new user or updates identity + token of an existing one.
func (s *Store) UpsertUser(ctx context.Context, u *User) error {
	enc, err := s.c.Encrypt([]byte(u.Token))
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Unix(now, 0)
	}
	u.UpdatedAt = time.Unix(now, 0)
	if u.TriggerMinute == 0 {
		u.TriggerMinute = 2
	}
	if u.JitterSec == 0 {
		u.JitterSec = 180
	}
	if u.RetryCount == 0 {
		u.RetryCount = 3
	}
	if u.RetryGapMin == 0 {
		u.RetryGapMin = 5
	}
	if u.SavedLocations == "" {
		u.SavedLocations = "[]"
	}
	if u.SignDays == 0 {
		// 0 would mean "never sign" — default new users to every day so
		// activation doesn't accidentally disable auto-sign.
		u.SignDays = 127
	}
	if u.SignDates == "" {
		u.SignDates = "[]"
	}
	if u.SkipDates == "" {
		u.SkipDates = "[]"
	}
	var expiresAt any
	if u.ExpiresAt != nil {
		expiresAt = u.ExpiresAt.Unix()
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO users (
  user_id, user_name, user_number, user_section, user_class,
  token_enc, token_exp, auto_sign, is_disabled, invite_code,
  lat, lng, address, city, road, poi, device_model, device_system,
  trigger_minute, jitter_sec, retry_count, retry_gap_min, saved_locations,
  pin_hash, dorm_id, send_address_fields, user_avatar_url,
  notify_email, notify_enabled, sign_days,
  is_guest, guest_label, sign_dates, expires_at,
  server_chan_key, server_chan_enabled, token_warned_at,
  skip_dates,
  created_at, updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(user_id) DO UPDATE SET
  user_name        = excluded.user_name,
  user_number      = excluded.user_number,
  user_section     = excluded.user_section,
  user_class       = excluded.user_class,
  user_avatar_url  = excluded.user_avatar_url,
  token_enc        = excluded.token_enc,
  token_exp        = excluded.token_exp,
  invite_code      = COALESCE(excluded.invite_code, users.invite_code),
  pin_hash         = COALESCE(excluded.pin_hash, users.pin_hash),
  updated_at       = excluded.updated_at
`,
		u.UserID, u.UserName, u.UserNumber, u.UserSection, u.UserClass,
		enc, u.TokenExp.Unix(), boolInt(u.AutoSign), boolInt(u.IsDisabled), nullStr(u.InviteCode),
		u.Lat, u.Lng, u.Address, u.City, u.Road, u.Poi,
		u.DeviceModel, u.DeviceSystem,
		u.TriggerMinute, u.JitterSec, u.RetryCount, u.RetryGapMin, u.SavedLocations,
		u.PinHash, u.DormID, boolInt(u.SendAddressFields), u.UserAvatarURL,
		u.NotifyEmail, boolInt(u.NotifyEnabled), u.SignDays,
		boolInt(u.IsGuest), u.GuestLabel, u.SignDates, expiresAt,
		u.ServerChanKey, boolInt(u.ServerChanEnabled), u.TokenWarnedAt,
		u.SkipDates,
		u.CreatedAt.Unix(), u.UpdatedAt.Unix())
	return err
}

// GetUser returns the full user record with decrypted token.
func (s *Store) GetUser(ctx context.Context, userID string) (*User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE user_id = ?", userID)
	return s.scanUser(row)
}

// scanUser supports either *sql.Row or *sql.Rows (anything with Scan).
type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanUser(r rowScanner) (*User, error) {
	var u User
	var enc []byte
	var tokenExp, createdAt, updatedAt int64
	var autoSign, isDisabled int
	var inviteCode sql.NullString
	var pinHash []byte
	var dormID sql.NullInt64
	var sendFields, notifyEnabled, isGuest, serverChanEnabled int
	var expiresAt sql.NullInt64
	err := r.Scan(
		&u.UserID, &u.UserName, &u.UserNumber, &u.UserSection, &u.UserClass,
		&enc, &tokenExp, &autoSign, &isDisabled, &inviteCode,
		&u.Lat, &u.Lng, &u.Address, &u.City, &u.Road, &u.Poi,
		&u.DeviceModel, &u.DeviceSystem,
		&u.TriggerMinute, &u.JitterSec, &u.RetryCount, &u.RetryGapMin, &u.SavedLocations,
		&pinHash, &dormID, &sendFields, &u.UserAvatarURL,
		&u.NotifyEmail, &notifyEnabled, &u.SignDays,
		&isGuest, &u.GuestLabel, &u.SignDates, &expiresAt,
		&u.ServerChanKey, &serverChanEnabled, &u.TokenWarnedAt,
		&u.SkipDates,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	tok, err := s.c.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	u.Token = string(tok)
	u.TokenExp = time.Unix(tokenExp, 0)
	u.AutoSign = autoSign != 0
	u.IsDisabled = isDisabled != 0
	if inviteCode.Valid {
		u.InviteCode = inviteCode.String
	}
	u.PinHash = pinHash
	if dormID.Valid {
		v := dormID.Int64
		u.DormID = &v
	}
	u.SendAddressFields = sendFields != 0
	u.NotifyEnabled = notifyEnabled != 0
	u.IsGuest = isGuest != 0
	u.ServerChanEnabled = serverChanEnabled != 0
	if expiresAt.Valid {
		t := time.Unix(expiresAt.Int64, 0)
		u.ExpiresAt = &t
	}
	u.CreatedAt = time.Unix(createdAt, 0)
	u.UpdatedAt = time.Unix(updatedAt, 0)
	return &u, nil
}

// SetUserDorm binds a user to a dorm and snapshots its coordinates + address
// + send_address_fields flag into the users table.
// Pass dorm=nil to unbind.
func (s *Store) SetUserDorm(ctx context.Context, userID string, dorm *Dorm) error {
	if dorm == nil {
		_, err := s.db.ExecContext(ctx, `
UPDATE users SET dorm_id = NULL, send_address_fields = 0, updated_at = ? WHERE user_id = ?
`, time.Now().Unix(), userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE users SET
  dorm_id             = ?,
  lat                 = ?,
  lng                 = ?,
  address             = ?,
  city                = ?,
  road                = ?,
  poi                 = ?,
  send_address_fields = ?,
  updated_at          = ?
WHERE user_id = ?
`, dorm.ID, dorm.Latitude, dorm.Longitude,
		dorm.Address, dorm.City, dorm.Road, dorm.Poi,
		boolInt(dorm.SendAddressFields), time.Now().Unix(), userID)
	return err
}

// FindByNumber matches a user by exact userNumber (学号). Used by lightweight PIN login.
func (s *Store) FindByNumber(ctx context.Context, number string) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE user_number = ?",
		number)
	return s.scanUser(row)
}

// SetPinHash updates the bcrypt PIN hash for a user.
func (s *Store) SetPinHash(ctx context.Context, userID string, hash []byte) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET pin_hash = ?, updated_at = ? WHERE user_id = ?`,
		hash, time.Now().Unix(), userID)
	return err
}

// UpdateSettings persists auto_sign + device + schedule + notify. Location is
// managed separately via SetUserDorm (or directly via UPDATE for legacy compat).
func (s *Store) UpdateSettings(ctx context.Context, u *User) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE users SET
  auto_sign           = ?,
  lat                 = ?,
  lng                 = ?,
  address             = ?,
  city                = ?,
  road                = ?,
  poi                 = ?,
  device_model        = ?,
  device_system       = ?,
  trigger_minute      = ?,
  jitter_sec          = ?,
  retry_count         = ?,
  retry_gap_min       = ?,
  saved_locations     = ?,
  notify_email        = ?,
  notify_enabled      = ?,
  sign_days           = ?,
  server_chan_key     = ?,
  server_chan_enabled = ?,
  updated_at          = ?
WHERE user_id = ?
`,
		boolInt(u.AutoSign), u.Lat, u.Lng, u.Address, u.City, u.Road, u.Poi,
		u.DeviceModel, u.DeviceSystem,
		u.TriggerMinute, u.JitterSec, u.RetryCount, u.RetryGapMin, u.SavedLocations,
		u.NotifyEmail, boolInt(u.NotifyEnabled), u.SignDays,
		u.ServerChanKey, boolInt(u.ServerChanEnabled),
		time.Now().Unix(), u.UserID)
	return err
}

// UpdateToken rotates the stored token for a user. Resets token_warned_at to
// 0 so the new token cycle starts fresh — a user who refreshes proactively
// won't get re-warned about the *old* token's imminent expiry.
func (s *Store) UpdateToken(ctx context.Context, userID, token string, exp time.Time) error {
	enc, err := s.c.Encrypt([]byte(token))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE users SET
  token_enc       = ?,
  token_exp       = ?,
  token_warned_at = 0,
  updated_at      = ?
WHERE user_id = ?
`, enc, exp.Unix(), time.Now().Unix(), userID)
	return err
}

// UpdateSkipDates replaces the user's skip-dates JSON list. Caller validates
// the date strings; we store whatever is passed (with a trivial guard
// against empty string).
func (s *Store) UpdateSkipDates(ctx context.Context, userID, skipDatesJSON string) error {
	if skipDatesJSON == "" {
		skipDatesJSON = "[]"
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET skip_dates = ?, updated_at = ? WHERE user_id = ?`,
		skipDatesJSON, time.Now().Unix(), userID)
	return err
}

// MarkTokenWarned records that a token-expiry warning was just dispatched
// for this user's current token. Reset by UpdateToken next time the token
// rotates, so each token cycle warns at most once.
func (s *Store) MarkTokenWarned(ctx context.Context, userID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET token_warned_at = ?, updated_at = ? WHERE user_id = ?`,
		at.Unix(), time.Now().Unix(), userID)
	return err
}

// UpdateUserProfile refreshes the snapshot fields we copy from the school
// system (name, class, avatar). Called after a token refresh so a user whose
// avatar fetch failed during activation can get a working avatar simply by
// re-grabbing their token.
func (s *Store) UpdateUserProfile(ctx context.Context, userID, name, number, section, class, avatarURL string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE users SET
  user_name       = ?,
  user_number     = ?,
  user_section    = ?,
  user_class      = ?,
  user_avatar_url = ?,
  updated_at      = ?
WHERE user_id = ?
`, name, number, section, class, avatarURL, time.Now().Unix(), userID)
	return err
}

// SetDisabled toggles a user's is_disabled flag.
func (s *Store) SetDisabled(ctx context.Context, userID string, disabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET is_disabled = ?, updated_at = ? WHERE user_id = ?`,
		boolInt(disabled), time.Now().Unix(), userID)
	return err
}

// SetInviteCode associates a user with an invite code.
func (s *Store) SetInviteCode(ctx context.Context, userID, code string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET invite_code = ?, updated_at = ? WHERE user_id = ?`,
		code, time.Now().Unix(), userID)
	return err
}

// ListAutoSignUsers returns user IDs with auto_sign enabled AND not disabled.
func (s *Store) ListAutoSignUsers(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id FROM users WHERE auto_sign = 1 AND is_disabled = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type UserListFilter struct {
	Search string
	Limit  int
	Offset int
}

// ListUsers returns users for admin display.
func (s *Store) ListUsers(ctx context.Context, f UserListFilter) ([]*User, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	if f.Search != "" {
		where = append(where, "(user_name LIKE ? OR user_number LIKE ? OR invite_code LIKE ?)")
		args = append(args, "%"+f.Search+"%", "%"+f.Search+"%", "%"+f.Search+"%")
	}
	q := "SELECT " + userColumns + " FROM users WHERE " + strings.Join(where, " AND ") + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*User, 0)
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListUsersByDorm returns all users currently bound to the given dorm,
// ordered by name. Used by the admin "show users in this dorm" view.
func (s *Store) ListUsersByDorm(ctx context.Context, dormID int64) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE dorm_id = ? ORDER BY user_name",
		dormID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*User, 0)
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers returns the total number of users (admin dashboard stat).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CountGuests returns the number of admin-managed temporary (guest) users.
func (s *Store) CountGuests(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE is_guest = 1`).Scan(&n)
	return n, err
}

// ListGuests returns all guest users, newest first.
func (s *Store) ListGuests(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE is_guest = 1 ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*User, 0)
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ExpiredGuest is the minimal info we keep about a guest we're about to
// delete — enough for an admin notification email.
type ExpiredGuest struct {
	UserID string
	Label  string
	Name   string // school's user_name, for the admin email body
}

// DeleteExpiredGuests removes guests whose expires_at < now. Returns the
// list of (user_id, label, name) we deleted so the caller can email admin.
func (s *Store) DeleteExpiredGuests(ctx context.Context, now time.Time) ([]ExpiredGuest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, guest_label, user_name FROM users
		 WHERE is_guest = 1 AND expires_at IS NOT NULL AND expires_at < ?`,
		now.Unix())
	if err != nil {
		return nil, err
	}
	var expired []ExpiredGuest
	for rows.Next() {
		var g ExpiredGuest
		if err := rows.Scan(&g.UserID, &g.Label, &g.Name); err != nil {
			rows.Close()
			return nil, err
		}
		expired = append(expired, g)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(expired) == 0 {
		return nil, nil
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM users WHERE is_guest = 1 AND expires_at IS NOT NULL AND expires_at < ?`,
		now.Unix()); err != nil {
		return nil, err
	}
	return expired, nil
}

// UpdateGuestSchedule patches a guest's label / sign_dates / expires_at /
// dorm binding. Only touches guest records (is_guest=1).
func (s *Store) UpdateGuestSchedule(ctx context.Context, userID, label, signDates string, expiresAt *time.Time) error {
	var exp any
	if expiresAt != nil {
		exp = expiresAt.Unix()
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE users SET
  guest_label = ?,
  sign_dates  = ?,
  expires_at  = ?,
  updated_at  = ?
WHERE user_id = ? AND is_guest = 1
`, label, signDates, exp, time.Now().Unix(), userID)
	return err
}

// DeleteUser removes a user; their records cascade via FK.
// Their invite_code is freed (set bound_user_id back to NULL — wait, design says permanent.
// We keep the binding so the user can re-activate the same code).
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE user_id = ?`, userID)
	return err
}

// DeleteUserAndUnbindCode also unbinds the invite code, freeing it for someone else.
func (s *Store) DeleteUserAndUnbindCode(ctx context.Context, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE invite_codes SET bound_user_id = NULL, bound_at = NULL WHERE bound_user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
