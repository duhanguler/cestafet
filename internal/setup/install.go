package setup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Plugin and marketplace identifiers.
const (
	MarketplaceName    = "cestafet"
	PluginName         = "cestafet"
	DefaultMarketplace = "orkono/cestafet" // GitHub repo that holds .claude-plugin/marketplace.json
)

// Runner runs the claude CLI for one profile; replaced in tests.
type Runner func(ctx context.Context, p Profile, args ...string) (string, error)

// ClaudeRunner runs `claude <args>` with CLAUDE_CONFIG_DIR set to the profile
// (unset for the default profile), stdin closed.
func ClaudeRunner(ctx context.Context, p Profile, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	exe := Claude()
	if exe.Path == "" {
		return "", fmt.Errorf("claude not found: not on PATH, and no Claude Code from the Claude desktop app")
	}
	cmd := exec.CommandContext(ctx, exe.Path, args...)
	cmd.Env = profileEnv(os.Environ(), p)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func profileEnv(env []string, p Profile) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		// Windows environment names are case-insensitive: drop every spelling.
		if !strings.HasPrefix(strings.ToUpper(kv), "CLAUDE_CONFIG_DIR=") {
			out = append(out, kv)
		}
	}
	if !p.Default {
		out = append(out, "CLAUDE_CONFIG_DIR="+p.Dir)
	}
	return out
}

// Install adds the marketplace and installs the plugin into a profile; an
// already-installed plugin is updated to the marketplace's version.
func Install(ctx context.Context, run Runner, p Profile, marketplace string) error {
	if out, err := run(ctx, p, "plugin", "marketplace", "add", marketplace); err != nil && !alreadyThere(out) {
		return fmt.Errorf("%s: add marketplace: %v: %s", p.Name(), err, out)
	}
	if out, err := run(ctx, p, "plugin", "marketplace", "update", MarketplaceName); err != nil {
		return fmt.Errorf("%s: update marketplace: %v: %s", p.Name(), err, out)
	}
	id := PluginName + "@" + MarketplaceName
	// An already-installed plugin is reported with or without a non-zero exit
	// (claude 2.1.330 exits 0); either way it needs an explicit update.
	out, err := run(ctx, p, "plugin", "install", id)
	if !alreadyThere(out) {
		if err != nil {
			return fmt.Errorf("%s: install plugin: %v: %s", p.Name(), err, out)
		}
		return nil
	}
	if out, err := run(ctx, p, "plugin", "update", id); err != nil {
		return fmt.Errorf("%s: update plugin: %v: %s", p.Name(), err, out)
	}
	return nil
}

// Uninstall removes the plugin and the marketplace from a profile.
func Uninstall(ctx context.Context, run Runner, p Profile) error {
	if out, err := run(ctx, p, "plugin", "uninstall", PluginName+"@"+MarketplaceName); err != nil && !notThere(out) {
		return fmt.Errorf("%s: uninstall plugin: %v: %s", p.Name(), err, out)
	}
	if out, err := run(ctx, p, "plugin", "marketplace", "remove", MarketplaceName); err != nil && !notThere(out) {
		return fmt.Errorf("%s: remove marketplace: %v: %s", p.Name(), err, out)
	}
	return nil
}

func alreadyThere(out string) bool {
	o := strings.ToLower(out)
	return strings.Contains(o, "already")
}

func notThere(out string) bool {
	o := strings.ToLower(out)
	return strings.Contains(o, "not found") || strings.Contains(o, "not installed") || strings.Contains(o, "no marketplace")
}
