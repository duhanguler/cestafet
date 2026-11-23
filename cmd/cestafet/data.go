package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/datadir"
	"github.com/orkono/cestafet/internal/migrate"
	"github.com/orkono/cestafet/internal/store"
)

// windowsData returns the data directory and legacy copies when migration
// applies (Windows, no location overridden by environment variables).
func windowsData() (migrate.Target, []migrate.Copy, bool) {
	if runtime.GOOS != "windows" || !datadir.BuiltIn() {
		return migrate.Target{}, nil, false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return migrate.Target{}, nil, false
	}
	return datadir.Target(datadir.WindowsDir(home)), datadir.LegacyCopies(home), true
}

// reportData prints the Windows data section of doctor without changing
// anything. It reports whether opening the database is safe now, i.e. it would
// not start or interrupt a migration.
func reportData() bool {
	t, copies, ok := windowsData()
	if !ok {
		return true
	}
	m, has, err := migrate.ReadMarker(t.Dir)
	fmt.Printf("\nData (Windows):\n  directory   %s\n", tilde(t.Dir))
	switch {
	case err != nil:
		fmt.Printf("  migration   unreadable marker: %v\n", err)
	case !has && fileExists(t.DB):
		fmt.Println("  migration   none needed")
	case !has:
		fmt.Println("  migration   not run yet (runs at the next session start)")
	case m.State == migrate.StateDone && m.Source == "":
		fmt.Printf("  migration   done %s, nothing to move\n", m.Time.Local().Format(time.DateTime))
	case m.State == migrate.StateDone:
		fmt.Printf("  migration   done %s from %s\n", m.Time.Local().Format(time.DateTime), m.Source)
	default:
		fmt.Printf("  migration   %s: %s\n", m.State, m.Error)
		fmt.Println("              cestafet stays inactive until this is resolved; see 'cestafet migrate'")
	}
	for _, r := range migrate.Inspect(copies) {
		if !fileExists(r.Copy.DB) && !fileExists(r.Copy.Projects) {
			continue
		}
		fmt.Printf("  old copy    %-24s %s\n", r.Copy.Name, copyState(r, m, has))
	}
	for _, o := range migrate.Orphans(copies) {
		fmt.Printf("  ! journal without its database: %s\n", tilde(o))
	}
	return err == nil && (fileExists(t.DB) && !has || has && m.State == migrate.StateDone)
}

func copyState(r migrate.Ranked, m migrate.Marker, has bool) string {
	switch {
	case r.Corrupt != "":
		return "unreadable: " + firstLine(r.Corrupt)
	case r.Empty:
		return "empty"
	}
	last := r.Activity.Local().Format(time.DateTime)
	if has && m.State == migrate.StateDone && r.Activity.After(m.Time) {
		return "ACTIVE, last write " + last + " after the migration: an old cestafet still writes here; " +
			"quit the Claude app (tray → Quit) and every session"
	}
	if has && m.Source == r.Copy.Name {
		return "migrated (kept as is), last activity " + last
	}
	return "stale, last activity " + last
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// runMigrate lists the old copies, or migrates from one of them explicitly.
func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	from := fs.String("from", "", "old copy to migrate from: a name shown by 'cestafet migrate', or a folder holding bus.db")
	replace := fs.Bool("replace", false, "move the current data aside (into a replaced-<time> folder) and migrate anyway")
	if err := fs.Parse(args); err != nil {
		return err
	}
	t, copies, ok := windowsData()
	if !ok {
		fmt.Println("Nothing to migrate: this applies to Windows with the default data location only.")
		return nil
	}
	if *from == "" {
		reportData()
		fmt.Println("\nTo choose a copy: cestafet migrate --from <name> [--replace]")
		return nil
	}
	src, err := pickCopy(*from, copies)
	if err != nil {
		return err
	}
	if *replace {
		if err := refuseLive(t.DB); err != nil {
			return err
		}
	}
	m, err := migrate.Replace(t, src, *replace, core.MigrateOptions())
	if err != nil {
		return err
	}
	fmt.Printf("Migrated from %s into %s.\n", m.Source, tilde(t.Dir))
	if m.Backup != "" {
		fmt.Printf("The previous data is in %s.\n", tilde(m.Backup))
	}
	return nil
}

func pickCopy(from string, copies []migrate.Copy) (migrate.Copy, error) {
	var names []string
	for _, c := range copies {
		if strings.EqualFold(c.Name, from) {
			return c, nil
		}
		names = append(names, c.Name)
	}
	if fi, err := os.Stat(from); err == nil && fi.IsDir() {
		return migrate.Copy{Name: from, DB: filepath.Join(from, "bus.db"), Projects: filepath.Join(from, "projects.toml")}, nil
	}
	return migrate.Copy{}, fmt.Errorf("unknown copy %q; known: %s, or a folder holding bus.db", from, strings.Join(names, ", "))
}

// refuseLive stops --replace while sessions still use the current data.
func refuseLive(db string) error {
	if !fileExists(db) {
		return nil
	}
	st, err := store.Open(db)
	if err != nil {
		return err
	}
	defer st.Close()
	list, err := st.Active()
	if err != nil {
		return err
	}
	var recent []string
	for _, s := range list {
		if time.Since(s.LastSeen) < store.IdleAfter {
			recent = append(recent, core.Label(s))
		}
	}
	if len(recent) > 0 {
		return fmt.Errorf("sessions are using the current data (%s); quit them and the Claude app first",
			strings.Join(recent, "; "))
	}
	return nil
}
