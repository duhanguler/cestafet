package setup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/orkono/cestafet/internal/identity"
)

// ClaudeExe is how the claude CLI was found.
type ClaudeExe struct {
	Path   string // empty when not found
	Source string // "PATH", "this session", "Claude desktop app"
}

// Finder holds the lookups FindClaude needs; replaced in tests.
type Finder struct {
	LookPath func(string) (string, error)
	Ancestor func() string // full path of this process's Claude Code, or ""
	Glob     func(string) ([]string, error)
	Profile  string                 // the user's home directory
	Verify   func(path string) bool // runs path --version
	Windows  bool
}

// FindClaude returns the claude CLI to run: claude on PATH; else (Windows) the
// Claude Code that started this process; else the newest Claude Code shipped
// by the Claude desktop app. The desktop app's own executable (under
// \WindowsApps\) is never used, and every candidate must answer --version.
func FindClaude(f Finder) ClaudeExe {
	if p, err := f.LookPath("claude"); err == nil {
		return ClaudeExe{Path: p, Source: "PATH"}
	}
	if !f.Windows {
		return ClaudeExe{}
	}
	if p := f.Ancestor(); usable(p) && f.Verify(p) {
		return ClaudeExe{Path: p, Source: "this session"}
	}
	for _, p := range desktopCandidates(f) {
		if f.Verify(p) {
			return ClaudeExe{Path: p, Source: "Claude desktop app"}
		}
	}
	return ClaudeExe{}
}

func usable(p string) bool {
	return p != "" && !strings.Contains(strings.ToLower(p), `\windowsapps\`)
}

// desktopCandidates lists the app's claude.exe files, newest version first.
func desktopCandidates(f Finder) []string {
	local := filepath.Join(f.Profile, "AppData", "Local")
	patterns := []string{
		filepath.Join(local, "Claude-Data", "*", "claude-code", "*", "*", "claude.exe"),
		filepath.Join(local, "Packages", "Claude_*", "LocalCache", "Local", "Claude-Data", "*", "claude-code", "*", "*", "claude.exe"),
	}
	var found []string
	for _, pat := range patterns {
		m, _ := f.Glob(pat)
		for _, p := range m {
			if usable(p) {
				found = append(found, p)
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		return versionLess(versionOf(found[j]), versionOf(found[i]))
	})
	return found
}

// versionOf is the claude-code/<version>/<id>/claude.exe version folder.
func versionOf(p string) string { return filepath.Base(filepath.Dir(filepath.Dir(p))) }

// versionLess compares dotted versions numerically; non-numeric parts sort first.
func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}

var (
	claudeOnce sync.Once
	claudeExe  ClaudeExe
)

// Claude finds the claude CLI once per process.
func Claude() ClaudeExe {
	claudeOnce.Do(func() {
		home, _ := os.UserHomeDir()
		claudeExe = FindClaude(Finder{
			LookPath: exec.LookPath,
			Ancestor: func() string {
				if c := identity.Claude(); c.PID > 0 {
					return identity.ExePath(c.PID)
				}
				return ""
			},
			Glob:    filepath.Glob,
			Profile: home,
			Verify:  verifyClaude,
			Windows: runtime.GOOS == "windows",
		})
	})
	return claudeExe
}

func verifyClaude(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	identity.Quiet(cmd)
	out, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "claude")
}
