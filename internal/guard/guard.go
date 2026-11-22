// Package guard matches shell commands against the glob patterns a standing
// rule may carry, so a session is asked before it runs a command the rule forbids.
package guard

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Limits on a rule's patterns.
const (
	MaxPatterns     = 5
	MaxPatternLen   = 120
	minLiteralChars = 3
)

// Validate normalises patterns (collapsed whitespace) and rejects lists that are
// too long, empty patterns and patterns broad enough to match almost anything.
func Validate(patterns []string) ([]string, error) {
	if len(patterns) > MaxPatterns {
		return nil, fmt.Errorf("a rule takes at most %d guard patterns, got %d", MaxPatterns, len(patterns))
	}
	var out []string
	for _, p := range patterns {
		p = normalise(p)
		switch {
		case p == "":
			return nil, errors.New("guard pattern is empty")
		case len(p) > MaxPatternLen:
			return nil, fmt.Errorf("guard pattern %q is longer than %d characters", p, MaxPatternLen)
		case literalChars(p) < minLiteralChars:
			return nil, fmt.Errorf("guard pattern %q is too broad: it needs at least %d characters besides * and ?", p, minLiteralChars)
		}
		out = append(out, p)
	}
	return out, nil
}

// Match reports whether command matches pattern: * is any text, ? one
// character, everything else literal; case-insensitive, whitespace collapsed,
// matched against the whole command and against each of its lines.
func Match(pattern, command string) bool {
	re := compile(normalise(pattern))
	if re.MatchString(normalise(command)) {
		return true
	}
	for _, line := range strings.Split(command, "\n") {
		if re.MatchString(normalise(line)) {
			return true
		}
	}
	return false
}

func compile(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("(?i)^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func normalise(s string) string { return strings.Join(strings.Fields(s), " ") }

func literalChars(p string) int {
	return len(strings.NewReplacer("*", "", "?", "", " ", "").Replace(p))
}
