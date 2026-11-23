package core

import (
	"github.com/orkono/cestafet/internal/projects"
	"github.com/orkono/cestafet/internal/store"
)

// Impact is a session whose project would change under a proposed mapping.
type Impact struct {
	Session store.Session
	From    string
	To      string
}

// Change describes a proposed edit of the project mapping for one location,
// before it is saved.
type Change struct {
	Location projects.Location
	Key      string // the location's member key: a git remote, a repo root or a folder
	IsRemote bool
	// ExplicitBefore / ExplicitAfter name the project that maps the key
	// directly (not through a parent path); empty when none.
	ExplicitBefore string
	ExplicitAfter  string
	// ProjectBefore / ProjectAfter are this session's resolved project.
	ProjectBefore string
	ProjectAfter  string
	Next          projects.Config
	Affected      []Impact // other live or idle sessions whose project changes
}

// NoOp reports whether the change alters neither the mapping nor any project.
func (c Change) NoOp() bool {
	return c.ExplicitBefore == c.ExplicitAfter && c.ProjectBefore == c.ProjectAfter
}

// PlanJoin previews mapping the session's current location to a named project.
func (a *App) PlanJoin(self store.Session, name string) (Change, error) {
	return a.plan(self, func(cfg projects.Config, l projects.Location) (projects.Config, error) {
		return cfg.Add(name, l)
	})
}

// PlanLeave previews removing the session's current location from its named project.
func (a *App) PlanLeave(self store.Session) (Change, error) {
	return a.plan(self, func(cfg projects.Config, l projects.Location) (projects.Config, error) {
		return cfg.Remove(l), nil
	})
}

func (a *App) plan(self store.Session, edit func(projects.Config, projects.Location) (projects.Config, error)) (Change, error) {
	cfg, err := projects.Load(a.ProjectsPath)
	if err != nil {
		return Change{}, err
	}
	loc, before, err := a.locateWith(self.Cwd, cfg)
	if err != nil {
		return Change{}, err
	}
	next, err := edit(cfg, loc)
	if err != nil {
		return Change{}, err
	}
	_, after, err := a.locateWith(self.Cwd, next)
	if err != nil {
		return Change{}, err
	}
	key, isRemote := loc.Member()
	explBefore, _ := cfg.Explicit(loc)
	explAfter, _ := next.Explicit(loc)
	affected, err := a.affected(cfg, next, self.ID)
	if err != nil {
		return Change{}, err
	}
	return Change{
		Location: loc, Key: key, IsRemote: isRemote,
		ExplicitBefore: explBefore, ExplicitAfter: explAfter,
		ProjectBefore: before, ProjectAfter: after,
		Next: next, Affected: affected,
	}, nil
}

// Apply saves a planned change and moves this session to its resulting project.
// Other sessions move at their next prompt.
func (a *App) Apply(self store.Session, c Change) error {
	if err := projects.Save(a.ProjectsPath, c.Next); err != nil {
		return err
	}
	return a.Store.SetProject(self.ID, c.ProjectAfter)
}

// affected re-resolves every other non-gone session under both mappings, with
// the same resolution registration uses, and returns those whose project differs.
func (a *App) affected(cur, next projects.Config, selfID string) ([]Impact, error) {
	sessions, err := a.Store.Active()
	if err != nil {
		return nil, err
	}
	var out []Impact
	for _, sess := range sessions {
		if sess.ID == selfID || sess.Cwd == "" {
			continue
		}
		_, from, err := a.locateWith(sess.Cwd, cur)
		if err != nil {
			return nil, err
		}
		_, to, err := a.locateWith(sess.Cwd, next)
		if err != nil {
			return nil, err
		}
		if from != to {
			out = append(out, Impact{Session: sess, From: from, To: to})
		}
	}
	return out, nil
}
