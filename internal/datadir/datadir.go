// Package datadir decides where cestafet keeps its files on Windows and finds
// the copies older versions left behind.
//
// Up to 0.3.0 Windows used %LOCALAPPDATA%\cestafet and %APPDATA%\cestafet. When
// Claude Code runs inside the Claude desktop app (an MSIX package), Windows
// redirects new files there into the package's private LocalCache, so sessions
// in the app and processes outside it saw different databases. From 0.3.1 the
// default is %USERPROFILE%\.cestafet, which is not virtualized.
package datadir

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/orkono/cestafet/internal/migrate"
)

// Name of the Windows data directory under the user profile.
const Name = ".cestafet"

// WindowsDir is the 0.3.1 default data directory for a user profile.
func WindowsDir(profile string) string { return filepath.Join(profile, Name) }

// Target is the migration target inside a data directory.
func Target(dir string) migrate.Target {
	return migrate.Target{Dir: dir, DB: filepath.Join(dir, "bus.db"), Projects: filepath.Join(dir, "projects.toml")}
}

// LegacyCopies lists the places 0.3.0 may have written to, built from the user
// profile path only (never from %LOCALAPPDATA%, whose view depends on whether
// the caller runs inside a package): the real AppData pair first, then each
// Claude package's LocalCache pair in lexical order.
func LegacyCopies(profile string) []migrate.Copy {
	local := filepath.Join(profile, "AppData", "Local")
	out := []migrate.Copy{{
		Name:     "AppData",
		DB:       filepath.Join(local, "cestafet", "bus.db"),
		Projects: filepath.Join(profile, "AppData", "Roaming", "cestafet", "projects.toml"),
		// Read from inside the desktop app's package, this path shows the
		// package's copy; that copy's own entry then names it.
		Redirectable: true,
	}}
	pkgs, _ := filepath.Glob(filepath.Join(local, "Packages", "Claude_*"))
	sort.Strings(pkgs)
	for _, p := range pkgs {
		cache := filepath.Join(p, "LocalCache")
		out = append(out, migrate.Copy{
			Name:     filepath.Base(p),
			DB:       filepath.Join(cache, "Local", "cestafet", "bus.db"),
			Projects: filepath.Join(cache, "Roaming", "cestafet", "projects.toml"),
		})
	}
	return out
}

// BuiltIn reports whether neither the database nor the projects file location
// is overridden by an environment variable, i.e. migration may run.
func BuiltIn() bool {
	for _, v := range []string{"CESTAFET_DB", "CESTAFET_PROJECTS", "XDG_DATA_HOME", "XDG_CONFIG_HOME"} {
		if os.Getenv(v) != "" {
			return false
		}
	}
	return true
}

// LogDir is where cestafet writes its hook error and diagnostic logs: the data
// directory on Windows (the user cache directory is virtualized there too),
// the user cache directory elsewhere.
func LogDir() (string, error) {
	if runtime.GOOS == "windows" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return WindowsDir(home), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cestafet"), nil
}
