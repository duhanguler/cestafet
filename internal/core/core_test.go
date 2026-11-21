package core

import (
	"path/filepath"
	"testing"

	"github.com/orkono/cestafet/internal/identity"
	"github.com/orkono/cestafet/internal/store"
)

// newApp builds an App whose git lookup treats /home/u/git/* as repositories
// and everything else as plain directories.
func newApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &App{
		Store:        st,
		ProjectsPath: filepath.Join(dir, "projects.toml"),
		Home:         "/home/u",
		LookupRepo: func(d string) identity.Repo {
			if filepath.Dir(d) == "/home/u/git" {
				return identity.Repo{Root: d, Remote: "github.com/o/" + filepath.Base(d)}
			}
			return identity.Repo{}
		},
	}
}

func mustRegister(t *testing.T, a *App, id, cwd string, pid int) store.Session {
	t.Helper()
	s, _, err := a.Register(id, cwd, pid)
	if err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
	return s
}

func TestNonGitChildJoinsParentSession(t *testing.T) {
	a := newApp(t)
	parent := mustRegister(t, a, "p", "/home/u/work/app", 1)
	child := mustRegister(t, a, "c", "/home/u/work/app/docs", 2)
	if parent.Project != "/home/u/work/app" || child.Project != parent.Project {
		t.Fatalf("parent=%q child=%q", parent.Project, child.Project)
	}
}

func TestReverseStartOrderMeetsOnNextRegistration(t *testing.T) {
	a := newApp(t)
	child := mustRegister(t, a, "c", "/home/u/work/app/docs", 2)
	if child.Project != "/home/u/work/app/docs" {
		t.Fatalf("lone child project = %q", child.Project)
	}
	mustRegister(t, a, "p", "/home/u/work/app", 1)
	// the child's next prompt re-registers it
	child = mustRegister(t, a, "c", "/home/u/work/app/docs", 2)
	if child.Project != "/home/u/work/app" {
		t.Fatalf("child did not join parent: %q", child.Project)
	}
}

func TestChildKeepsProjectWhenParentLeaves(t *testing.T) {
	a := newApp(t)
	mustRegister(t, a, "p", "/home/u/work/app", 1)
	mustRegister(t, a, "c", "/home/u/work/app/docs", 2)
	if err := a.Store.End("p"); err != nil {
		t.Fatal(err)
	}
	child := mustRegister(t, a, "c", "/home/u/work/app/docs", 2)
	if child.Project != "/home/u/work/app" {
		t.Fatalf("child lost its project: %q", child.Project)
	}
}

func TestSiblingsAndHomeDoNotMerge(t *testing.T) {
	a := newApp(t)
	mustRegister(t, a, "home", "/home/u", 1)
	mustRegister(t, a, "a", "/home/u/work/a", 2)
	b := mustRegister(t, a, "b", "/home/u/work/b", 3)
	if b.Project != "/home/u/work/b" {
		t.Fatalf("sibling or home merged: %q", b.Project)
	}
	top := mustRegister(t, a, "top", "/", 4)
	tmp := mustRegister(t, a, "tmp", "/tmp/x", 5)
	if top.Project != "/" || tmp.Project != "/tmp/x" {
		t.Fatalf("root used as project: top=%q tmp=%q", top.Project, tmp.Project)
	}
}

func TestGitRepoInsideNonGitParentKeepsItsRemote(t *testing.T) {
	a := newApp(t)
	mustRegister(t, a, "p", "/home/u", 1)
	mustRegister(t, a, "w", "/home/u/git", 2)
	r := mustRegister(t, a, "r", "/home/u/git/api", 3)
	if r.Project != "github.com/o/api" {
		t.Fatalf("git repo project = %q", r.Project)
	}
}
