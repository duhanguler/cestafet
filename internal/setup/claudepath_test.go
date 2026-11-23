package setup

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func finder(onPath, ancestor string, globbed []string) Finder {
	return Finder{
		LookPath: func(string) (string, error) {
			if onPath == "" {
				return "", errors.New("not found")
			}
			return onPath, nil
		},
		Ancestor: func() string { return ancestor },
		Glob: func(pat string) ([]string, error) {
			var out []string
			for _, g := range globbed {
				if ok, _ := filepath.Match(pat, g); ok {
					out = append(out, g)
				}
			}
			return out, nil
		},
		Profile: "/u",
		Verify:  func(p string) bool { return !strings.Contains(p, "broken") },
		Windows: true,
	}
}

func TestFindClaudePrefersPath(t *testing.T) {
	got := FindClaude(finder("/bin/claude", "/x/claude.exe", nil))
	if got.Path != "/bin/claude" || got.Source != "PATH" {
		t.Fatalf("got %+v", got)
	}
}

func TestFindClaudeUsesSessionAncestorButNotTheAppItself(t *testing.T) {
	got := FindClaude(finder("", "/x/claude-code/2.1.330/ab/claude.exe", nil))
	if got.Source != "this session" {
		t.Fatalf("got %+v", got)
	}
	got = FindClaude(finder("", `C:\Program Files\WindowsApps\Claude_1\app\claude.exe`, nil))
	if got.Path != "" {
		t.Fatalf("the desktop app's own executable was used: %+v", got)
	}
}

func TestFindClaudePicksNewestDesktopVersionNumerically(t *testing.T) {
	base := filepath.Join("/u", "AppData", "Local", "Claude-Data", "claude_x", "claude-code")
	g := []string{
		filepath.Join(base, "2.1.99", "a", "claude.exe"),
		filepath.Join(base, "2.1.330", "b", "claude.exe"),
		filepath.Join(base, "2.1.400", "broken", "claude.exe"),
	}
	got := FindClaude(finder("", "", g))
	if got.Path != g[1] || got.Source != "Claude desktop app" {
		t.Fatalf("got %+v", got)
	}
}

func TestFindClaudeOutsideWindowsIsPathOnly(t *testing.T) {
	f := finder("", "/x/claude", nil)
	f.Windows = false
	if got := FindClaude(f); got.Path != "" {
		t.Fatalf("got %+v", got)
	}
}
