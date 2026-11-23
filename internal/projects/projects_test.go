package projects

import (
	"path/filepath"
	"testing"

	"github.com/orkono/cestafet/internal/identity"
)

func loc(remote, root, cwd string) Location {
	return Location{Cwd: cwd, Repo: identity.Repo{Remote: remote, Root: root}}
}

func TestResolveAutomatic(t *testing.T) {
	c := Config{Projects: map[string]Project{}}
	if got := c.Resolve(loc("github.com/o/a", "/w/a", "/w/a/sub")); got != "github.com/o/a" {
		t.Errorf("remote: %q", got)
	}
	if got := c.Resolve(loc("", "/w/a", "/w/a/sub")); got != "/w/a" {
		t.Errorf("root: %q", got)
	}
	if got := c.Resolve(loc("", "", "/tmp/x")); got != "/tmp/x" {
		t.Errorf("cwd: %q", got)
	}
}

func TestAddGroupsSeveralReposIntoOneProject(t *testing.T) {
	c := Config{Projects: map[string]Project{}}
	backend := loc("github.com/t/backend", "/w/backend", "/w/backend")
	ui := loc("github.com/t/ui", "/w/ui", "/w/ui")
	plain := loc("", "", "/w/notes")
	var err error
	for _, l := range []Location{backend, ui, plain} {
		if c, err = c.Add("tonton", l); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range []Location{backend, ui, loc("", "", "/w/notes/deep")} {
		if got := c.Resolve(l); got != "tonton" {
			t.Errorf("Resolve(%+v) = %q, want tonton", l, got)
		}
	}
	if m := c.Members("tonton"); len(m) != 3 {
		t.Errorf("members = %v", m)
	}
}

func TestAddMovesRepoBetweenProjects(t *testing.T) {
	c := Config{Projects: map[string]Project{}}
	l := loc("github.com/t/backend", "/w/backend", "/w/backend")
	c, _ = c.Add("old", l)
	c2, _ := c.Add("new", l)
	if got := c2.Resolve(l); got != "new" {
		t.Fatalf("resolve = %q", got)
	}
	if _, ok := c2.Projects["old"]; ok {
		t.Fatalf("empty project 'old' was kept")
	}
	if got := c.Resolve(l); got != "old" {
		t.Fatalf("Add mutated its receiver")
	}
	if got := c2.Remove(l).Resolve(l); got != "github.com/t/backend" {
		t.Fatalf("after remove = %q", got)
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"tonton", "a", "my-app.v2", "x_1"} {
		if err := ValidName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Ton", "-x", "a b", "../x"} {
		if err := ValidName(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "projects.toml")
	c, _ := Config{Projects: map[string]Project{}}.Add("tonton", loc("github.com/t/a", "/w/a", "/w/a"))
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Resolve(loc("github.com/t/a", "", "/x")) != "tonton" {
		t.Fatalf("round trip lost mapping: %+v", got)
	}
	empty, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil || len(empty.Projects) != 0 {
		t.Fatalf("missing file: %+v %v", empty, err)
	}
}

func TestSameKeyAndExplicit(t *testing.T) {
	cfg := Config{Projects: map[string]Project{"p": {Remotes: []string{"github.com/o/r"}}}}
	remote := loc("github.com/o/r", "/x/r", "/x/r")
	if !SameKey("github.com/o/r", remote) || SameKey("github.com/o/other", remote) || SameKey("", remote) {
		t.Fatal("remote key comparison")
	}
	if name, ok := cfg.Explicit(remote); !ok || name != "p" {
		t.Fatalf("explicit = %q %v", name, ok)
	}
	folder := loc("", "", "/x/notes")
	if !SameKey("/x/notes/", folder) {
		t.Fatal("path key comparison should clean the path")
	}
	if _, ok := cfg.Explicit(folder); ok {
		t.Fatal("folder has no explicit mapping")
	}
}
