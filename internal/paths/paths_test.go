package paths

import "testing"

func TestCanon(t *testing.T) {
	unix := map[string]string{
		"/Users/u/app/":    "/Users/u/app",
		"/Users/u//app/./": "/Users/u/app",
		"/":                "/",
	}
	for in, want := range unix {
		if got := canon(in, posix); got != want {
			t.Errorf("posix canon(%q) = %q, want %q", in, got, want)
		}
	}
	win := map[string]string{
		`C:\Users\u\ce-test`:      `C:\Users\u\ce-test`,
		`C:/Users/u/ce-test`:      `C:\Users\u\ce-test`, // git rev-parse --show-toplevel
		`c:\Users\u\ce-test\`:     `C:\Users\u\ce-test`,
		`/c/Users/u/ce-test`:      `C:\Users\u\ce-test`, // Git Bash
		`/C/Users/u`:              `C:\Users\u`,
		`C:\`:                     `C:\`,
		`\\server\share\dir\..\x`: `\\server\share\x`,
		`D:\Projects\App`:         `D:\Projects\App`, // spelling kept, only compared case-insensitively
		"":                        "",                // no path (e.g. no git root) stays empty
	}
	for in, want := range win {
		if got := canon(in, windows); got != want {
			t.Errorf("windows canon(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithinAndSame(t *testing.T) {
	cases := []struct {
		st           style
		path, parent string
		within, same bool
	}{
		{posix, "/w/app/docs", "/w/app", true, false},
		{posix, "/w/app", "/w/app", true, true},
		{posix, "/w/application", "/w/app", false, false},
		{posix, "/W/app/docs", "/w/app", false, false}, // case-sensitive on posix
		{posix, "/w", "/", true, false},
		{windows, `C:\Users\u\app\docs`, `C:\Users\u\app`, true, false},
		{windows, `c:\users\U\APP\docs`, `C:\Users\u\app`, true, false},
		{windows, `C:\Users\u\app`, `c:\users\u\APP`, true, true},
		{windows, `C:\Users\u\application`, `C:\Users\u\app`, false, false},
		{windows, `D:\Users\u\app`, `C:\Users\u`, false, false}, // other volume
		{windows, `C:\x`, `C:\`, true, false},
		{windows, `\\srv\share\a`, `\\srv\share`, true, false},
	}
	for _, c := range cases {
		if got := within(c.path, c.parent, c.st); got != c.within {
			t.Errorf("within(%q, %q, %v) = %v", c.path, c.parent, c.st, got)
		}
		if got := same(c.path, c.parent, c.st); got != c.same {
			t.Errorf("same(%q, %q, %v) = %v", c.path, c.parent, c.st, got)
		}
	}
}

func TestIsRootAndIsPath(t *testing.T) {
	for p, want := range map[string]bool{"/": true, "/w": false} {
		if got := isRoot(p, posix); got != want {
			t.Errorf("posix isRoot(%q) = %v", p, got)
		}
	}
	for p, want := range map[string]bool{`C:\`: true, `c:\`: true, `C:\w`: false, `\\srv\share`: true, `\\srv\share\a`: false} {
		if got := isRoot(p, windows); got != want {
			t.Errorf("windows isRoot(%q) = %v", p, got)
		}
	}
	for p, want := range map[string]bool{`C:\w`: true, `\\srv\share\a`: true, "github.com/o/r": false, "/w": false} {
		if got := isPath(p, windows); got != want {
			t.Errorf("windows isPath(%q) = %v", p, got)
		}
	}
	for p, want := range map[string]bool{"/w": true, "github.com/o/r": false} {
		if got := isPath(p, posix); got != want {
			t.Errorf("posix isPath(%q) = %v", p, got)
		}
	}
}
