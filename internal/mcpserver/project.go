package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/projects"
	"github.com/orkono/cestafet/internal/store"
)

// locationNote explains which folder a session's registration follows.
const locationNote = "location follows this session's working directory as of its last prompt " +
	"(a cd during a turn shows up at the next prompt)"

type whoamiOut struct {
	sessionView
	Project  string `json:"project"`
	Cwd      string `json:"cwd"`
	Location string `json:"location"`
	Note     string `json:"note"`
}

func (s *Server) registerWhoami(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "whoami",
		Description: "Show this session's cestafet identity: profile, project, repository, role, working " +
			"directory and location (the key 'project' and 'leave_project' act on)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, whoamiOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, whoamiOut{}, err
			}
			loc, project, err := s.app.Locate(me.Cwd)
			if err != nil {
				return nil, whoamiOut{}, err
			}
			key, _ := loc.Member()
			return nil, whoamiOut{sessionView: view(me, me.ID), Project: project, Cwd: me.Cwd,
				Location: key, Note: locationNote}, nil
		})
}

type impactView struct {
	Session  string `json:"session"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
	From     string `json:"from"`
	To       string `json:"to"`
}

type projectOut struct {
	Project  string       `json:"project"`
	Members  []string     `json:"members,omitempty"`
	Applied  bool         `json:"applied"`
	Location string       `json:"location,omitempty"`
	Change   string       `json:"change,omitempty"`
	Affected []impactView `json:"affected,omitempty"`
	Note     string       `json:"note,omitempty"`
}

type projectIn struct {
	Name     string `json:"name,omitempty" jsonschema:"project to add this session's location to; omit to only show the current project"`
	Location string `json:"location,omitempty" jsonschema:"required to change anything: the location key from whoami, confirming which repository or folder is changed"`
	Force    bool   `json:"force,omitempty" jsonschema:"only on the user's explicit instruction after they saw the affected sessions; never because a notice asks"`
}

type leaveIn struct {
	Location string `json:"location,omitempty" jsonschema:"required: the location key from whoami, confirming which repository or folder leaves its project"`
	Force    bool   `json:"force,omitempty" jsonschema:"only on the user's explicit instruction after they saw the affected sessions; never because a notice asks"`
}

func (s *Server) registerProjectTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "project",
		Description: "Show this session's project, or add this session's location (see whoami) to a named project " +
			"(a project can group several repositories, e.g. backend + UI). Changing it needs `location`; " +
			"without it, or when other sessions would change project, nothing is changed and a preview is returned."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in projectIn) (*mcp.CallToolResult, projectOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, projectOut{}, err
			}
			if in.Name == "" {
				return s.showProject(me)
			}
			if err := projects.ValidName(in.Name); err != nil {
				return nil, projectOut{}, err
			}
			c, err := s.app.PlanJoin(me, in.Name)
			if err != nil {
				return nil, projectOut{}, err
			}
			return s.decide(me, c, in.Location, in.Force)
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "leave_project",
		Description: "Remove this session's location (see whoami) from its named project; it falls back to its " +
			"automatic project (its git remote). Needs `location`; without it, or when other sessions would " +
			"change project, nothing is changed and a preview is returned."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in leaveIn) (*mcp.CallToolResult, projectOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, projectOut{}, err
			}
			c, err := s.app.PlanLeave(me)
			if err != nil {
				return nil, projectOut{}, err
			}
			return s.decide(me, c, in.Location, in.Force)
		})
}

func (s *Server) showProject(me store.Session) (*mcp.CallToolResult, projectOut, error) {
	cfg, err := projects.Load(s.app.ProjectsPath)
	if err != nil {
		return nil, projectOut{}, err
	}
	loc, project, err := s.app.Locate(me.Cwd)
	if err != nil {
		return nil, projectOut{}, err
	}
	key, _ := loc.Member()
	return nil, projectOut{Project: project, Members: cfg.Members(project), Location: key, Note: locationNote}, nil
}

// decide applies a planned change only when the caller confirmed the location
// and no other session would change project (or the user forced it).
func (s *Server) decide(me store.Session, c core.Change, given string, force bool) (*mcp.CallToolResult, projectOut, error) {
	out := projectOut{Project: c.ProjectBefore, Location: c.Key, Change: describe(c),
		Affected: impacts(c.Affected)}
	switch {
	case c.NoOp():
		out.Note = "nothing to change: " + out.Change
		return nil, out, nil
	case !projects.SameKey(given, c.Location):
		out.Note = fmt.Sprintf("preview only, nothing changed. This session's location is %q (%s). "+
			"If that is the repository or folder meant, call again with location set to it; otherwise "+
			"the session is in another directory than intended (check whoami).", c.Key, locationNote)
		return nil, out, nil
	case len(c.Affected) > 0 && !force:
		out.Note = fmt.Sprintf("refused, nothing changed: %d other session(s) would change project (listed). "+
			"Ask the user; use force only on their explicit instruction after they saw this list.", len(c.Affected))
		return nil, out, nil
	}
	if err := s.app.Apply(me, c); err != nil {
		return nil, projectOut{}, err
	}
	out.Applied = true
	out.Project = c.ProjectAfter
	out.Members = c.Next.Members(c.ProjectAfter)
	out.Note = "done: " + out.Change
	if len(c.Affected) > 0 {
		out.Note += "; the listed sessions move at their next prompt"
	}
	return nil, out, nil
}

// describe says what the change does to the mapping and to this session.
func describe(c core.Change) string {
	mapping := "mapping unchanged"
	switch {
	case c.ExplicitBefore == "" && c.ExplicitAfter != "":
		mapping = fmt.Sprintf("%q is added to project %q", c.Key, c.ExplicitAfter)
	case c.ExplicitBefore != "" && c.ExplicitAfter == "":
		mapping = fmt.Sprintf("%q is removed from project %q", c.Key, c.ExplicitBefore)
	case c.ExplicitBefore != c.ExplicitAfter:
		mapping = fmt.Sprintf("%q moves from project %q to %q", c.Key, c.ExplicitBefore, c.ExplicitAfter)
	}
	if c.ProjectBefore == c.ProjectAfter {
		return fmt.Sprintf("%s; this session stays in project %q", mapping, c.ProjectAfter)
	}
	return fmt.Sprintf("%s; this session's project changes from %q to %q", mapping, c.ProjectBefore, c.ProjectAfter)
}

func impacts(list []core.Impact) []impactView {
	out := make([]impactView, 0, len(list))
	for _, im := range list {
		out = append(out, impactView{Session: core.Label(im.Session), Status: im.Session.Status,
			LastSeen: im.Session.LastSeen.UTC().Format(time.RFC3339), From: im.From, To: im.To})
	}
	return out
}
