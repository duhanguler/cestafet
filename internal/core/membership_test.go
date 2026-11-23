package core

import (
	"testing"

	"github.com/orkono/cestafet/internal/projects"
)

func saveConfig(t *testing.T, a *App, cfg projects.Config) {
	t.Helper()
	if err := projects.Save(a.ProjectsPath, cfg); err != nil {
		t.Fatal(err)
	}
}

// The auditor incident: a session that cd'd into a shared repo plans to leave;
// the two live sessions working there would lose the project.
func TestPlanLeaveListsPeersWhoseProjectChanges(t *testing.T) {
	a := newApp(t)
	saveConfig(t, a, projects.Config{Projects: map[string]projects.Project{
		"fnet": {Remotes: []string{"github.com/o/fnet"}}}})
	repo := tp("/home/u/git/fnet")
	mustRegister(t, a, "k1", repo, 1)
	mustRegister(t, a, "k2", repo, 2)
	auditor := mustRegister(t, a, "au", repo, 3)

	c, err := a.PlanLeave(auditor)
	if err != nil {
		t.Fatal(err)
	}
	if c.Key != "github.com/o/fnet" || !c.IsRemote {
		t.Fatalf("key = %q remote=%v", c.Key, c.IsRemote)
	}
	if len(c.Affected) != 2 {
		t.Fatalf("affected = %+v, want k1 and k2", c.Affected)
	}
	for _, im := range c.Affected {
		if im.From != "fnet" || im.To != "github.com/o/fnet" {
			t.Fatalf("impact %+v", im)
		}
	}
	if c.ExplicitBefore != "fnet" || c.ExplicitAfter != "" || c.NoOp() {
		t.Fatalf("explicit %q→%q noop=%v", c.ExplicitBefore, c.ExplicitAfter, c.NoOp())
	}
}

// A parent mapping still covers the repo: removing its explicit entry moves nobody.
func TestPlanLeaveWithRetainedParentMovesNobody(t *testing.T) {
	a := newApp(t)
	saveConfig(t, a, projects.Config{Projects: map[string]projects.Project{
		"fnet": {Remotes: []string{"github.com/o/fnet"}, Paths: []string{tp("/home/u/git")}}}})
	repo := tp("/home/u/git/fnet")
	mustRegister(t, a, "k1", repo, 1)
	auditor := mustRegister(t, a, "au", repo, 3)

	c, err := a.PlanLeave(auditor)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Affected) != 0 {
		t.Fatalf("affected = %+v, want none", c.Affected)
	}
	if c.ProjectAfter != "fnet" || c.ExplicitBefore != "fnet" || c.ExplicitAfter != "" {
		t.Fatalf("change %+v", c)
	}
	if c.NoOp() {
		t.Fatal("removing an explicit entry is not a no-op")
	}
}

// Joining another project pulls the repo out of its current one, and its live
// sessions with it.
func TestPlanJoinListsSessionsPulledFromAnotherProject(t *testing.T) {
	a := newApp(t)
	saveConfig(t, a, projects.Config{Projects: map[string]projects.Project{
		"alpha": {Remotes: []string{"github.com/o/fnet"}}}})
	repo := tp("/home/u/git/fnet")
	mustRegister(t, a, "k1", repo, 1)
	me := mustRegister(t, a, "me", repo, 2)

	c, err := a.PlanJoin(me, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Affected) != 1 || c.Affected[0].From != "alpha" || c.Affected[0].To != "beta" {
		t.Fatalf("affected = %+v", c.Affected)
	}
	if err := a.Apply(me, c); err != nil {
		t.Fatal(err)
	}
	got, err := a.Store.Get("me")
	if err != nil || got.Project != "beta" {
		t.Fatalf("self project = %q, %v", got.Project, err)
	}
}

func TestPlanLeaveWithoutMappingIsNoOp(t *testing.T) {
	a := newApp(t)
	me := mustRegister(t, a, "me", tp("/home/u/git/fnet"), 1)
	c, err := a.PlanLeave(me)
	if err != nil {
		t.Fatal(err)
	}
	if !c.NoOp() || len(c.Affected) != 0 {
		t.Fatalf("change %+v", c)
	}
}
