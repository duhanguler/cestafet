// Package identity derives who a session is: its profile, its repository and
// the claude process that owns it.
package identity

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultProfile is the profile name used when CLAUDE_CONFIG_DIR is unset.
const DefaultProfile = "claude"

// Profile returns the profile name: the base name of CLAUDE_CONFIG_DIR without
// a leading dot (".claude-orkono" -> "claude-orkono"), or "claude".
func Profile() string {
	return ProfileName(os.Getenv("CLAUDE_CONFIG_DIR"))
}

// ProfileName turns a config directory into a profile name.
func ProfileName(configDir string) string {
	dir := strings.TrimRight(configDir, "/")
	if dir == "" {
		return DefaultProfile
	}
	return strings.TrimPrefix(filepath.Base(dir), ".")
}

// Repo describes the git repository a directory belongs to.
type Repo struct {
	Root   string // absolute repo root; "" when not a git repo
	Remote string // normalized origin URL, e.g. github.com/orkono/x; "" when none
	Branch string
}

// gitTimeout bounds every git call so a hook never hangs a session.
const gitTimeout = 2 * time.Second

func git(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// LookupRepo inspects the git repository containing dir.
func LookupRepo(dir string) Repo {
	root := git(dir, "rev-parse", "--show-toplevel")
	if root == "" {
		return Repo{}
	}
	r := Repo{Root: root, Branch: git(root, "rev-parse", "--abbrev-ref", "HEAD")}
	if url := git(root, "remote", "get-url", "origin"); url != "" {
		r.Remote = NormalizeRemote(url, sshHostname)
	}
	return r
}

// NormalizeRemote turns a git remote URL into "host/owner/repo". SSH host
// aliases (e.g. "github-work" from ~/.ssh/config) are resolved through resolve.
func NormalizeRemote(url string, resolve func(alias string) string) string {
	u := strings.TrimSpace(url)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://", "git+ssh://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	if at := strings.Index(u, "@"); at >= 0 && at < strings.IndexAny(u+":/", ":/") {
		u = u[at+1:]
	}
	host, path := u, ""
	if i := strings.IndexAny(u, ":/"); i >= 0 {
		host, path = u[:i], strings.TrimLeft(u[i+1:], "/")
	}
	if colon := strings.Index(host, ":"); colon >= 0 { // host:port
		host = host[:colon]
	}
	if !strings.Contains(host, ".") && resolve != nil {
		if real := resolve(host); real != "" {
			host = real
		}
	}
	// a numeric port left in the path by "ssh://host:22/owner/repo"
	if first, rest, ok := strings.Cut(path, "/"); ok {
		if _, err := strconv.Atoi(first); err == nil {
			path = rest
		}
	}
	return strings.ToLower(host) + "/" + path
}

// sshHostname resolves an ~/.ssh/config alias to its real host name.
func sshHostname(alias string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-G", alias).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if h, ok := strings.CutPrefix(line, "hostname "); ok {
			return strings.TrimSpace(h)
		}
	}
	return ""
}

// Claude walks up the process tree from the current process and returns the
// Claude Code process that owns it: claude, or claude.exe on Windows, where
// hooks run through Git Bash (cestafet < bash < bash < claude.exe). The zero
// Proc means no Claude Code ancestor was found.
func Claude() Proc {
	return claudeAncestor(os.Getppid(), procInfo)
}

// ClaudePID is Claude().PID.
func ClaudePID() int { return Claude().PID }

func claudeAncestor(start int, info func(pid int) (Proc, bool)) Proc {
	pid := start
	var child Proc
	for depth := 0; pid > 1 && depth < 32; depth++ {
		p, ok := info(pid)
		if !ok {
			return Proc{}
		}
		if depth > 0 && !p.Created.IsZero() && !child.Created.IsZero() && p.Created.After(child.Created) {
			return Proc{} // the recorded parent pid was reused by a younger process
		}
		if IsClaude(p.Name) {
			return p
		}
		child, pid = p, p.PPID
	}
	return Proc{}
}
