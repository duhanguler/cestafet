// Package paths gives filesystem paths one spelling when they are used as keys
// (projects, mappings) and compares them the way the platform does. On darwin
// and linux it is filepath.Clean and plain string comparison, as before; on
// Windows a path from Git Bash (/c/...), from git (C:/...) and from Claude Code
// (C:\...) are the same key, compared case-insensitively.
package paths

import (
	"path"
	"regexp"
	"runtime"
	"strings"
)

type style int

const (
	posix style = iota
	windows
)

func (s style) String() string {
	if s == windows {
		return "windows"
	}
	return "posix"
}

func host() style {
	if runtime.GOOS == "windows" {
		return windows
	}
	return posix
}

// Canon returns the canonical spelling of a filesystem path. On Windows an
// existing path's 8.3 short names (PROGRA~1) are expanded to their long names.
func Canon(p string) string { return canon(expand(p), host()) }

// Same reports whether two paths name the same location.
func Same(a, b string) bool { return same(expand(a), expand(b), host()) }

// Within reports whether p is parent or a path below it.
func Within(p, parent string) bool { return within(expand(p), expand(parent), host()) }

// expand returns p with 8.3 short names replaced by long names. It is the
// identity on darwin and linux, for paths without a "~", and for paths that do
// not exist; the pure helpers below never touch the filesystem.
func expand(p string) string {
	if host() != windows || !strings.Contains(p, "~") {
		return p
	}
	return longName(canon(p, windows))
}

// IsRoot reports whether p is a filesystem root: "/", a drive root (C:\) or a
// UNC share root (\\server\share).
func IsRoot(p string) bool { return isRoot(p, host()) }

// IsPath reports whether a project key is an absolute path rather than a
// remote ("github.com/o/r") or a named project.
func IsPath(key string) bool { return isPath(key, host()) }

var gitBashDrive = regexp.MustCompile(`^/([A-Za-z])(/|$)`)
var driveVolume = regexp.MustCompile(`^([A-Za-z]):`)

func canon(p string, st style) string {
	if p == "" {
		return "" // no path (e.g. outside git) is not the current directory
	}
	if st == posix {
		return path.Clean(p)
	}
	s := strings.ReplaceAll(p, `\`, "/")
	if m := gitBashDrive.FindStringSubmatch(s); m != nil {
		s = strings.ToUpper(m[1]) + ":/" + s[len(m[0]):]
	}
	vol, rest := splitVolume(s)
	if vol == "" {
		return strings.ReplaceAll(path.Clean(s), "/", `\`)
	}
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	rest = path.Clean(rest)
	if strings.HasPrefix(vol, "//") && rest == "/" {
		rest = ""
	}
	return strings.ReplaceAll(vol+rest, "/", `\`)
}

// splitVolume splits a slash-form Windows path into its volume ("C:" upper-cased,
// or "//server/share") and the rest.
func splitVolume(s string) (string, string) {
	if m := driveVolume.FindStringSubmatch(s); m != nil {
		return strings.ToUpper(m[1]) + ":", s[2:]
	}
	if strings.HasPrefix(s, "//") {
		parts := strings.SplitN(s[2:], "/", 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			vol := "//" + parts[0] + "/" + parts[1]
			if len(parts) == 3 {
				return vol, "/" + parts[2]
			}
			return vol, ""
		}
	}
	return "", s
}

func fold(p string, st style) string {
	c := canon(p, st)
	if st == windows {
		return strings.ToLower(c)
	}
	return c
}

func same(a, b string, st style) bool { return fold(a, st) == fold(b, st) }

func within(p, parent string, st style) bool {
	a, b := fold(p, st), fold(parent, st)
	if a == b {
		return true
	}
	sep := "/"
	if st == windows {
		sep = `\`
	}
	if !strings.HasSuffix(b, sep) {
		b += sep
	}
	return strings.HasPrefix(a, b)
}

func isRoot(p string, st style) bool {
	c := canon(p, st)
	if st == posix {
		return c == "/"
	}
	vol, rest := splitVolume(strings.ReplaceAll(c, `\`, "/"))
	return vol != "" && (rest == "" || rest == "/")
}

func isPath(key string, st style) bool {
	if st == posix {
		return strings.HasPrefix(key, "/")
	}
	vol, _ := splitVolume(strings.ReplaceAll(key, `\`, "/"))
	return vol != ""
}
