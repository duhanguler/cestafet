package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/orkono/cestafet/internal/paths"
	"sort"
)

// PluginInfo is the state of the cestafet plugin in one profile.
type PluginInfo struct {
	Installed bool
	Enabled   bool
	Version   string
}

// PluginState reads the plugin's state from `claude plugin list --json`.
func PluginState(ctx context.Context, run Runner, p Profile) (PluginInfo, error) {
	out, err := run(ctx, p, "plugin", "list", "--json")
	if err != nil {
		return PluginInfo{}, fmt.Errorf("%s: list plugins: %v: %s", p.Name(), err, out)
	}
	var list []struct {
		ID      string `json:"id"`
		Version string `json:"version"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return PluginInfo{}, fmt.Errorf("%s: parse plugin list: %w", p.Name(), err)
	}
	for _, e := range list {
		if e.ID == PluginName+"@"+MarketplaceName {
			return PluginInfo{Installed: true, Enabled: e.Enabled == nil || *e.Enabled, Version: e.Version}, nil
		}
	}
	return PluginInfo{}, nil
}

// VersionWarnings compares the running binary with the plugin installed in
// each profile and with the cestafet the hooks resolve on PATH. A "dev" build
// skips the version comparison.
func VersionWarnings(binVersion, self, onPath string, plugins map[string]PluginInfo) []string {
	var w []string
	if onPath == "" {
		w = append(w, "cestafet is not on PATH: hooks and the MCP server cannot start")
	} else if !paths.Same(self, onPath) {
		w = append(w, fmt.Sprintf("hooks run %s, not this binary (%s): check PATH order", onPath, self))
	}
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := plugins[name]
		switch {
		case !p.Installed:
		case !p.Enabled:
			w = append(w, fmt.Sprintf("%s: plugin is installed but disabled", name))
		case binVersion != "dev" && p.Version != binVersion:
			w = append(w, fmt.Sprintf("%s: plugin %s, binary %s: run `cestafet setup` to update", name, p.Version, binVersion))
		}
	}
	return w
}
