package identity

import (
	"path/filepath"
	"strings"
	"time"
)

// Proc is one process in an ancestor chain.
type Proc struct {
	PID     int       `json:"pid"`
	PPID    int       `json:"ppid"`
	Name    string    `json:"name"`
	Created time.Time `json:"created,omitzero"`
}

// Chain returns pid and its ancestors, nearest first, at most max entries. It
// stops at the first process it cannot read. On Windows a parent created after
// its child is a reused pid, and the walk stops there.
func Chain(pid, max int) []Proc {
	var out []Proc
	for len(out) < max && pid > 0 {
		p, ok := procInfo(pid)
		if !ok {
			break
		}
		if n := len(out); n > 0 && !p.Created.IsZero() && !out[n-1].Created.IsZero() &&
			p.Created.After(out[n-1].Created) {
			break
		}
		out = append(out, p)
		if p.PPID == pid {
			break
		}
		pid = p.PPID
	}
	return out
}

// IsClaude reports whether a process name or path is the Claude Code
// executable: claude, or claude.exe on Windows, case-insensitively.
func IsClaude(name string) bool {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	return base == "claude" || base == "claude.exe"
}
