// Package migrate moves cestafet's data from an old location to a new one
// exactly once, safely under concurrent first opens. It is platform-neutral:
// the caller supplies the candidate copies (on Windows, the real AppData copy
// and the copies the Claude desktop app's MSIX package redirected writes to).
//
// A copy is a pair: a database and a projects file. The newest non-empty copy
// wins and supplies both, so a database is never paired with another copy's
// project names. A marker file records the chosen source before anything is
// published; until it says "done", the target must not be opened.
package migrate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Copy is one candidate data location.
type Copy struct {
	Name     string // short label for reports, e.g. "AppData" or the package folder
	DB       string // bus.db path; may not exist
	Projects string // projects.toml path; may not exist
	// Redirectable marks a path the OS may redirect into another candidate's
	// files (MSIX virtualization from inside a package). When it turns out to be
	// the same file as a later candidate, that candidate's name and paths are used.
	Redirectable bool
}

// Target is the new data location.
type Target struct {
	Dir      string // holds the marker and the lock
	DB       string
	Projects string
}

// Marker states.
const (
	StateDone    = "done"
	StatePending = "pending"
	StateFailed  = "failed"
	StateBlocked = "blocked"
)

// Marker is the persisted record of a migration, written to Target.Dir.
type Marker struct {
	State        string    `json:"state"`
	Source       string    `json:"source,omitempty"` // Copy.Name of the winner; empty when nothing was migrated
	SourceDB     string    `json:"source_db,omitempty"`
	SourceProj   string    `json:"source_projects,omitempty"`
	Backup       string    `json:"backup,omitempty"` // --replace: where the previous target went
	MovedAside   bool      `json:"moved_aside,omitempty"`
	DBDone       bool      `json:"db_done,omitempty"`
	ProjectsDone bool      `json:"projects_done,omitempty"`
	Time         time.Time `json:"time"`
	Error        string    `json:"error,omitempty"`
}

// Errors returned while a migration is not finished; the target must not be opened.
var (
	ErrBlocked  = errors.New("migration blocked")
	ErrDeferred = errors.New("migration deferred")
)

