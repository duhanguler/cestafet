//go:build windows

package migrate

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// lock takes an exclusive lock on path with LockFileEx, waiting for it. The OS
// releases it when the process exits, so a crash never leaves a stale lock.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open migration lock: %w", err)
	}
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("take migration lock: %w", err)
	}
	return func() {
		_ = windows.UnlockFileEx(h, 0, 1, 0, ol)
		_ = f.Close()
	}, nil
}
