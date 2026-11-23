package migrate

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Ranked is a candidate with what a read-only look at it found.
type Ranked struct {
	Copy     Copy
	Activity time.Time // latest session or notice; projects file mtime when there is no database
	Empty    bool      // no sessions and no notices (and no projects file)
	Corrupt  string    // why it was skipped, if it was
}

// Rank orders candidates: newest activity first, empty copies last, ties by
// input order (the caller lists the real copy before package copies, and
// package copies in lexical order). Corrupt candidates are dropped. A locked
// candidate fails the whole ranking, so a busy newest copy never loses to an
// older one; the caller retries later. Duplicates of one file are removed.
func Rank(candidates []Copy) ([]Ranked, error) {
	var out []Ranked
	for _, c := range dedupe(candidates) {
		r, err := inspect(c)
		if err != nil {
			return nil, err
		}
		if r.Corrupt != "" {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Empty != out[j].Empty {
			return !out[i].Empty
		}
		return out[i].Activity.After(out[j].Activity)
	})
	return out, nil
}

// Inspect looks at every candidate read-only, keeping corrupt ones, for reports.
func Inspect(candidates []Copy) []Ranked {
	var out []Ranked
	for _, c := range dedupe(candidates) {
		r, err := inspect(c)
		if err != nil {
			r = Ranked{Copy: c, Corrupt: err.Error()}
		}
		out = append(out, r)
	}
	return out
}

func inspect(c Copy) (Ranked, error) {
	r := Ranked{Copy: c, Empty: true}
	if fi, err := os.Stat(c.Projects); err == nil {
		r.Empty = false
		r.Activity = fi.ModTime()
	}
	if !exists(c.DB) {
		return r, nil
	}
	db, err := sql.Open("sqlite", readOnlyURI(c.DB))
	if err != nil {
		r.Corrupt = err.Error()
		return r, nil
	}
	defer db.Close()
	var rows, last int64
	err = db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM sessions) + (SELECT COUNT(*) FROM notices),
		MAX(COALESCE((SELECT MAX(last_seen) FROM sessions), 0), COALESCE((SELECT MAX(created_at) FROM notices), 0))`).
		Scan(&rows, &last)
	if err != nil {
		if isBusy(err) {
			return r, fmt.Errorf("%s is locked: %w", c.DB, err)
		}
		r.Corrupt = err.Error()
		return r, nil
	}
	if rows > 0 {
		r.Empty = false
		if t := time.Unix(last, 0); t.After(r.Activity) {
			r.Activity = t
		}
	}
	return r, nil
}

func isBusy(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "locked") || strings.Contains(s, "busy")
}

// Orphans lists journal files (-wal, -shm) whose database is not beside them,
// e.g. redirected into another folder by package virtualization. Copying a
// database without its journal could silently drop committed changes.
func Orphans(candidates []Copy) []string {
	var out []string
	for _, c := range candidates {
		if c.DB == "" || exists(c.DB) {
			continue
		}
		for _, j := range []string{c.DB + "-wal", c.DB + "-shm"} {
			if exists(j) {
				out = append(out, j)
			}
		}
	}
	return out
}

func dedupe(candidates []Copy) []Copy {
	var out []Copy
	var seen []os.FileInfo
	for _, c := range candidates {
		fi, err := os.Stat(c.DB)
		if err != nil {
			fi, err = os.Stat(c.Projects)
		}
		if err == nil {
			dup := false
			for _, s := range seen {
				if os.SameFile(s, fi) {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			seen = append(seen, fi)
		}
		out = append(out, c)
	}
	return out
}

// readOnlyURI opens a database without writing to it, failing fast when locked.
func readOnlyURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p, RawQuery: "mode=ro&_pragma=busy_timeout(2000)"}
	return u.String()
}

// checkAndRewrite verifies a snapshot and rewrites path keys in it.
func checkAndRewrite(path string, rewrite func(string) string) error {
	u := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return fmt.Errorf("open snapshot: %w", err)
	}
	defer db.Close()
	var res string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&res); err != nil {
		return fmt.Errorf("check snapshot: %w", err)
	}
	if res != "ok" {
		return fmt.Errorf("snapshot failed integrity check: %s", res)
	}
	if rewrite == nil {
		return nil
	}
	return rewriteKeys(db, rewrite)
}

// rewriteKeys applies rewrite to every distinct project and repo key; values it
// leaves unchanged (remotes, names, already-long paths) are untouched.
func rewriteKeys(db *sql.DB, rewrite func(string) string) error {
	targets := []struct{ table, col string }{
		{"sessions", "project"}, {"sessions", "repo"}, {"notices", "project"},
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, t := range targets {
		rows, err := tx.Query(`SELECT DISTINCT ` + t.col + ` FROM ` + t.table)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("read %s.%s: %w", t.table, t.col, err)
		}
		var vals []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err == nil {
				vals = append(vals, v)
			}
		}
		rows.Close()
		for _, v := range vals {
			if nv := rewrite(v); nv != v {
				if _, err := tx.Exec(`UPDATE `+t.table+` SET `+t.col+` = ? WHERE `+t.col+` = ?`, nv, v); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("rewrite %s.%s: %w", t.table, t.col, err)
				}
			}
		}
	}
	return tx.Commit()
}
