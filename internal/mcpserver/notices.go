package mcpserver

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/store"
)

type sentOut struct {
	NoticeID   int64    `json:"notice_id"`
	Origin     string   `json:"origin"`
	Recipients []string `json:"recipients"`
	Note       string   `json:"note,omitempty"`
}

type noticeView struct {
	ID        int64  `json:"id"`
	From      string `json:"from"`
	Origin    string `json:"origin"`
	Request   string `json:"user_request,omitempty"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	Acked     bool   `json:"acked"`
	Kind      string `json:"kind"`
	Active    bool   `json:"active_rule,omitempty"`
}

func (s *Server) registerNoticeTools(srv *mcp.Server) {
	type coordinateOut struct {
		Project      string   `json:"project"`
		Previous     string   `json:"previous_coordinator,omitempty"`
		AnnouncedTo  []string `json:"announced_to"`
		AnnounceNote string   `json:"note"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "coordinate",
		Description: "Make this session the coordinator of its project and announce it to the other sessions. " +
			"Use when the user asks this session to coordinate the project."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, coordinateOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, coordinateOut{}, err
			}
			prev, err := s.app.Store.MakeCoordinator(me.ID)
			if err != nil {
				return nil, coordinateOut{}, err
			}
			me.Role = store.RoleCoordinator
			body := "This session (" + core.Label(me) + ") is now the coordinator of project " + me.Project +
				". Notices from it relay the user's decisions for the project."
			announce := store.NewNotice{From: me, Origin: store.OriginAgent, Body: body}
			if pid := s.recentPrompt(me.ID); pid > 0 {
				announce.Origin, announce.PromptID = store.OriginUser, pid
			}
			_, to, err := s.app.Store.Send(announce)
			if err != nil {
				return nil, coordinateOut{}, err
			}
			return nil, coordinateOut{Project: me.Project, Previous: prev, AnnouncedTo: to,
				AnnounceNote: "sessions see the announcement at their next prompt or tool call"}, nil
		})

	type notifyIn struct {
		Body           string   `json:"body" jsonschema:"the notice, written for the other sessions, at most 4096 bytes"`
		To             []string `json:"to,omitempty" jsonschema:"session ids to address; omit to reach every other session of the project"`
		OnBehalfOfUser bool     `json:"on_behalf_of_user" jsonschema:"true only when the user asked in this turn to tell the other sessions; false for your own initiative"`
		Standing       bool     `json:"standing,omitempty" jsonschema:"true for a rule that stays in force until lifted, e.g. 'no deploys to staging while testing', 'from now on ...'; false for one-off news"`
		ExpiresIn      string   `json:"expires_in,omitempty" jsonschema:"standing rules only: end it automatically after this long, e.g. 2h or 1d (max 30d); omit to keep it until lifted"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "notify",
		Description: "Send a notice to the other sessions of this project (every repository and profile in it). " +
			"Use when the user asks to tell, notify or inform the other sessions, e.g. 'tell the others testing " +
			"started, no deploys to staging'. Sessions receive it at their next prompt or tool call. Set standing " +
			"for a rule that must hold until lifted; sessions that start later still receive an active rule."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in notifyIn) (*mcp.CallToolResult, sentOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, sentOut{}, err
			}
			expires, err := parseExpiry(in.ExpiresIn)
			if err != nil {
				return nil, sentOut{}, err
			}
			n := store.NewNotice{From: me, Body: in.Body, To: in.To, ExpiresIn: expires}
			if in.Standing || expires > 0 {
				n.Kind = store.KindRule
			}
			var note string
			n.Origin, n.PromptID, note = s.origin(me, in.OnBehalfOfUser)
			id, to, err := s.app.Store.Send(n)
			if err != nil {
				return nil, sentOut{}, err
			}
			if len(to) == 0 {
				later := "sessions that start within 24h will still receive it"
				if n.Kind == store.KindRule {
					later = "sessions that start later receive it for as long as the rule is in force"
				}
				note = strings.TrimSpace(note + "; no other session is on this project right now — " + later)
			}
			return nil, sentOut{NoticeID: id, Origin: n.Origin, Recipients: to, Note: strings.TrimPrefix(note, "; ")}, nil
		})

	type noticesIn struct {
		UnackedOnly bool `json:"unacked_only,omitempty" jsonschema:"only notices not yet acknowledged"`
	}
	type noticesOut struct {
		Notices []noticeView `json:"notices"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "notices",
		Description: "List notices this session received from other sessions of the project, newest first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in noticesIn) (*mcp.CallToolResult, noticesOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, noticesOut{}, err
			}
			list, err := s.app.Store.Received(me.ID, in.UnackedOnly, 50)
			if err != nil {
				return nil, noticesOut{}, err
			}
			out := noticesOut{Notices: []noticeView{}}
			for _, n := range list {
				out.Notices = append(out.Notices, noticeView{ID: n.ID, From: n.FromProfile + " (" + n.FromRole + ")",
					Origin: n.Origin, Request: n.PromptText, Body: n.Body,
					CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339), Acked: !n.AckedAt.IsZero(), Kind: n.Kind,
					Active: n.Kind == store.KindRule && n.LiftedAt.IsZero() &&
						(n.ExpiresAt.IsZero() || n.ExpiresAt.After(time.Now()))})
			}
			return nil, out, nil
		})

	type ackIn struct {
		NoticeID int64  `json:"notice_id"`
		Reply    string `json:"reply,omitempty" jsonschema:"one line back to the sender, e.g. 'ok, holding deploys'"`
	}
	type ackOut struct {
		Acked bool `json:"acked"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "ack",
		Description: "Acknowledge a received notice, optionally with a one-line reply to its sender."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in ackIn) (*mcp.CallToolResult, ackOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, ackOut{}, err
			}
			if len(in.Reply) > 300 {
				return nil, ackOut{}, fmtErr("reply must be at most 300 characters")
			}
			if err := s.app.Store.Ack(in.NoticeID, me.ID, in.Reply); err != nil {
				return nil, ackOut{}, err
			}
			return nil, ackOut{Acked: true}, nil
		})

	type statusIn struct {
		NoticeID int64 `json:"notice_id"`
	}
	type statusOut struct {
		Deliveries []store.Delivery `json:"deliveries"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "notice_status",
		Description: "Show, per recipient, whether a notice you sent was delivered and acknowledged, with replies."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in statusIn) (*mcp.CallToolResult, statusOut, error) {
			if _, err := s.self(); err != nil {
				return nil, statusOut{}, err
			}
			d, err := s.app.Store.Status(in.NoticeID)
			if err != nil {
				return nil, statusOut{}, err
			}
			if d == nil {
				d = []store.Delivery{}
			}
			return nil, statusOut{Deliveries: d}, nil
		})
}

// recentPrompt returns the id of the session's last user prompt if it is recent.
func (s *Server) recentPrompt(sessionID string) int64 {
	p, err := s.app.Store.LastPrompt(sessionID)
	if err != nil || time.Since(p.At) > userPromptWindow {
		return 0
	}
	return p.ID
}
