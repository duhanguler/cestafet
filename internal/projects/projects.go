// Package projects maps repositories to cestafet projects. A project can span
// several repositories; the mapping lives in one central TOML file, never
// inside the repositories themselves.
package projects

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

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

// DefaultPath returns $XDG_CONFIG_HOME/cestafet/projects.toml, falling back to
// ~/.config/cestafet/projects.toml.
func DefaultPath() (string, error) {
	if p := os.Getenv("CESTAFET_PROJECTS"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "cestafet", "projects.toml"), nil
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
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
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
	prefix = strings.TrimRight(prefix, "/")
	return path == prefix || strings.HasPrefix(path, prefix+"/")
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

func (c Config) without(l Location) Config {
	key, _ := l.Member()
	out := Config{Projects: map[string]Project{}}
	for name, p := range c.Projects {
		np := Project{
			Remotes: slices.DeleteFunc(slices.Clone(p.Remotes), func(r string) bool { return r == key }),
			Paths:   slices.DeleteFunc(slices.Clone(p.Paths), func(r string) bool { return r == key }),
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
