// Package projects maps repositories to cestafet projects. A project can span
// several repositories; the mapping lives in one central TOML file, never
// inside the repositories themselves.
package projects

import (
	"bytes"
	"fmt"
	"github.com/orkono/cestafet/internal/paths"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/orkono/cestafet/internal/identity"
)

// Project lists the repositories of one named project.
type Project struct {
	Remotes []string `toml:"remotes"`
	Paths   []string `toml:"paths"`
}

// Config is the content of projects.toml.
type Config struct {
	Projects map[string]Project `toml:"projects"`
}

// DefaultPath returns $CESTAFET_PROJECTS, else
// $XDG_CONFIG_HOME/cestafet/projects.toml, else %USERPROFILE%\.cestafet\projects.toml
// on Windows and ~/.config/cestafet/projects.toml elsewhere.
func DefaultPath() (string, error) {
	if p := os.Getenv("CESTAFET_PROJECTS"); p != "" {
		return p, nil
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "cestafet", "projects.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, ".cestafet", "projects.toml"), nil
	}
	return filepath.Join(home, ".config", "cestafet", "projects.toml"), nil
}

// Normalize parses a projects file and returns it re-encoded with path
// entries in canonical spelling (8.3 short names expanded on Windows).
func Normalize(data []byte) ([]byte, error) {
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, err
	}
	for name, p := range cfg.Projects {
		np := Project{Remotes: slices.Clone(p.Remotes)}
		for _, q := range p.Paths {
			np.Paths = append(np.Paths, paths.Canon(q))
		}
		cfg.Projects[name] = np
	}
	var buf bytes.Buffer
	buf.WriteString("# cestafet projects: which repositories belong to which project.\n")
	buf.WriteString("# Managed by /cestafet:project; safe to edit by hand.\n\n")
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Load reads the config; a missing file is an empty config.
func Load(path string) (Config, error) {
	cfg := Config{Projects: map[string]Project{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]Project{}
	}
	return cfg, nil
}

// Save writes the config atomically.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("# cestafet projects: which repositories belong to which project.\n")
	buf.WriteString("# Managed by /cestafet:project; safe to edit by hand.\n\n")
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	// A temp file of its own per writer, so two sessions saving at once never
	// share one; the rename is retried because Windows refuses to replace a
	// file another process (or a virus scanner) has open for a moment.
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	tmp := f.Name()
	_, werr := f.Write(buf.Bytes())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write config: %w", werr)
	}
	var rerr error
	for attempt := 0; attempt < 10; attempt++ {
		if rerr = os.Rename(tmp, path); rerr == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = os.Remove(tmp)
	return fmt.Errorf("replace config: %w", rerr)
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// ValidName reports whether name is an acceptable project name.
func ValidName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("project name %q must be 1-64 lowercase letters, digits, '.', '_' or '-'", name)
	}
	return nil
}

// Location is where a session runs, as seen by project resolution.
type Location struct {
	Cwd  string
	Repo identity.Repo
}

// Member is the key a location is mapped by: the repo's remote, else its root,
// else the cwd.
func (l Location) Member() (key string, isRemote bool) {
	switch {
	case l.Repo.Remote != "":
		return l.Repo.Remote, true
	case l.Repo.Root != "":
		return l.Repo.Root, false
	default:
		return l.Cwd, false
	}
}

// Resolve returns the project of a location: an explicit mapping by remote,
// then by longest path prefix, then the automatic key (remote, root or cwd).
func (c Config) Resolve(l Location) string {
	if l.Repo.Remote != "" {
		for _, name := range c.names() {
			if slices.Contains(c.Projects[name].Remotes, l.Repo.Remote) {
				return name
			}
		}
	}
	best, bestLen := "", -1
	for _, name := range c.names() {
		for _, p := range c.Projects[name].Paths {
			if under(l.Cwd, p) && len(p) > bestLen {
				best, bestLen = name, len(p)
			}
		}
	}
	if best != "" {
		return best
	}
	key, _ := l.Member()
	return key
}

func under(path, prefix string) bool {
	return paths.Within(path, prefix)
}

func (c Config) names() []string {
	names := make([]string, 0, len(c.Projects))
	for n := range c.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Add maps the location's repository to project name, removing it from any
// other project first. It returns a new Config.
func (c Config) Add(name string, l Location) (Config, error) {
	if err := ValidName(name); err != nil {
		return c, err
	}
	out := c.without(l)
	key, isRemote := l.Member()
	p := out.Projects[name]
	if isRemote {
		p.Remotes = append(slices.Clone(p.Remotes), key)
	} else {
		p.Paths = append(slices.Clone(p.Paths), key)
	}
	out.Projects[name] = p
	return out, nil
}

// Remove unmaps the location's repository. It returns a new Config.
func (c Config) Remove(l Location) Config { return c.without(l) }

// Explicit returns the project that maps the location's member key directly
// (not through a parent path), if any.
func (c Config) Explicit(l Location) (string, bool) {
	key, isRemote := l.Member()
	for _, name := range c.names() {
		p := c.Projects[name]
		if isRemote && slices.Contains(p.Remotes, key) {
			return name, true
		}
		if !isRemote && slices.ContainsFunc(p.Paths, func(q string) bool { return paths.Same(q, key) }) {
			return name, true
		}
	}
	return "", false
}

// SameKey reports whether a caller-supplied key names the location's member
// key: exactly for a remote, as the same filesystem location for a path.
func SameKey(given string, l Location) bool {
	key, isRemote := l.Member()
	if given == "" {
		return false
	}
	if isRemote {
		return given == key
	}
	return paths.Same(given, key)
}

func (c Config) without(l Location) Config {
	key, isRemote := l.Member()
	out := Config{Projects: map[string]Project{}}
	for name, p := range c.Projects {
		np := Project{
			Remotes: slices.DeleteFunc(slices.Clone(p.Remotes), func(r string) bool { return isRemote && r == key }),
			Paths:   slices.DeleteFunc(slices.Clone(p.Paths), func(r string) bool { return !isRemote && paths.Same(r, key) }),
		}
		if len(np.Remotes)+len(np.Paths) > 0 {
			out.Projects[name] = np
		}
	}
	return out
}

// Members lists the repositories mapped to a project.
func (c Config) Members(name string) []string {
	p, ok := c.Projects[name]
	if !ok {
		return nil
	}
	return append(slices.Clone(p.Remotes), p.Paths...)
}
