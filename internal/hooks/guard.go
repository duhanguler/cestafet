package hooks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/guard"
	"github.com/orkono/cestafet/internal/store"
)

// guardReason checks a Bash command against the guard patterns of the
// project's rules in force. It returns the reason to ask the user with, or ""
// to let the command run. It never denies: the user decides.
func guardReason(app *core.App, in Input) (string, error) {
	if in.ToolName != "Bash" || strings.TrimSpace(in.ToolInput.Command) == "" {
		return "", nil
	}
	sess, err := app.Store.Get(in.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil // not registered yet; the guard is best effort
	}
	if err != nil {
		return "", err
	}
	rules, err := app.Store.GuardedRules(sess.Project)
	if err != nil {
		return "", err
	}
	for _, r := range rules {
		for _, p := range r.Guards {
			if guard.Match(p, in.ToolInput.Command) {
				return reason(r, p), nil
			}
		}
	}
	return "", nil
}

func reason(r store.Notice, pattern string) string {
	from := r.FromProfile
	if r.FromRole == store.RoleCoordinator {
		from = "the coordinator (" + from + ")"
	}
	who := "an agent's own request"
	if r.Origin == store.OriginUser {
		who = "the user's decision, relayed"
	}
	return fmt.Sprintf("cestafet rule #%d from %s (%s): %s\nThis command matches its guard %q. "+
		"Run it only if the rule no longer applies.", r.ID, from, who, quoteBody(r.Body), pattern)
}

func quoteBody(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = strings.ToValidUTF8(s[:300], "") + "…"
	}
	return s
}
