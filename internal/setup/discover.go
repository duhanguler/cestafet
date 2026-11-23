// Package setup finds the Claude Code profiles on this machine and installs
// the cestafet plugin into the ones the developer confirms.
package setup

import (
	"bufio"
	"github.com/orkono/cestafet/internal/paths"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Profile is one Claude Code configuration directory.
type Profile struct {
	Dir      string   // absolute config dir, e.g. ~/.claude-orkono
	Default  bool     // ~/.claude, used when CLAUDE_CONFIG_DIR is unset
	Wrappers []string // scripts or rc files that set CLAUDE_CONFIG_DIR to Dir
}

// Name is the profile's display name.
func (p Profile) Name() string { return strings.TrimPrefix(filepath.Base(p.Dir), ".") }

// Discover finds profiles in home: ~/.claude, ~/.claude-*, every
// CLAUDE_CONFIG_DIR assignment in shell rc files and in scripts on path, and
// the current CLAUDE_CONFIG_DIR. Only directories that look like a profile
// (settings.json or projects/ inside) are returned.
func Discover(home string, pathDirs []string, currentConfigDir string) []Profile {
	found := map[string]*Profile{}
	add := func(dir, wrapper string) {
		dir = expand(dir, home)
		if dir == "" || !looksLikeProfile(dir) {
			return
		}
		p, ok := found[dir]
		if !ok {
			p = &Profile{Dir: dir, Default: paths.Same(dir, filepath.Join(home, ".claude"))}
			found[dir] = p
		}
		if wrapper != "" && !contains(p.Wrappers, wrapper) {
			p.Wrappers = append(p.Wrappers, wrapper)
		}
	}

	add(filepath.Join(home, ".claude"), "")
	if matches, err := filepath.Glob(filepath.Join(home, ".claude-*")); err == nil {
		for _, m := range matches {
			add(m, "")
		}
	}
	for _, rc := range shellProfiles(home) {
		for _, dir := range scanFile(filepath.Join(home, rc)) {
			add(dir, "~/"+filepath.ToSlash(rc))
		}
	}
	for _, dir := range userEnvConfigDirs() {
		add(dir, "user environment")
	}
	for _, pd := range pathDirs {
		entries, err := os.ReadDir(pd)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !scanCandidate(e.Name()) {
				continue
			}
			for _, dir := range scanFile(filepath.Join(pd, e.Name())) {
				add(dir, e.Name())
			}
		}
	}
	if currentConfigDir != "" {
		add(currentConfigDir, "")
	}

	out := make([]Profile, 0, len(found))
	for _, p := range found {
		sort.Strings(p.Wrappers)
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Default != out[j].Default {
			return out[i].Default
		}
		return out[i].Dir < out[j].Dir
	})
	return out
}

func looksLikeProfile(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "settings.json")); err == nil && !info.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(dir, "projects"))
	return err == nil && info.IsDir()
}

// maxScanBytes skips binaries and large files when scanning PATH.
const maxScanBytes = 64 * 1024

func scanFile(path string) []string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > maxScanBytes {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var dirs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.ContainsRune(line, 0) {
			return nil // binary
		}
		dirs = append(dirs, extract(line)...)
	}
	return dirs
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
