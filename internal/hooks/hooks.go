// Package hooks implements the Claude Code hook handlers: they register the
// session, keep it alive and inject pending notices into its context.
package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/orkono/cestafet/internal/core"
)

// Input is the subset of the hook payload cestafet reads.
type Input struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Source    string `json:"source"` // SessionStart: startup, resume, clear, compact
	Prompt    string `json:"prompt"` // UserPromptSubmit
}

type specificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

type output struct {
	HookSpecificOutput specificOutput `json:"hookSpecificOutput"`
}

// Events maps the CLI event names to Claude Code hook event names.
var Events = map[string]string{
	"session-start": "SessionStart",
	"prompt":        "UserPromptSubmit",
	"post-tool":     "PostToolUse",
	"session-end":   "SessionEnd",
}

// Handle runs one hook event. It reads the payload from in and writes the hook
// response (if any) to out. claudePID identifies the owning claude process.
func Handle(app *core.App, event string, in io.Reader, out io.Writer, claudePID int) error {
	name, ok := Events[event]
	if !ok {
		return fmt.Errorf("unknown hook event %q", event)
	}
	var input Input
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return fmt.Errorf("decode hook payload: %w", err)
	}
	if input.SessionID == "" {
		return fmt.Errorf("hook payload has no session_id")
	}

	context, err := run(app, name, input, claudePID)
	if err != nil || context == "" {
		return err
	}
	return json.NewEncoder(out).Encode(output{specificOutput{HookEventName: name, AdditionalContext: context}})
}

func run(app *core.App, name string, in Input, claudePID int) (string, error) {
	switch name {
	case "SessionEnd":
		return "", app.Store.End(in.SessionID)

	case "SessionStart":
		sess, _, err := app.Register(in.SessionID, in.Cwd, claudePID)
		if err != nil {
			return "", err
		}
		return join(app.Orientation(sess), pending(app, sess.ID)), nil

	case "UserPromptSubmit":
		// Re-resolve the project on each prompt so a /cestafet:project change or
		// a cwd change is picked up; cheap compared to a model turn.
		sess, isNew, err := app.Register(in.SessionID, in.Cwd, claudePID)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(in.Prompt) != "" {
			if _, err := app.Store.RecordPrompt(sess.ID, in.Prompt); err != nil {
				return "", err
			}
		}
		orient := ""
		if isNew { // plugin installed mid-session: no SessionStart was seen
			orient = app.Orientation(sess)
		}
		return join(orient, pending(app, sess.ID)), nil

	default: // PostToolUse: must stay cheap, no git calls
		if _, err := app.Store.Get(in.SessionID); err != nil {
			if _, _, err := app.Register(in.SessionID, in.Cwd, claudePID); err != nil {
				return "", err
			}
		} else if err := app.Store.Touch(in.SessionID); err != nil {
			return "", err
		}
		return pending(app, in.SessionID), nil
	}
}

func pending(app *core.App, sessionID string) string {
	notices, err := app.Store.TakePending(sessionID)
	if err != nil {
		return ""
	}
	return core.RenderNotices(notices)
}

func join(parts ...string) string {
	var keep []string
	for _, p := range parts {
		if p != "" {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, "\n\n")
}
