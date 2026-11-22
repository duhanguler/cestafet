package guard

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	good := []string{"*deploy* staging*", "kubectl * -n staging *", "git push * main"}
	if got, err := Validate(good); err != nil || len(got) != 3 {
		t.Fatalf("Validate(good) = %v, %v", got, err)
	}
	if got, err := Validate([]string{"  ./deploy.sh   staging  "}); err != nil || got[0] != "./deploy.sh staging" {
		t.Fatalf("whitespace not normalised: %v, %v", got, err)
	}
	if got, err := Validate(nil); err != nil || got != nil {
		t.Fatalf("Validate(nil) = %v, %v", got, err)
	}
	bad := map[string][]string{
		"too many":      {"a*1x", "b*2xx", "c*3xx", "d*4xx", "e*5xx", "f*6xx"},
		"too long":      {strings.Repeat("x", MaxPatternLen+1)},
		"empty":         {"   "},
		"only wildcard": {"*"},
		"too broad":     {"*a*"},
	}
	for name, p := range bad {
		if _, err := Validate(p); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, command string
		want             bool
	}{
		{"*deploy* staging*", "./scripts/deploy.sh staging", true},
		{"*deploy* staging*", "DEPLOY_ENV=1 ./Deploy.sh   STAGING --force", true},
		{"*deploy* staging*", "./scripts/deploy.sh production", false},
		{"*deploy* staging*", "echo staging", false},
		{"kubectl * -n staging *", "kubectl apply -n staging -f app.yaml", true},
		{"kubectl * -n staging *", "kubectl apply -n prod -f app.yaml", false},
		{"git push * main", "git push origin main", true},
		{"git push * main", "git push origin main-fix", false},
		{"git push * main", "cd x && git push origin main", false},
		{"*git push * main*", "cd x && git push origin main", true},
		{"rm -rf ?uild*", "rm -rf build/", true},
		{"*a.b*", "axb", false}, // dots are literal
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.command); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.command, got, c.want)
		}
	}
}

func TestMatchesMultilineCommand(t *testing.T) {
	if !Match("*deploy* staging*", "set -e\n./deploy.sh staging\n") {
		t.Fatal("a guard must match inside a multi-line command")
	}
}
