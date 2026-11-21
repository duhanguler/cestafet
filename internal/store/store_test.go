package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func register(t *testing.T, s *Store, id, project string) Session {
	t.Helper()
	in := Session{ID: id, ClaudePID: len(id) + 100, Profile: "p-" + id, Project: project, Cwd: "/w/" + id}
	if _, err := s.Register(in); err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	return got
}

func TestOpenCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "bus.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("db mode = %o, want 600", perm)
	}
}

func TestRegisterKeepsSummaryAndRoleOnRefresh(t *testing.T) {
	s := newStore(t)
	register(t, s, "a", "proj")
	if err := s.SetSummary("a", "auth refactor"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MakeCoordinator("a"); err != nil {
		t.Fatal(err)
	}
	isNew, err := s.Register(Session{ID: "a", ClaudePID: 999, Profile: "p", Project: "proj", Cwd: "/w"})
	if err != nil || isNew {
		t.Fatalf("re-register: new=%v err=%v", isNew, err)
	}
	got, _ := s.Get("a")
	if got.Summary != "auth refactor" || got.Role != RoleCoordinator || got.ClaudePID != 999 {
		t.Fatalf("unexpected session after refresh: %+v", got)
	}
}

func TestSingleCoordinatorPerProject(t *testing.T) {
	s := newStore(t)
	register(t, s, "a", "proj")
	register(t, s, "b", "proj")
	if prev, err := s.MakeCoordinator("a"); err != nil || prev != "" {
		t.Fatalf("first coordinator: prev=%q err=%v", prev, err)
	}
	prev, err := s.MakeCoordinator("b")
	if err != nil || prev != "a" {
		t.Fatalf("takeover: prev=%q err=%v", prev, err)
	}
	c, err := s.Coordinator("proj")
	if err != nil || c.ID != "b" {
		t.Fatalf("coordinator = %+v, %v", c, err)
	}
	a, _ := s.Get("a")
	if a.Role != RoleMember {
		t.Fatalf("old coordinator role = %s", a.Role)
	}
}

func TestSendReachesOtherSessionsOfProjectOnly(t *testing.T) {
	s := newStore(t)
	from := register(t, s, "coord", "proj")
	register(t, s, "m1", "proj")
	register(t, s, "m2", "proj")
	register(t, s, "other", "elsewhere")

	id, to, err := s.Send(NewNotice{From: from, Origin: OriginAgent, Body: "no deploys to staging"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(to) != 2 {
		t.Fatalf("recipients = %v, want m1 and m2", to)
	}
	for _, sid := range []string{"m1", "m2"} {
		got, err := s.TakePending(sid)
		if err != nil || len(got) != 1 || got[0].ID != id {
			t.Fatalf("pending for %s = %+v, %v", sid, got, err)
		}
		again, _ := s.TakePending(sid)
		if len(again) != 0 {
			t.Fatalf("notice delivered twice to %s", sid)
		}
	}
	for _, sid := range []string{"coord", "other"} {
		if got, _ := s.TakePending(sid); len(got) != 0 {
			t.Fatalf("%s must not receive the notice", sid)
		}
	}
}

func TestSendRejectsBadInput(t *testing.T) {
	s := newStore(t)
	from := register(t, s, "a", "proj")
	register(t, s, "x", "elsewhere")
	cases := map[string]NewNotice{
		"empty":         {From: from, Origin: OriginAgent, Body: "   "},
		"too long":      {From: from, Origin: OriginAgent, Body: strings.Repeat("x", MaxBodyBytes+1)},
		"bad origin":    {From: from, Origin: "boss", Body: "hi"},
		"other project": {From: from, Origin: OriginAgent, Body: "hi", To: []string{"x"}},
	}
	for name, n := range cases {
		if _, _, err := s.Send(n); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestUserOriginCarriesPromptText(t *testing.T) {
	s := newStore(t)
	from := register(t, s, "coord", "proj")
	register(t, s, "m", "proj")
	pid, err := s.RecordPrompt("coord", "tell the others: testing started")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Send(NewNotice{From: from, Origin: OriginUser, PromptID: pid, Body: "Testing started."}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.TakePending("m")
	if len(got) != 1 || got[0].Origin != OriginUser || got[0].PromptText != "tell the others: testing started" {
		t.Fatalf("notice = %+v", got)
	}
}

func TestBackfillGivesLateSessionRecentNotices(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_800_000_000, 0)
	s.SetClock(func() time.Time { return now })
	from := register(t, s, "coord", "proj")
	if _, _, err := s.Send(NewNotice{From: from, Origin: OriginAgent, Body: "old"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(BackfillWindow + time.Hour)
	if _, _, err := s.Send(NewNotice{From: from, Origin: OriginAgent, Body: "recent"}); err != nil {
		t.Fatal(err)
	}
	register(t, s, "late", "proj")
	if err := s.Backfill("late", "proj"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.TakePending("late")
	if len(got) != 1 || got[0].Body != "recent" {
		t.Fatalf("backfilled = %+v", got)
	}
}

func TestAckAndStatus(t *testing.T) {
	s := newStore(t)
	from := register(t, s, "coord", "proj")
	register(t, s, "m", "proj")
	id, _, _ := s.Send(NewNotice{From: from, Origin: OriginAgent, Body: "hi"})
	if err := s.Ack(id, "m", "ok, paused deploys"); err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(id, "coord", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ack by sender: %v", err)
	}
	st, err := s.Status(id)
	if err != nil || len(st) != 1 || st[0].AckedAt.IsZero() || st[0].Reply != "ok, paused deploys" {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if unacked, _ := s.Received("m", true, 10); len(unacked) != 0 {
		t.Fatalf("acked notice still listed as unacked")
	}
}

func TestIdleAndGone(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_800_000_000, 0)
	s.SetClock(func() time.Time { return now })
	register(t, s, "a", "proj")
	now = now.Add(IdleAfter + time.Minute)
	a, _ := s.Get("a")
	if a.Status != StatusIdle {
		t.Fatalf("status = %s, want idle", a.Status)
	}
	if err := s.End("a"); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List("proj")
	if len(list) != 0 {
		t.Fatalf("gone session still listed: %+v", list)
	}
	if _, err := s.ByClaudePID(a.ClaudePID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("gone session found by pid: %v", err)
	}
}
