package store

import (
	"context"
	"database/sql"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Dorm represents one row of dorm_locations — an admin-curated checkin location.
type Dorm struct {
	ID                int64
	Name              string
	Latitude          float64
	Longitude         float64
	Address           string
	City              string
	Road              string
	Poi               string
	Note              string
	SendAddressFields bool // if true, sign requests for users bound here include address/city/road/poi
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

const dormColumns = `id, name, latitude, longitude, address, city, road, poi, note, send_address_fields, created_at, updated_at`

func (s *Store) CreateDorm(ctx context.Context, d *Dorm) error {
	now := time.Now().Unix()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Unix(now, 0)
	}
	d.UpdatedAt = time.Unix(now, 0)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO dorm_locations (name, latitude, longitude, address, city, road, poi, note, send_address_fields, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?)
`, d.Name, d.Latitude, d.Longitude, d.Address, d.City, d.Road, d.Poi, d.Note,
		boolInt(d.SendAddressFields), d.CreatedAt.Unix(), d.UpdatedAt.Unix())
	if err != nil {
		return err
	}
	d.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) GetDorm(ctx context.Context, id int64) (*Dorm, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+dormColumns+" FROM dorm_locations WHERE id = ?", id)
	return s.scanDormRow(row)
}

func (s *Store) ListDorms(ctx context.Context) ([]*Dorm, error) {
	// SQLite's ORDER BY name is lexicographic, which sorts "10号楼" before
	// "2号楼". We want natural order — numeric chunks compared as numbers,
	// so 1, 2, ..., 9, 10, 11, 12. SQLite has no built-in NUMERIC collation,
	// so we pull all rows and re-sort in Go.
	rows, err := s.db.QueryContext(ctx, "SELECT "+dormColumns+" FROM dorm_locations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Dorm, 0)
	for rows.Next() {
		d, err := s.scanDormRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b *Dorm) int {
		return naturalCompare(a.Name, b.Name)
	})
	return out, nil
}

// naturalCompare is a string comparator that treats runs of ASCII digits as
// integers. "1号楼" < "2号楼" < "10号楼". Returns -1/0/+1 per slices.SortFunc.
// Non-ASCII characters compare by byte; that's fine for our use because the
// numeric prefix is what disambiguates dorm names.
func naturalCompare(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ad := a[ai] >= '0' && a[ai] <= '9'
		bd := b[bi] >= '0' && b[bi] <= '9'
		if ad && bd {
			aEnd := ai
			for aEnd < len(a) && a[aEnd] >= '0' && a[aEnd] <= '9' {
				aEnd++
			}
			bEnd := bi
			for bEnd < len(b) && b[bEnd] >= '0' && b[bEnd] <= '9' {
				bEnd++
			}
			an, _ := strconv.Atoi(a[ai:aEnd])
			bn, _ := strconv.Atoi(b[bi:bEnd])
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
			ai, bi = aEnd, bEnd
			continue
		}
		if a[ai] != b[bi] {
			if a[ai] < b[bi] {
				return -1
			}
			return 1
		}
		ai++
		bi++
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// UpdateDorm patches a dorm record. Only non-nil pointer fields are touched.
func (s *Store) UpdateDorm(ctx context.Context, id int64,
	name *string, lat, lng *float64, address, city, road, poi, note *string,
	sendAddressFields *bool) error {
	sets := []string{}
	args := []any{}
	if name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *name)
	}
	if lat != nil {
		sets = append(sets, "latitude = ?")
		args = append(args, *lat)
	}
	if lng != nil {
		sets = append(sets, "longitude = ?")
		args = append(args, *lng)
	}
	if address != nil {
		sets = append(sets, "address = ?")
		args = append(args, *address)
	}
	if city != nil {
		sets = append(sets, "city = ?")
		args = append(args, *city)
	}
	if road != nil {
		sets = append(sets, "road = ?")
		args = append(args, *road)
	}
	if poi != nil {
		sets = append(sets, "poi = ?")
		args = append(args, *poi)
	}
	if note != nil {
		sets = append(sets, "note = ?")
		args = append(args, *note)
	}
	if sendAddressFields != nil {
		sets = append(sets, "send_address_fields = ?")
		args = append(args, boolInt(*sendAddressFields))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().Unix())
	args = append(args, id)
	_, err := s.db.ExecContext(ctx,
		"UPDATE dorm_locations SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	return err
}

// DeleteDorm removes a dorm. Caller should check no users reference it first.
func (s *Store) DeleteDorm(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM dorm_locations WHERE id = ?`, id)
	return err
}

// CountDormUsers returns the number of users currently bound to a dorm.
func (s *Store) CountDormUsers(ctx context.Context, id int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE dorm_id = ?`, id).Scan(&n)
	return n, err
}

// CountDorms returns total number of dorms (admin stats).
func (s *Store) CountDorms(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dorm_locations`).Scan(&n)
	return n, err
}

func (s *Store) scanDormRow(row *sql.Row) (*Dorm, error) {
	var d Dorm
	var createdAt, updatedAt int64
	var sendFields int
	err := row.Scan(
		&d.ID, &d.Name, &d.Latitude, &d.Longitude,
		&d.Address, &d.City, &d.Road, &d.Poi, &d.Note,
		&sendFields, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.SendAddressFields = sendFields != 0
	d.CreatedAt = time.Unix(createdAt, 0)
	d.UpdatedAt = time.Unix(updatedAt, 0)
	return &d, nil
}

func (s *Store) scanDormRows(rows *sql.Rows) (*Dorm, error) {
	var d Dorm
	var createdAt, updatedAt int64
	var sendFields int
	if err := rows.Scan(
		&d.ID, &d.Name, &d.Latitude, &d.Longitude,
		&d.Address, &d.City, &d.Road, &d.Poi, &d.Note,
		&sendFields, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	d.SendAddressFields = sendFields != 0
	d.CreatedAt = time.Unix(createdAt, 0)
	d.UpdatedAt = time.Unix(updatedAt, 0)
	return &d, nil
}
