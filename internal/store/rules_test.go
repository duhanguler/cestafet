package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func sendRule(t *testing.T, s *Store, from Session, body string, expires time.Duration) int64 {
	t.Helper()
	id, _, err := s.Send(NewNotice{From: from, Origin: OriginUser, Body: body, Kind: KindRule, ExpiresIn: expires})
	if err != nil {
		t.Fatalf("send rule: %v", err)
	}
	return id
}

func activeIDs(t *testing.T, s *Store, project string) []int64 {
	t.Helper()
	rules, err := s.ActiveRules(project)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range rules {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestRuleStaysActiveUntilLifted(t *testing.T) {
	s := newStore(t)
	coord := register(t, s, "coord", "proj")
	register(t, s, "m", "proj")
	id := sendRule(t, s, coord, "no deploys to staging", 0)

	if got := activeIDs(t, s, "proj"); len(got) != 1 || got[0] != id {
		t.Fatalf("active = %v", got)
	}
	liftID, to, err := s.Lift(id, coord, OriginUser, 0, "testing finished, deploys allowed")
	if err != nil {
		t.Fatalf("lift: %v", err)
	}
	if len(to) != 1 || to[0] != "m" {
		t.Fatalf("lift recipients = %v", to)
	}
	if got := activeIDs(t, s, "proj"); len(got) != 0 {
		t.Fatalf("lifted rule still active: %v", got)
	}
	pending, _ := s.TakePending("m")
	if len(pending) != 2 || pending[1].ID != liftID || pending[1].Lifts != id || pending[1].Kind != KindInfo {
		t.Fatalf("pending = %+v", pending)
	}
	if !strings.Contains(pending[1].Body, "testing finished") {
		t.Fatalf("lift body = %q", pending[1].Body)
	}
	if _, _, err := s.Lift(id, coord, OriginUser, 0, ""); err == nil {
		t.Fatal("lifting twice must fail")
	}
}

func TestLiftRejectsInfoAndOtherProject(t *testing.T) {
	s := newStore(t)
	coord := register(t, s, "coord", "proj")
	other := register(t, s, "x", "elsewhere")
	info, _, _ := s.Send(NewNotice{From: coord, Origin: OriginAgent, Body: "fyi"})
	if _, _, err := s.Lift(info, coord, OriginAgent, 0, ""); err == nil {
		t.Error("lifting an info notice must fail")
	}
	rule := sendRule(t, s, coord, "freeze", 0)
	if _, _, err := s.Lift(rule, other, OriginAgent, 0, ""); err == nil {
		t.Error("lifting another project's rule must fail")
	}
	if _, _, err := s.Lift(999, coord, OriginAgent, 0, ""); err == nil {
		t.Error("lifting a missing notice must fail")
	}
}

func TestRuleExpires(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_800_000_000, 0)
	s.SetClock(func() time.Time { return now })
	coord := register(t, s, "coord", "proj")
	sendRule(t, s, coord, "freeze for two hours", 2*time.Hour)
	if got := activeIDs(t, s, "proj"); len(got) != 1 {
		t.Fatalf("active before expiry = %v", got)
	}
	now = now.Add(2*time.Hour + time.Second)
	if got := activeIDs(t, s, "proj"); len(got) != 0 {
		t.Fatalf("expired rule still active: %v", got)
	}
}

func TestSendValidatesRuleFields(t *testing.T) {
	s := newStore(t)
	coord := register(t, s, "coord", "proj")
	cases := map[string]NewNotice{
		"bad kind":       {From: coord, Origin: OriginAgent, Body: "x", Kind: "order"},
		"expiry on info": {From: coord, Origin: OriginAgent, Body: "x", ExpiresIn: time.Hour},
		"negative":       {From: coord, Origin: OriginAgent, Body: "x", Kind: KindRule, ExpiresIn: -time.Hour},
		"too long":       {From: coord, Origin: OriginAgent, Body: "x", Kind: KindRule, ExpiresIn: MaxRuleLifetime + time.Hour},
	}
	for name, n := range cases {
		if _, _, err := s.Send(n); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestBackfillGivesActiveRulesOfAnyAge(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_800_000_000, 0)
	s.SetClock(func() time.Time { return now })
	coord := register(t, s, "coord", "proj")
	oldRule := sendRule(t, s, coord, "old rule, still active", 0)
	lifted := sendRule(t, s, coord, "old rule, lifted", 0)
	if _, _, err := s.Lift(lifted, coord, OriginUser, 0, ""); err != nil {
		t.Fatal(err)
	}
	sendRule(t, s, coord, "old rule, expires", time.Hour)
	if _, _, err := s.Send(NewNotice{From: coord, Origin: OriginAgent, Body: "old info"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(BackfillWindow + 2*time.Hour)

	register(t, s, "late", "proj")
	if err := s.Backfill("late", "proj"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.TakePending("late")
	if len(got) != 1 || got[0].ID != oldRule || got[0].Kind != KindRule {
		t.Fatalf("backfilled = %+v", got)
	}
}

func TestBackfillSkipsLiftNotices(t *testing.T) {
	s := newStore(t)
	coord := register(t, s, "coord", "proj")
	rule := sendRule(t, s, coord, "freeze", 0)
	if _, _, err := s.Lift(rule, coord, OriginUser, 0, "done"); err != nil {
		t.Fatal(err)
	}
	register(t, s, "late", "proj")
	if err := s.Backfill("late", "proj"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.TakePending("late"); len(got) != 0 {
		t.Fatalf("late session got %+v", got)
	}
}

func TestOpenMigratesV010Database(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	// The v0.1.0 notices table, with one row.
	if _, err := db.Exec(`CREATE TABLE notices (
		id INTEGER PRIMARY KEY AUTOINCREMENT, project TEXT NOT NULL, from_session TEXT NOT NULL,
		from_profile TEXT NOT NULL, from_role TEXT NOT NULL, origin TEXT NOT NULL, prompt_id INTEGER,
		body TEXT NOT NULL, created_at INTEGER NOT NULL);
		INSERT INTO notices (project, from_session, from_profile, from_role, origin, body, created_at)
		VALUES ('proj', 'a', 'p', 'member', 'agent', 'legacy', 1);`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	for i := 0; i < 2; i++ { // the second open must be a no-op
		s, err := Open(path)
		if err != nil {
			t.Fatalf("open #%d: %v", i, err)
		}
		list, err := s.ProjectNotices("proj", 10)
		if err != nil || len(list) != 1 || list[0].Kind != KindInfo || list[0].Body != "legacy" {
			t.Fatalf("open #%d: notices = %+v, %v", i, list, err)
		}
		_ = s.Close()
	}
}

func TestLiftBodyKeepsUTF8Intact(t *testing.T) {
	body := liftBody(7, strings.Repeat("ş", 400), strings.Repeat("ğ", 3000))
	if !utf8.ValidString(body) || len(body) > MaxBodyBytes {
		t.Fatalf("lift body: valid=%v len=%d", utf8.ValidString(body), len(body))
	}
}

func TestGuardedRules(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_800_000_000, 0)
	s.SetClock(func() time.Time { return now })
	coord := register(t, s, "coord", "proj")
	other := register(t, s, "x", "elsewhere")
	guarded, _, err := s.Send(NewNotice{From: coord, Origin: OriginUser, Kind: KindRule,
		Body: "no deploys to staging", Guards: []string{"*deploy* staging*"}})
	if err != nil {
		t.Fatal(err)
	}
	sendRule(t, s, coord, "unguarded rule", 0)
	short, _, _ := s.Send(NewNotice{From: coord, Origin: OriginUser, Kind: KindRule, Body: "short freeze",
		Guards: []string{"git push * main"}, ExpiresIn: time.Hour})
	lifted, _, _ := s.Send(NewNotice{From: coord, Origin: OriginUser, Kind: KindRule, Body: "old",
		Guards: []string{"*make release*"}})
	if _, _, err := s.Lift(lifted, coord, OriginUser, 0, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Send(NewNotice{From: other, Origin: OriginUser, Kind: KindRule, Body: "theirs",
		Guards: []string{"*terraform apply*"}}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GuardedRules("proj")
	if err != nil || len(got) != 2 || got[0].ID != short || got[1].ID != guarded {
		t.Fatalf("guarded = %+v, %v", got, err)
	}
	if len(got[1].Guards) != 1 || got[1].Guards[0] != "*deploy* staging*" {
		t.Fatalf("guards = %v", got[1].Guards)
	}
	now = now.Add(2 * time.Hour)
	if got, _ := s.GuardedRules("proj"); len(got) != 1 || got[0].ID != guarded {
		t.Fatalf("after expiry = %+v", got)
	}
}

func TestSendValidatesGuards(t *testing.T) {
	s := newStore(t)
	coord := register(t, s, "coord", "proj")
	cases := map[string]NewNotice{
		"guard on info": {From: coord, Origin: OriginAgent, Body: "x", Guards: []string{"*deploy*"}},
		"too broad":     {From: coord, Origin: OriginAgent, Body: "x", Kind: KindRule, Guards: []string{"*"}},
	}
	for name, n := range cases {
		if _, _, err := s.Send(n); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
