//go:build windows

package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	win "golang.org/x/sys/windows"
)

func shortName(t *testing.T, p string) string {
	t.Helper()
	in, _ := win.UTF16PtrFromString(p)
	buf := make([]uint16, win.MAX_PATH)
	n, err := win.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Skipf("no short name: %v", err)
	}
	s := win.UTF16ToString(buf[:n])
	if !strings.Contains(s, "~") {
		t.Skip("8.3 name generation is off on this volume")
	}
	return s
}

func TestShortNameIsTheSameKey(t *testing.T) {
	long := filepath.Join(t.TempDir(), "a long directory name")
	if err := os.Mkdir(long, 0o700); err != nil {
		t.Fatal(err)
	}
	short := shortName(t, long)
	if !Same(short, long) {
		t.Fatalf("Same(%q, %q) = false", short, long)
	}
	if Canon(short) != Canon(long) {
		t.Fatalf("Canon: %q vs %q", Canon(short), Canon(long))
	}
	if !Within(filepath.Join(short, "sub"), long) {
		t.Fatal("Within through a short parent")
	}
}

func TestMissingShortPathStaysAsIs(t *testing.T) {
	p := `C:\NOSUCH~1\x`
	if Canon(p) != p {
		t.Fatalf("Canon(%q) = %q", p, Canon(p))
	}
}
