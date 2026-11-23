//go:build windows

package paths

import (
	"path/filepath"
	"strings"

	win "golang.org/x/sys/windows"
)

// longName expands 8.3 short names in p. When p does not exist (a file still
// to be created, a subfolder named only in a key) the longest existing prefix
// is expanded and the rest kept as written. Any error leaves p as it is.
func longName(p string) string {
	if l, ok := getLong(p); ok {
		return l
	}
	dir, base := filepath.Split(p)
	dir = strings.TrimRight(dir, `\`)
	if dir == "" || dir == p || len(dir) <= len(filepath.VolumeName(p))+1 {
		return p
	}
	return filepath.Join(longName(dir), base)
}

func getLong(p string) (string, bool) {
	in, err := win.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, win.MAX_PATH)
	for {
		n, err := win.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return "", false
		}
		if int(n) <= len(buf) {
			return strings.TrimPrefix(win.UTF16ToString(buf[:n]), `\\?\`), true
		}
		buf = make([]uint16, n)
	}
}
