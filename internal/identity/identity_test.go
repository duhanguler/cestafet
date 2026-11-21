package identity

import "testing"

func TestNormalizeRemote(t *testing.T) {
	resolve := func(alias string) string {
		if alias == "github-work" {
			return "github.com"
		}
		return ""
	}
	cases := map[string]string{
		"git@github.com:orkono/cestafet.git":          "github.com/orkono/cestafet",
		"https://github.com/orkono/cestafet":          "github.com/orkono/cestafet",
		"https://user@GitHub.com/orkono/cestafet.git": "github.com/orkono/cestafet",
		"ssh://git@gitlab.example.com:22/team/app":    "gitlab.example.com/team/app",
		"git@github-work:orkono/cestafet.git":         "github.com/orkono/cestafet",
		"git@unknown-alias:team/app.git":              "unknown-alias/team/app",
	}
	for in, want := range cases {
		if got := NormalizeRemote(in, resolve); got != want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProfileName(t *testing.T) {
	cases := map[string]string{
		"":                         "claude",
		"/Users/x/.claude-orkono":  "claude-orkono",
		"/Users/x/.claude-orkono/": "claude-orkono",
		"/opt/profiles/work":       "work",
	}
	for in, want := range cases {
		if got := ProfileName(in); got != want {
			t.Errorf("ProfileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClaudeAncestor(t *testing.T) {
	tree := map[int]struct {
		ppid int
		comm string
	}{
		40: {39, "/bin/bash"},
		39: {38, "/Users/x/.local/bin/claude"},
		38: {1, "zsh"},
	}
	info := func(pid int) (int, string, bool) {
		n, ok := tree[pid]
		return n.ppid, n.comm, ok
	}
	if got := claudeAncestor(40, info); got != 39 {
		t.Fatalf("ancestor = %d, want 39", got)
	}
	if got := claudeAncestor(38, info); got != 0 {
		t.Fatalf("ancestor without claude = %d, want 0", got)
	}
}
