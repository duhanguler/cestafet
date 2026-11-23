package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractConfigDirs(t *testing.T) {
	cases := map[string][]string{
		`export CLAUDE_CONFIG_DIR="$HOME/.claude-work"`:                     {"$HOME/.claude-work"},
		`CLAUDE_CONFIG_DIR=~/.claude-x exec claude "$@"`:                    {"~/.claude-x"},
		`$env:CLAUDE_CONFIG_DIR = "C:\Users\Osman Hömek\.claude-work"`:      {`C:\Users\Osman Hömek\.claude-work`},
		`$env:CLAUDE_CONFIG_DIR='$env:USERPROFILE\.claude-a'; claude @args`: {`$env:USERPROFILE\.claude-a`},
		`set "CLAUDE_CONFIG_DIR=%USERPROFILE%\.claude-b"`:                   {`%USERPROFILE%\.claude-b`},
		`set CLAUDE_CONFIG_DIR=%USERPROFILE%\.claude-c`:                     {`%USERPROFILE%\.claude-c`},
		`setx CLAUDE_CONFIG_DIR "D:\profiles\work"`:                         {`D:\profiles\work`},
		`REM set CLAUDE_CONFIG_DIR=C:\nope`:                                 nil,
		`:: set CLAUDE_CONFIG_DIR=C:\nope`:                                  nil,
		`# export CLAUDE_CONFIG_DIR=/nope`:                                  nil,
		`echo hello`:                                                        nil,
	}
	for line, want := range cases {
		got := extract(line)
		if len(got) != len(want) || (len(want) > 0 && got[0] != want[0]) {
			t.Errorf("extract(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestExpandVariables(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Osman Hömek")
	t.Setenv("CESTAFET_TEST_PROFILE_ROOT", home)
	cases := map[string]string{
		"$HOME/.claude-a":                           filepath.Join(home, ".claude-a"),
		"${HOME}/.claude-a":                         filepath.Join(home, ".claude-a"),
		"~/.claude-a":                               filepath.Join(home, ".claude-a"),
		"%CESTAFET_TEST_PROFILE_ROOT%/.claude-b":    filepath.Join(home, ".claude-b"),
		"$env:CESTAFET_TEST_PROFILE_ROOT/.claude-c": filepath.Join(home, ".claude-c"),
		"%CESTAFET_UNSET_VAR_XYZ%/.claude-d":        "",
		"relative/dir":                              "",
	}
	for in, want := range cases {
		if got := expand(in, home); got != want {
			t.Errorf("expand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanCandidateSkipsBinariesByName(t *testing.T) {
	for name, want := range map[string]bool{
		"claude-work": true, "claude-work.cmd": true, "c.bat": true, "c.ps1": true, "c.sh": true,
		"git.exe": false, "lib.dll": false, "notes.txt": false,
	} {
		if got := scanCandidate(name); got != want {
			t.Errorf("scanCandidate(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestDiscoverFindsPowerShellProfile(t *testing.T) {
	home := t.TempDir()
	prof := filepath.Join(home, "profiles", "work dir")
	mkProfile(t, prof, true)
	ps := filepath.Join(home, "Documents", "PowerShell")
	if err := os.MkdirAll(ps, 0o700); err != nil {
		t.Fatal(err)
	}
	line := "function claude-work { $env:CLAUDE_CONFIG_DIR = \"" + prof + "\"; claude @args }\n"
	if err := os.WriteFile(filepath.Join(ps, "Microsoft.PowerShell_profile.ps1"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	found := Discover(home, nil, "")
	var ok bool
	for _, p := range found {
		if p.Dir == prof {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("profile with a space from the PowerShell profile not found: %+v", found)
	}
}
