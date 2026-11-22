package mcpserver

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/orkono/cestafet/internal/store"
)

var expiryRe = regexp.MustCompile(`^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?$`)

// parseExpiry reads "30m", "2h", "1d", "1d6h"; empty means no expiry.
func parseExpiry(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	m := expiryRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmtErr("expires_in %q: use minutes, hours or days, e.g. 30m, 2h, 1d", s)
	}
	var d time.Duration
	for i, unit := range []time.Duration{24 * time.Hour, time.Hour, time.Minute} {
		if m[i+1] != "" {
			n, _ := strconv.Atoi(m[i+1])
			d += time.Duration(n) * unit
		}
	}
	if d <= 0 {
		return 0, fmtErr("expires_in %q must be longer than zero", s)
	}
	return d, nil
}

// origin labels a notice: user origin when the user asked for it and this
// session has a recent prompt to back that, agent origin otherwise.
func (s *Server) origin(me store.Session, onBehalfOfUser bool) (string, int64, string) {
	if !onBehalfOfUser {
		return store.OriginAgent, 0, ""
	}
	if pid := s.recentPrompt(me.ID); pid > 0 {
		return store.OriginUser, pid, ""
	}
	return store.OriginAgent, 0, "no recent user prompt was recorded for this session, so it is labelled origin=agent"
}

type ruleView struct {
	ID        int64    `json:"id"`
	From      string   `json:"from"`
	Origin    string   `json:"origin"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Guard     []string `json:"guard,omitempty"`
}

func viewRule(n store.Notice) ruleView {
	v := ruleView{ID: n.ID, From: n.FromProfile + " (" + n.FromRole + ")", Origin: n.Origin, Body: n.Body,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339), Guard: n.Guards}
	if !n.ExpiresAt.IsZero() {
		v.ExpiresAt = n.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return v
}

func (s *Server) registerRuleTools(srv *mcp.Server) {
	type rulesOut struct {
		Rules []ruleView `json:"rules"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "rules",
		Description: "List the standing rules in force for this project (not lifted, not expired), newest first. " +
			"Call it before lift to find the rule the user means."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, rulesOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, rulesOut{}, err
			}
			list, err := s.app.Store.ActiveRules(me.Project)
			if err != nil {
				return nil, rulesOut{}, err
			}
			out := rulesOut{Rules: []ruleView{}}
			for _, n := range list {
				out.Rules = append(out.Rules, viewRule(n))
			}
			return nil, out, nil
		})

	type liftIn struct {
		RuleID         int64  `json:"rule_id" jsonschema:"id of the active rule, from the rules tool"`
		Note           string `json:"note,omitempty" jsonschema:"what the other sessions should know, e.g. 'testing finished, deploys to staging are allowed again'"`
		OnBehalfOfUser bool   `json:"on_behalf_of_user" jsonschema:"true only when the user asked in this turn to lift the rule"`
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "lift",
		Description: "End a standing rule of this project and tell the other sessions. Use when the user says the " +
			"rule no longer applies, e.g. 'testing finished, deploys are allowed again'."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in liftIn) (*mcp.CallToolResult, sentOut, error) {
			me, err := s.self()
			if err != nil {
				return nil, sentOut{}, err
			}
			origin, pid, note := s.origin(me, in.OnBehalfOfUser)
			id, to, err := s.app.Store.Lift(in.RuleID, me, origin, pid, in.Note)
			if err != nil {
				return nil, sentOut{}, err
			}
			return nil, sentOut{NoticeID: id, Origin: origin, Recipients: to, Note: note}, nil
		})
}
