// Package mcpserver exposes cestafet to a Claude Code session as MCP tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/identity"
	"github.com/orkono/cestafet/internal/projects"
	"github.com/orkono/cestafet/internal/store"
)

// userPromptWindow is how recent the user's last prompt must be for a notice
// to be labelled origin="user".
const userPromptWindow = 30 * time.Minute

// Server serves the tools of one session.
type Server struct {
	app    *core.App
	claude identity.Proc
}

// New creates a server for the session owned by the given Claude Code process.
func New(app *core.App, claude identity.Proc) *Server {
	return &Server{app: app, claude: claude}
}

// self finds this server's session. It is resolved on every call because the
// MCP server may start before the SessionStart hook has registered the session,
// and /clear can give the same claude process a new session id.
func (s *Server) self() (store.Session, error) {
	if s.claude.PID == 0 {
		return store.Session{}, errors.New("cestafet could not find the claude process that started this MCP server")
	}
	sess, err := s.app.Store.ByOwner(s.claude.PID, core.StartedMillis(s.claude))
	// A hook may be registering the session right now (startup, /clear): wait briefly.
	for i := 0; i < 10 && errors.Is(err, store.ErrNotFound); i++ {
		time.Sleep(200 * time.Millisecond)
		sess, err = s.app.Store.ByOwner(s.claude.PID, core.StartedMillis(s.claude))
	}
	if errors.Is(err, store.ErrNotFound) {
		return store.Session{}, errors.New("this session is not registered yet; the cestafet plugin's hooks register it " +
			"at session start or on the next prompt (run `cestafet doctor` if this persists)")
	}
	return sess, err
}

// Run serves MCP over stdio until the client disconnects.
func (s *Server) Run(ctx context.Context, version string) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "cestafet", Version: version}, nil)
	s.register(srv)
	return srv.Run(ctx, &mcp.StdioTransport{})
}

type empty struct{}

type sessionView struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Profile  string `json:"profile"`
	Repo     string `json:"repo"`
	Branch   string `json:"branch,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
	IsYou    bool   `json:"is_you,omitempty"`
}

func view(sess store.Session, selfID string) sessionView {
	return sessionView{
		ID: sess.ID, Label: core.Label(sess), Profile: sess.Profile, Repo: sess.Repo, Branch: sess.Branch,
		Summary: sess.Summary, Role: sess.Role, Status: sess.Status,
		LastSeen: sess.LastSeen.UTC().Format(time.RFC3339), IsYou: sess.ID == selfID,
	}
}

func (s *Server) register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "whoami",
		Description: "Show this session's cestafet identity: profile, project, repository, role."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, sessionView, error) {
			me, err := s.self()
			if err != nil {
				return nil, sessionView{}, err
			}
			v := view(me, me.ID)
			return nil, v, nil
		})

	type introduceIn struct {
		Summary string `json:"summary" jsonschema:"one line on what this session is working on"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "introduce",
		Description: "Tell the other sessions of the project what this session is working on (shown by 'sessions')."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in introduceIn) (*mcp.CallToolResult, sessionView, error) {
			me, err := s.self()
			if err != nil {
				return nil, sessionView{}, err
			}
			if len(in.Summary) > 300 {
				return nil, sessionView{}, errors.New("summary must be at most 300 characters")
			}
			if err := s.app.Store.SetSummary(me.ID, in.Summary); err != nil {
				return nil, sessionView{}, err
			}
			me.Summary = in.Summary
			return nil, view(me, me.ID), nil
		})

	type sessionsOut struct {
		Project  string        `json:"project"`
		Sessions []sessionView `json:"sessions"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "sessions",
		Description: "List the sessions working on this project (all repositories of it, all profiles), coordinator first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, sessionsOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, sessionsOut{}, err
			}
			list, err := s.app.Store.List(me.Project)
			if err != nil {
				return nil, sessionsOut{}, err
			}
			out := sessionsOut{Project: me.Project}
			for _, sess := range list {
				out.Sessions = append(out.Sessions, view(sess, me.ID))
			}
			return nil, out, nil
		})

	s.registerProjectTools(srv)
	s.registerNoticeTools(srv)
	s.registerRuleTools(srv)
}

type projectOut struct {
	Project string   `json:"project"`
	Members []string `json:"members,omitempty"`
	Note    string   `json:"note,omitempty"`
}

func (s *Server) registerProjectTools(srv *mcp.Server) {
	type projectIn struct {
		Name string `json:"name,omitempty" jsonschema:"project to add this repository to; omit to only show the current project"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "project",
		Description: "Show this session's project, or add this repository to a named project (a project can group " +
			"several repositories, e.g. backend + UI) and move this session there."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in projectIn) (*mcp.CallToolResult, projectOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, projectOut{}, err
			}
			cfg, err := projects.Load(s.app.ProjectsPath)
			if err != nil {
				return nil, projectOut{}, err
			}
			if in.Name == "" {
				return nil, projectOut{Project: me.Project, Members: cfg.Members(me.Project)}, nil
			}
			loc, _, err := s.app.Locate(me.Cwd)
			if err != nil {
				return nil, projectOut{}, err
			}
			next, err := cfg.Add(in.Name, loc)
			if err != nil {
				return nil, projectOut{}, err
			}
			if err := projects.Save(s.app.ProjectsPath, next); err != nil {
				return nil, projectOut{}, err
			}
			if err := s.app.Store.SetProject(me.ID, in.Name); err != nil {
				return nil, projectOut{}, err
			}
			return nil, projectOut{Project: in.Name, Members: next.Members(in.Name),
				Note: "other sessions in this repository move to the project on their next prompt"}, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "leave_project",
		Description: "Remove this repository from its named project; it falls back to its automatic project (its git remote)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, projectOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, projectOut{}, err
			}
			cfg, err := projects.Load(s.app.ProjectsPath)
			if err != nil {
				return nil, projectOut{}, err
			}
			loc, _, err := s.app.Locate(me.Cwd)
			if err != nil {
				return nil, projectOut{}, err
			}
			next := cfg.Remove(loc)
			if err := projects.Save(s.app.ProjectsPath, next); err != nil {
				return nil, projectOut{}, err
			}
			project := next.Resolve(loc)
			if err := s.app.Store.SetProject(me.ID, project); err != nil {
				return nil, projectOut{}, err
			}
			return nil, projectOut{Project: project}, nil
		})
}

func fmtErr(format string, args ...any) error { return fmt.Errorf(format, args...) }
