package migrate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orkono/cestafet/internal/store"
)

// makeCopy creates a candidate with sessions last seen at the given time
// (zero: an empty database) and a projects file.
func makeCopy(t *testing.T, name string, seen time.Time, projects string) Copy {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	c := Copy{Name: name, DB: filepath.Join(dir, "bus.db"), Projects: filepath.Join(dir, "projects.toml")}
	st, err := store.Open(c.DB)
	if err != nil {
		t.Fatal(err)
	}
	if !seen.IsZero() {
		st.SetClock(func() time.Time { return seen })
		if _, err := st.Register(store.Session{ID: name + "-s", Profile: "p", Project: "proj-" + name, Cwd: "/x"}); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.Close()
	if projects != "" {
		if err := os.WriteFile(c.Projects, []byte(projects), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func newTarget(t *testing.T) Target {
	dir := filepath.Join(t.TempDir(), ".cestafet")
	return Target{Dir: dir, DB: filepath.Join(dir, "bus.db"), Projects: filepath.Join(dir, "projects.toml")}
}

func projectsOf(t *testing.T, db string) []string {
	t.Helper()
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	list, err := st.Active()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range list {
		out = append(out, s.Project)
	}
	return out
}

var t0 = time.Unix(1_800_000_000, 0)

func TestNewestCopyWinsAndSuppliesBothFiles(t *testing.T) {
	old := makeCopy(t, "real", t0, "# real\n")
	pkg := makeCopy(t, "pkg", t0.Add(time.Hour), "# pkg\n")
	tg := newTarget(t)
	m, err := Ensure(tg, []Copy{old, pkg}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.State != StateDone || m.Source != "pkg" {
		t.Fatalf("marker %+v", m)
	}
	if got := projectsOf(t, tg.DB); len(got) != 1 || got[0] != "proj-pkg" {
		t.Fatalf("db sessions %v", got)
	}
	if data, _ := os.ReadFile(tg.Projects); string(data) != "# pkg\n" {
		t.Fatalf("projects file %q", data)
	}
	// Later opens are a cheap no-op.
	if m2, err := Ensure(tg, []Copy{old, pkg}, Options{}); err != nil || m2.Source != "pkg" {
		t.Fatalf("second ensure %+v %v", m2, err)
	}
}

func TestEmptyCopyLosesToOlderNonEmpty(t *testing.T) {
	empty := makeCopy(t, "empty", time.Time{}, "")
	full := makeCopy(t, "full", t0, "")
	m, err := Ensure(newTarget(t), []Copy{empty, full}, Options{})
	if err != nil || m.Source != "full" {
		t.Fatalf("marker %+v %v", m, err)
	}
}

func TestTiesKeepInputOrder(t *testing.T) {
	a := makeCopy(t, "real", t0, "")
	b := makeCopy(t, "pkg", t0, "")
	m, err := Ensure(newTarget(t), []Copy{a, b}, Options{})
	if err != nil || m.Source != "real" {
		t.Fatalf("marker %+v %v", m, err)
	}
}

func TestNothingToMigrateCreatesNoTarget(t *testing.T) {
	tg := newTarget(t)
	m, err := Ensure(tg, nil, Options{})
	if err != nil || m.State != StateDone || m.Source != "" {
		t.Fatalf("marker %+v %v", m, err)
	}
	if exists(tg.DB) {
		t.Fatal("ensure created the database")
	}
}

func TestExistingTargetWithoutMarkerIsLeftAlone(t *testing.T) {
	tg := newTarget(t)
	st, err := store.Open(tg.DB)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	c := makeCopy(t, "real", t0, "")
	m, err := Ensure(tg, []Copy{c}, Options{})
	if err != nil || m.Source != "" {
		t.Fatalf("marker %+v %v", m, err)
	}
	if got := projectsOf(t, tg.DB); len(got) != 0 {
		t.Fatalf("target was overwritten: %v", got)
	}
}

func TestOrphanJournalBlocks(t *testing.T) {
	c := makeCopy(t, "real", t0, "")
	orphanDir := filepath.Join(t.TempDir(), "pkg")
	_ = os.MkdirAll(orphanDir, 0o700)
	orphan := Copy{Name: "pkg", DB: filepath.Join(orphanDir, "bus.db"), Projects: filepath.Join(orphanDir, "projects.toml")}
	if err := os.WriteFile(orphan.DB+"-wal", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tg := newTarget(t)
	m, err := Ensure(tg, []Copy{c, orphan}, Options{})
	if !errors.Is(err, ErrBlocked) || m.State != StateBlocked {
		t.Fatalf("marker %+v err %v", m, err)
	}
	if exists(tg.DB) {
		t.Fatal("blocked migration created the target")
	}
	// A journal beside its own database is normal.
	if err := os.WriteFile(c.DB+"-wal", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if len(Orphans([]Copy{c})) != 0 {
		t.Fatal("own journal reported as orphan")
	}
}

// An interruption after the database was published resumes from the recorded
// source, even if another copy has become newer since.
func TestResumeCompletesThePairFromTheRecordedSource(t *testing.T) {
	a := makeCopy(t, "real", t0, "# real\n")
	tg := newTarget(t)
	_ = os.MkdirAll(tg.Dir, 0o700)
	if err := copyDB(a.DB, tg.DB, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(tg.Dir, Marker{State: StatePending, Source: "real", SourceDB: a.DB,
		SourceProj: a.Projects, DBDone: true, Time: t0}); err != nil {
		t.Fatal(err)
	}
	newer := makeCopy(t, "pkg", t0.Add(time.Hour), "# pkg\n")
	m, err := Ensure(tg, []Copy{a, newer}, Options{})
	if err != nil || m.State != StateDone || m.Source != "real" {
		t.Fatalf("marker %+v %v", m, err)
	}
	if data, _ := os.ReadFile(tg.Projects); string(data) != "# real\n" {
		t.Fatalf("projects file paired from another copy: %q", data)
	}
}

func TestConcurrentFirstOpensMigrateOnce(t *testing.T) {
	c := makeCopy(t, "real", t0, "# real\n")
	tg := newTarget(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Ensure(tg, []Copy{c}, Options{}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := projectsOf(t, tg.DB); len(got) != 1 || got[0] != "proj-real" {
		t.Fatalf("sessions %v", got)
	}
}

func TestRewriteAppliesToPathKeys(t *testing.T) {
	c := makeCopy(t, "real", t0, "")
	rw := func(s string) string { return strings.Replace(s, "proj-", "long-", 1) }
	tg := newTarget(t)
	if _, err := Ensure(tg, []Copy{c}, Options{Rewrite: rw}); err != nil {
		t.Fatal(err)
	}
	if got := projectsOf(t, tg.DB); len(got) != 1 || got[0] != "long-real" {
		t.Fatalf("sessions %v", got)
	}
}

func TestReplaceRefusesThenMovesAside(t *testing.T) {
	tg := newTarget(t)
	first := makeCopy(t, "real", t0, "# real\n")
	if _, err := Ensure(tg, []Copy{first}, Options{}); err != nil {
		t.Fatal(err)
	}
	other := makeCopy(t, "pkg", t0, "# pkg\n")
	if _, err := Replace(tg, other, false, Options{}); err == nil {
		t.Fatal("replace without --replace should refuse")
	}
	m, err := Replace(tg, other, true, Options{Now: func() time.Time { return t0 }})
	if err != nil || m.State != StateDone || m.Backup == "" {
		t.Fatalf("marker %+v %v", m, err)
	}
	if got := projectsOf(t, tg.DB); len(got) != 1 || got[0] != "proj-pkg" {
		t.Fatalf("sessions %v", got)
	}
	if !exists(filepath.Join(m.Backup, "bus.db")) || !exists(filepath.Join(m.Backup, "projects.toml")) {
		t.Fatal("previous target not kept in the backup")
	}
}

// Interrupted right after moving the old target aside: the retry finishes the new pair.
func TestReplaceResumesAfterMoveAside(t *testing.T) {
	tg := newTarget(t)
	src := makeCopy(t, "pkg", t0, "# pkg\n")
	_ = os.MkdirAll(tg.Dir, 0o700)
	backup := filepath.Join(tg.Dir, "replaced-x")
	if err := writeMarker(tg.Dir, Marker{State: StatePending, Source: "pkg", SourceDB: src.DB,
		SourceProj: src.Projects, Backup: backup, MovedAside: true, Time: t0}); err != nil {
		t.Fatal(err)
	}
	m, err := Ensure(tg, nil, Options{})
	if err != nil || m.State != StateDone {
		t.Fatalf("marker %+v %v", m, err)
	}
	if data, _ := os.ReadFile(tg.Projects); string(data) != "# pkg\n" {
		t.Fatalf("projects %q", data)
	}
}
