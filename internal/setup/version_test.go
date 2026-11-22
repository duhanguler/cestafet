package setup

import (
	"context"
	"strings"
	"testing"
)

func TestPluginStateParsesJSONList(t *testing.T) {
	run := func(ctx context.Context, p Profile, args ...string) (string, error) {
		if strings.Join(args, " ") != "plugin list --json" {
			t.Fatalf("args = %v", args)
		}
		return `[{"id":"other@x","version":"9"},{"id":"cestafet@cestafet","version":"0.1.0","enabled":false}]`, nil
	}
	st, err := PluginState(context.Background(), run, Profile{Dir: "/p"})
	if err != nil || !st.Installed || st.Version != "0.1.0" || st.Enabled {
		t.Fatalf("state = %+v, %v", st, err)
	}
	none := func(context.Context, Profile, ...string) (string, error) { return `[]`, nil }
	if st, err := PluginState(context.Background(), none, Profile{Dir: "/p"}); err != nil || st.Installed {
		t.Fatalf("empty list: %+v, %v", st, err)
	}
}

func TestVersionWarnings(t *testing.T) {
	plugins := map[string]PluginInfo{
		"~/.claude":   {Installed: true, Enabled: true, Version: "0.1.1"},
		"~/.claude-a": {Installed: true, Enabled: true, Version: "0.1.0"},
		"~/.claude-b": {Installed: true, Enabled: false, Version: "0.1.1"},
		"~/.claude-c": {},
	}
	got := strings.Join(VersionWarnings("0.1.1", "/bin/cestafet", "/other/cestafet", plugins), "\n")
	for _, want := range []string{"~/.claude-a", "0.1.0", "~/.claude-b", "disabled", "/other/cestafet"} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "~/.claude-c") || strings.Contains(got, "~/.claude:") {
		t.Errorf("unexpected warning:\n%s", got)
	}
	if w := VersionWarnings("dev", "/bin/cestafet", "/bin/cestafet", plugins); len(w) != 1 {
		t.Errorf("dev build should only warn about the disabled plugin, got %v", w)
	}
}
