package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkProfile(t *testing.T, dir string, withSettings bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if withSettings {
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	} else if err := os.MkdirAll(filepath.Join(dir, "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	mkProfile(t, filepath.Join(home, ".claude"), true)
	mkProfile(t, filepath.Join(home, ".claude-work"), false)
	mkProfile(t, filepath.Join(home, "profiles", "side"), true)
	if err := os.MkdirAll(filepath.Join(home, ".claude-empty"), 0o700); err != nil { // not a profile
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	wrapper := "#!/bin/sh\nexport CLAUDE_CONFIG_DIR=\"$HOME/.claude-work\"\nexec claude \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "claude-work"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	rc := "alias side='CLAUDE_CONFIG_DIR=~/profiles/side claude'\n# CLAUDE_CONFIG_DIR=~/.claude-commented\n"
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte(rc), 0o600); err != nil {
		t.Fatal(err)
	}

	got := Discover(home, []string{bin}, "")
	var names []string
	for _, p := range got {
		names = append(names, p.Name())
	}
	if strings.Join(names, ",") != "claude,claude-work,side" {
		t.Fatalf("profiles = %v", names)
	}
	if !got[0].Default {
		t.Fatalf("~/.claude not marked default")
	}
	if len(got[1].Wrappers) != 1 || got[1].Wrappers[0] != "claude-work" {
		t.Fatalf("wrappers = %v", got[1].Wrappers)
	}
	if len(got[2].Wrappers) != 1 || got[2].Wrappers[0] != "~/.zshrc" {
		t.Fatalf("rc wrapper = %v", got[2].Wrappers)
	}
}

func TestProfileEnv(t *testing.T) {
	env := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/old"}
	def := profileEnv(env, Profile{Dir: "/h/.claude", Default: true})
	if strings.Join(def, " ") != "PATH=/bin" {
		t.Fatalf("default env = %v", def)
	}
	other := profileEnv(env, Profile{Dir: "/h/.claude-x"})
	if strings.Join(other, " ") != "PATH=/bin CLAUDE_CONFIG_DIR=/h/.claude-x" {
		t.Fatalf("profile env = %v", other)
	}
}

func TestInstallRunsMarketplaceThenPlugin(t *testing.T) {
	var calls []string
	run := func(_ context.Context, p Profile, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if args[1] == "marketplace" && args[2] == "add" {
			return "Marketplace 'cestafet' is already installed", errors.New("exit 1")
		}
		return "ok", nil
	}
	if err := Install(context.Background(), run, Profile{Dir: "/h/.claude-x"}, "orkono/cestafet"); err != nil {
		t.Fatal(err)
	}
	want := "plugin marketplace add orkono/cestafet|plugin marketplace update cestafet|plugin install cestafet@cestafet"
	if strings.Join(calls, "|") != want {
		t.Fatalf("calls = %v", calls)
	}
}

func TestInstallReportsRealFailure(t *testing.T) {
	run := func(_ context.Context, p Profile, args ...string) (string, error) {
		return "network unreachable", errors.New("exit 1")
	}
	err := Install(context.Background(), run, Profile{Dir: "/h/.claude-x"}, "orkono/cestafet")
	if err == nil || !strings.Contains(err.Error(), "network unreachable") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallUpdatesAnAlreadyInstalledPlugin(t *testing.T) {
	var calls []string
	run := func(_ context.Context, p Profile, args ...string) (string, error) {
		c := strings.Join(args, " ")
		calls = append(calls, c)
		if c == "plugin install cestafet@cestafet" {
			return "Plugin cestafet@cestafet is already installed", errors.New("exit 1")
		}
		return "ok", nil
	}
	if err := Install(context.Background(), run, Profile{Dir: "/h/.claude-x"}, "orkono/cestafet"); err != nil {
		t.Fatal(err)
	}
	if last := calls[len(calls)-1]; last != "plugin update cestafet@cestafet" {
		t.Fatalf("calls = %v", calls)
	}
}
