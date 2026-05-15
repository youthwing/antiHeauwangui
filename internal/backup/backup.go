// Package backup performs daily hot backups of the SQLite database using
// VACUUM INTO (consistent snapshot, no exclusive lock).
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// HourLocal is the local-time hour at which the daily backup runs.
	HourLocal = 23
	// KeepCount is how many recent backups to retain.
	KeepCount = 7
	// FilePrefix prefixes every backup file name.
	FilePrefix = "wangui-"
	FileSuffix = ".db"
)

// Backup wires the daily backup goroutine to a *sql.DB and a destination dir.
type Backup struct {
	DB  *sql.DB
	Dir string
	Log *slog.Logger
}

// Start launches the backup loop until ctx is cancelled.
func (b *Backup) Start(ctx context.Context) {
	if b.Log == nil {
		b.Log = slog.Default()
	}
	go b.loop(ctx)
}

func (b *Backup) loop(ctx context.Context) {
	if err := os.MkdirAll(b.Dir, 0o700); err != nil {
		b.Log.Error("backup dir create failed", "err", err.Error())
		return
	}
	for {
		next := nextRun(time.Now())
		b.Log.Info("backup armed", "next", next.Format(time.RFC3339))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
		if err := b.RunOnce(ctx); err != nil {
			b.Log.Error("backup failed", "err", err.Error())
		}
	}
}

func nextRun(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), HourLocal, 0, 0, 0, now.Location())
	if today.After(now) {
		return today
	}
	return today.Add(24 * time.Hour)
}

// RunOnce performs a single backup + GC cycle. Exported for manual triggering / testing.
func (b *Backup) RunOnce(ctx context.Context) error {
	now := time.Now()
	dst := filepath.Join(b.Dir, FilePrefix+now.Format("20060102-150405")+FileSuffix)
	// VACUUM INTO produces a consistent snapshot file with no locks held on the source.
	if _, err := b.DB.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escapeSQLPath(dst))); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	// Restrict permissions on the snapshot file.
	_ = os.Chmod(dst, 0o600)
	b.Log.Info("backup written", "file", dst)
	return b.gc()
}

// gc removes backups older than KeepCount.
func (b *Backup) gc() error {
	entries, err := os.ReadDir(b.Dir)
	if err != nil {
		return err
	}
	type pick struct {
		name    string
		modTime time.Time
	}
	var files []pick
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, FilePrefix) || !strings.HasSuffix(name, FileSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, pick{name: name, modTime: info.ModTime()})
	}
	if len(files) <= KeepCount {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	for _, f := range files[KeepCount:] {
		p := filepath.Join(b.Dir, f.name)
		if err := os.Remove(p); err != nil {
			b.Log.Warn("backup gc remove failed", "file", p, "err", err.Error())
			continue
		}
		b.Log.Info("backup gc removed", "file", p)
	}
	return nil
}

// escapeSQLPath escapes single quotes in a path so it can be embedded
// in a SQL string literal. SQLite uses '' to escape a single quote.
func escapeSQLPath(p string) string {
	return strings.ReplaceAll(p, "'", "''")
}
