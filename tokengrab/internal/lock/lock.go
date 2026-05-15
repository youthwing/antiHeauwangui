// Package lock provides a single-instance file lock so two copies of the GUI
// can't fight each other over the system proxy / cert store.
package lock

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type Lock struct {
	f *flock.Flock
}

// Acquire takes the per-user lockfile. Returns an error if another instance
// already holds it.
func Acquire() (*Lock, error) {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir cache: %w", err)
	}
	path := filepath.Join(dir, "wangui-tokengrab.lock")
	fl := flock.New(path)
	ok, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("另一个 Token Grab 实例已经在运行")
	}
	return &Lock{f: fl}, nil
}

func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	_ = l.f.Unlock()
}