// Options are the injectable parts.
type Options struct {
	// Rewrite canonicalises a stored path key (8.3 short names on Windows).
	// Nil means keys are kept as they are.
	Rewrite func(string) string
	// ValidateProjects parses a projects file and returns it re-encoded with
	// Rewrite applied to its paths. Nil copies the file unchanged.
	ValidateProjects func(data []byte) ([]byte, error)
	Now              func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

const markerName = "migration.json"

// ReadMarker returns the marker in dir; ok is false when there is none.
func ReadMarker(dir string) (Marker, bool, error) {
	var data []byte
	var err error
	// On Windows a read can collide with a concurrent replace (sharing
	// violation) for a moment: retry briefly.
	for attempt := 0; attempt < 20; attempt++ {
		data, err = os.ReadFile(filepath.Join(dir, markerName))
		if err == nil || os.IsNotExist(err) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if os.IsNotExist(err) {
		return Marker{}, false, nil
	}
	if err != nil {
		return Marker{}, false, fmt.Errorf("read migration marker: %w", err)
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return Marker{}, false, fmt.Errorf("parse migration marker: %w", err)
	}
	return m, true, nil
}

func writeMarker(dir string, m Marker) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, markerName+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write migration marker: %w", err)
	}
	// Windows refuses to replace a file another process is reading for a
	// moment (a concurrent ReadMarker outside the lock): retry briefly.
	var rerr error
	for attempt := 0; attempt < 20; attempt++ {
		if rerr = os.Rename(tmp, filepath.Join(dir, markerName)); rerr == nil {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("write migration marker: %w", rerr)
}

// Ensure makes sure the target is ready to open, migrating the newest
// candidate copy on first use. It is cheap once the marker says done.
// ErrBlocked and ErrDeferred mean: do not open the target now.
func Ensure(t Target, candidates []Copy, o Options) (Marker, error) {
	if m, ok, err := ReadMarker(t.Dir); err != nil {
		return Marker{}, err
	} else if ok && m.State == StateDone {
		return m, nil
	} else if !ok && exists(t.DB) {
		return Marker{State: StateDone}, nil // nothing to migrate: the target predates any marker
	}
	if err := os.MkdirAll(t.Dir, 0o700); err != nil {
		return Marker{}, fmt.Errorf("create data directory: %w", err)
	}
	unlock, err := lock(filepath.Join(t.Dir, "migrate.lock"))
	if err != nil {
		return Marker{}, err
	}
	defer unlock()

	// Another process may have finished while this one waited for the lock.
	m, ok, err := ReadMarker(t.Dir)
	if err != nil {
		return Marker{}, err
	}
	switch {
	case ok && m.State == StateDone:
		return m, nil
	case ok && m.State == StatePending:
		return finish(t, m, o)
	case !ok && exists(t.DB):
		return Marker{State: StateDone}, nil
	}
	return choose(t, candidates, o)
}

// choose ranks the candidates and migrates the winner.
func choose(t Target, candidates []Copy, o Options) (Marker, error) {
	if orphan := Orphans(candidates); len(orphan) > 0 {
		m := Marker{State: StateBlocked, Time: o.now(),
			Error: fmt.Sprintf("database journal without its database: %v", orphan)}
		if err := writeMarker(t.Dir, m); err != nil {
			return Marker{}, err
		}
		return m, fmt.Errorf("%w: %s", ErrBlocked, m.Error)
	}
	ranked, err := Rank(candidates)
	if err != nil {
		m := Marker{State: StateFailed, Time: o.now(), Error: err.Error()}
		if werr := writeMarker(t.Dir, m); werr != nil {
			return Marker{}, werr
		}
		return m, fmt.Errorf("%w: %v", ErrDeferred, err)
	}
	if len(ranked) == 0 || ranked[0].Empty {
		m := Marker{State: StateDone, Time: o.now()}
		return m, writeMarker(t.Dir, m)
	}
	win := ranked[0].Copy
	m := Marker{State: StatePending, Source: win.Name, SourceDB: win.DB, SourceProj: win.Projects, Time: o.now()}
	if err := writeMarker(t.Dir, m); err != nil {
		return Marker{}, err
	}
	return finish(t, m, o)
}

// finish completes a pending migration from its recorded source, step by step,
// recording each step so an interruption resumes where it stopped.
func finish(t Target, m Marker, o Options) (Marker, error) {
	fail := func(err error) (Marker, error) {
		// Stay pending: the recorded source and finished steps are kept for the retry.
		m.Error = err.Error()
		_ = writeMarker(t.Dir, m)
		return m, fmt.Errorf("%w: %v", ErrDeferred, err)
	}
	if m.Backup != "" && !m.MovedAside {
		if err := moveAside(t, m.Backup); err != nil {
			return fail(err)
		}
		m.MovedAside = true
		if err := writeMarker(t.Dir, m); err != nil {
			return fail(err)
		}
	}
	if !m.DBDone {
		if err := copyDB(m.SourceDB, t.DB, o); err != nil {
			return fail(err)
		}
		m.DBDone = true
		if err := writeMarker(t.Dir, m); err != nil {
			return fail(err)
		}
	}
	if !m.ProjectsDone {
		if err := copyProjects(m.SourceProj, t.Projects, o); err != nil {
			return fail(err)
		}
		m.ProjectsDone = true
	}
	m.State, m.Error, m.Time = StateDone, "", o.now()
	if err := writeMarker(t.Dir, m); err != nil {
		return fail(err)
	}
	return m, nil
}

// moveAside moves an existing target pair into the backup directory.
func moveAside(t Target, backup string) error {
	if err := os.MkdirAll(backup, 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	for _, p := range []string{t.DB, t.DB + "-wal", t.DB + "-shm", t.Projects} {
		if !exists(p) {
			continue
		}
		if err := os.Rename(p, filepath.Join(backup, filepath.Base(p))); err != nil {
			return fmt.Errorf("move %s aside: %w", p, err)
		}
	}
	return nil
}

// copyDB snapshots src with VACUUM INTO next to dst, checks it, rewrites path
// keys, and publishes it without replacing an existing file.
func copyDB(src, dst string, o Options) error {
	if src == "" || !exists(src) {
		return nil // the winning copy had no database file (projects only)
	}
	if exists(dst) {
		return nil // published by an earlier attempt that stopped before recording it
	}
	tmp := dst + ".migrating"
	_ = os.Remove(tmp)
	defer os.Remove(tmp)
	if err := snapshot(src, tmp); err != nil {
		return err
	}
	if err := checkAndRewrite(tmp, o.Rewrite); err != nil {
		return err
	}
	return publish(tmp, dst)
}

func copyProjects(src, dst string, o Options) error {
	if src == "" || !exists(src) || exists(dst) {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if o.ValidateProjects != nil {
		if data, err = o.ValidateProjects(data); err != nil {
			return fmt.Errorf("projects file %s: %w", src, err)
		}
	}
	tmp := dst + ".migrating"
	_ = os.Remove(tmp)
	defer os.Remove(tmp)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	return publish(tmp, dst)
}

// publish moves tmp to dst only if dst does not exist: a hard link fails on an
// existing name on every platform, unlike a rename on Windows.
func publish(tmp, dst string) error {
	if err := os.Link(tmp, dst); err != nil {
		if exists(dst) {
			return nil // another writer published first; its copy is from the same source
		}
		return fmt.Errorf("publish %s: %w", dst, err)
	}
	return nil
}

func snapshot(src, tmp string) error {
	db, err := sql.Open("sqlite", inspectURI(src))
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer db.Close()
	if _, err := db.Exec(`VACUUM INTO ?`, tmp); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return nil
}

func exists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// Replace migrates from an explicitly chosen copy (cestafet migrate --from).
// An existing target is refused unless replace is set; then it is moved to
// Dir/replaced-<time> first, recorded in the marker so an interruption either
// finishes the new pair or leaves the backup as the way back. The caller makes
// sure no session is using the target.
func Replace(t Target, src Copy, replace bool, o Options) (Marker, error) {
	if !exists(src.DB) && !exists(src.Projects) {
		return Marker{}, fmt.Errorf("%s has neither a database nor a projects file", src.Name)
	}
	if err := os.MkdirAll(t.Dir, 0o700); err != nil {
		return Marker{}, fmt.Errorf("create data directory: %w", err)
	}
	unlock, err := lock(filepath.Join(t.Dir, "migrate.lock"))
	if err != nil {
		return Marker{}, err
	}
	defer unlock()
	m := Marker{State: StatePending, Source: src.Name, SourceDB: src.DB, SourceProj: src.Projects, Time: o.now()}
	if exists(t.DB) || exists(t.Projects) {
		if !replace {
			return Marker{}, fmt.Errorf("%s already has data; use --replace to move it aside", t.Dir)
		}
		m.Backup = filepath.Join(t.Dir, "replaced-"+o.now().UTC().Format("20060102T150405Z"))
	}
	if err := writeMarker(t.Dir, m); err != nil {
		return Marker{}, err
	}
	return finish(t, m, o)
}
