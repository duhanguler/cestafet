//go:build !windows

package migrate

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// lock takes an exclusive advisory lock on path, waiting for it. The OS drops
// it when the process exits, so a crash never leaves a stale lock.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open migration lock: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("take migration lock: %w", err)
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
