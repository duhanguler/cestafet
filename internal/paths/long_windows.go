//go:build windows

package paths

import (
	"strings"

	win "golang.org/x/sys/windows"
)

// longName asks Windows for the long form of an existing path, falling back
// to the input on any error (missing path, access denied).
func longName(p string) string {
	in, err := win.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, win.MAX_PATH)
	for {
		n, err := win.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return p
		}
		if int(n) <= len(buf) {
			out := win.UTF16ToString(buf[:n])
			out = strings.TrimPrefix(out, `\\?\`)
			return out
		}
		buf = make([]uint16, n)
	}
}
