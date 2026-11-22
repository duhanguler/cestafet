package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/orkono/cestafet/internal/guard"
)

// Notice kinds.
const (
	KindInfo = "info" // news, delivered once
	KindRule = "rule" // stays in force until lifted or expired
)

// MaxRuleLifetime caps a rule's expiry.
const MaxRuleLifetime = 30 * 24 * time.Hour

// activeRule is the SQL condition for a rule (alias n) still in force; it takes
// the current unix time as its one parameter.
const activeRule = `n.lifted_at IS NULL AND (n.expires_at IS NULL OR n.expires_at > ?)`

// ruleFields validates a notice's kind and expiry and returns the values to
// store: the kind and the expiry as unix seconds (nil when none).
func (s *Store) ruleFields(n NewNotice) (string, any, error) {
	kind := n.Kind
	if kind == "" {
		kind = KindInfo
	}
	switch {
	case kind != KindInfo && kind != KindRule:
		return "", nil, fmt.Errorf("invalid notice kind %q", kind)
	case n.ExpiresIn < 0:
		return "", nil, errors.New("expiry must be positive")
	case n.ExpiresIn > 0 && kind != KindRule:
		return "", nil, errors.New("only a standing rule can expire")
	case n.ExpiresIn > MaxRuleLifetime:
		return "", nil, fmt.Errorf("expiry %s exceeds the limit of %s", n.ExpiresIn, MaxRuleLifetime)
	case n.ExpiresIn > 0:
		return kind, s.now().Add(n.ExpiresIn).Unix(), nil
	}
	return kind, nil, nil
}

// guardField validates a notice's guard patterns and returns them as stored.
func guardField(kind string, patterns []string) (string, error) {
	if len(patterns) == 0 {
		return "", nil
	}
	if kind != KindRule {
		return "", errors.New("only a standing rule can carry guard patterns")
	}
	clean, err := guard.Validate(patterns)
	if err != nil {
		return "", err
	}
	return strings.Join(clean, "\n"), nil
}

// GuardedRules lists the project's rules in force that carry guard patterns,
// newest first. It is called before every Bash command, so it stays one
// indexed query.
func (s *Store) GuardedRules(project string) ([]Notice, error) {
	rows, err := s.db.Query(`SELECT `+noticeBase+`, 0
		FROM notices n LEFT JOIN prompts p ON p.id = n.prompt_id
		WHERE n.project = ? AND n.kind = 'rule' AND n.guards != '' AND `+activeRule+` ORDER BY n.id DESC`,
		project, s.unix())
	if err != nil {
		return nil, fmt.Errorf("guarded rules: %w", err)
	}
	return scanNotices(rows)
}

// ActiveRules lists the rules of a project still in force, newest first.
func (s *Store) ActiveRules(project string) ([]Notice, error) {
	rows, err := s.db.Query(`SELECT `+noticeBase+`, 0
		FROM notices n LEFT JOIN prompts p ON p.id = n.prompt_id
		WHERE n.project = ? AND n.kind = 'rule' AND `+activeRule+` ORDER BY n.id DESC`,
		project, s.unix())
	if err != nil {
		return nil, fmt.Errorf("active rules: %w", err)
	}
	return scanNotices(rows)
}

// Lift ends an active rule of the lifter's project and sends an info notice
// about it to every other live session of the project. note is the lifter's
// own words; an empty note gets a generated one. It returns the lift notice id
// and its recipients.
func (s *Store) Lift(ruleID int64, by Session, origin string, promptID int64, note string) (int64, []string, error) {
	var project, kind, body string
	err := s.db.QueryRow(`SELECT project, kind, body FROM notices WHERE id = ?`, ruleID).Scan(&project, &kind, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, fmt.Errorf("notice %d: %w", ruleID, ErrNotFound)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("read notice %d: %w", ruleID, err)
	}
	if project != by.Project {
		return 0, nil, fmt.Errorf("notice %d belongs to project %q, not %q", ruleID, project, by.Project)
	}
	if kind != KindRule {
		return 0, nil, fmt.Errorf("notice %d is not a standing rule", ruleID)
	}
	res, err := s.db.Exec(`UPDATE notices AS n SET lifted_at = ?, lifted_by = ? WHERE n.id = ? AND `+activeRule,
		s.unix(), by.ID, ruleID, s.unix())
	if err != nil {
		return 0, nil, fmt.Errorf("lift rule %d: %w", ruleID, err)
	}
	if changed, _ := res.RowsAffected(); changed == 0 {
		return 0, nil, fmt.Errorf("rule %d is no longer active (already lifted or expired)", ruleID)
	}
	id, to, err := s.Send(NewNotice{From: by, Origin: origin, PromptID: promptID,
		Body: liftBody(ruleID, body, note), lifts: ruleID})
	if err != nil {
		_, _ = s.db.Exec(`UPDATE notices SET lifted_at = NULL, lifted_by = NULL WHERE id = ?`, ruleID)
		return 0, nil, err
	}
	return id, to, nil
}

func liftBody(ruleID int64, rule, note string) string {
	quoted := strings.Join(strings.Fields(rule), " ")
	if len(quoted) > 300 {
		quoted = clip(quoted, 300) + "…"
	}
	head := fmt.Sprintf("Lifted rule #%d (%q).", ruleID, quoted)
	note = strings.TrimSpace(note)
	if note == "" {
		return head
	}
	if max := MaxBodyBytes - len(head) - 1; len(note) > max {
		note = clip(note, max)
	}
	return head + "\n" + note
}

// clip cuts s to at most n bytes without splitting a UTF-8 sequence.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
