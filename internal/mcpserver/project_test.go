package mcpserver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/identity"
	"github.com/orkono/cestafet/internal/paths"
	"github.com/orkono/cestafet/internal/projects"
	"github.com/orkono/cestafet/internal/store"
)

func testApp(t *testing.T) *core.App {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &core.App{Store: st, ProjectsPath: filepath.Join(dir, "projects.toml"),
		LookupRepo: func(d string) identity.Repo {
			if filepath.Base(d) == "fnet" {
				return identity.Repo{Root: d, Remote: "github.com/o/fnet"}
			}
			return identity.Repo{}
		}}
}

// auditorSetup reproduces the incident: two sessions and an auditor in the
// shared repo, which is mapped to project fnet.
func auditorSetup(t *testing.T) (*Server, store.Session) {
	t.Helper()
	app := testApp(t)
	if err := projects.Save(app.ProjectsPath, projects.Config{Projects: map[string]projects.Project{
		"fnet": {Remotes: []string{"github.com/o/fnet"}}}}); err != nil {
		t.Fatal(err)
	}
	repo := paths.Canon(filepath.Join(t.TempDir(), "fnet"))
	var me store.Session
	for i, id := range []string{"k1", "k2", "au"} {
		s, _, err := app.Register(id, repo, identity.Proc{PID: 10 + i})
		if err != nil {
			t.Fatal(err)
		}
		me = s
	}
	return &Server{app: app}, me
}

func TestLeaveWithoutLocationIsAPreview(t *testing.T) {
	s, me := auditorSetup(t)
	c, err := s.app.PlanLeave(me)
	if err != nil {
		t.Fatal(err)
	}
	_, out, err := s.decide(me, c, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Applied || out.Location != "github.com/o/fnet" || !strings.Contains(out.Note, "preview only") {
		t.Fatalf("out = %+v", out)
	}
	if len(out.Affected) != 2 {
		t.Fatalf("affected = %+v", out.Affected)
	}
}

func TestLeaveRefusedWhenPeersWouldMove(t *testing.T) {
	s, me := auditorSetup(t)
	c, _ := s.app.PlanLeave(me)
	_, out, err := s.decide(me, c, "github.com/o/fnet", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Applied || !strings.Contains(out.Note, "refused") {
		t.Fatalf("out = %+v", out)
	}
	cfg, _ := projects.Load(s.app.ProjectsPath)
	if len(cfg.Members("fnet")) != 1 {
		t.Fatal("mapping changed despite refusal")
	}
}

func TestForceNeverBypassesLocation(t *testing.T) {
	s, me := auditorSetup(t)
	c, _ := s.app.PlanLeave(me)
	_, out, _ := s.decide(me, c, "/somewhere/else", true)
	if out.Applied {
		t.Fatal("force applied a change with a mismatched location")
	}
	_, out, err := s.decide(me, c, "github.com/o/fnet", true)
	if err != nil || !out.Applied || out.Project != "github.com/o/fnet" {
		t.Fatalf("forced leave: %+v, %v", out, err)
	}
	if !strings.Contains(out.Note, "next prompt") {
		t.Fatalf("note = %q", out.Note)
	}
}
