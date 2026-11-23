package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/orkono/cestafet/internal/paths"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/projects"
	"github.com/orkono/cestafet/internal/setup"
	"github.com/orkono/cestafet/internal/store"
)

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func discover() ([]setup.Profile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate home directory: %w", err)
	}
	return setup.Discover(home, filepath.SplitList(os.Getenv("PATH")), os.Getenv("CLAUDE_CONFIG_DIR")), nil
}

// choose resolves --profile flags or asks the developer which profiles to use.
func choose(found []setup.Profile, explicit []string, yes bool, verb string) ([]setup.Profile, error) {
	if len(explicit) > 0 {
		var out []setup.Profile
		for _, dir := range explicit {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("locate home directory: %w", err)
			}
			if dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
				dir = home + dir[1:] // only a leading ~: 8.3 names such as OSMANH~1 keep theirs
			}
			abs, err := filepath.Abs(os.ExpandEnv(dir))
			if err != nil {
				return nil, err
			}
			p := setup.Profile{Dir: abs, Default: paths.Same(abs, filepath.Join(home, ".claude"))}
			out = append(out, p)
		}
		return out, nil
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no Claude Code profile found (looked for ~/.claude, ~/.claude-*, CLAUDE_CONFIG_DIR in rc files and PATH scripts)")
	}
	fmt.Printf("Found %d Claude Code profile(s):\n", len(found))
	for i, p := range found {
		extra := ""
		if p.Default {
			extra = " (default)"
		}
		if len(p.Wrappers) > 0 {
			extra += " via " + strings.Join(p.Wrappers, ", ")
		}
		fmt.Printf("  %d) %s%s\n", i+1, tilde(p.Dir), extra)
	}
	if yes {
		return found, nil
	}
	fmt.Printf("%s the cestafet plugin for which profiles? [all / numbers like 1,3 / none]: ", verb)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	switch line {
	case "", "a", "all", "y", "yes":
		return found, nil
	case "n", "no", "none":
		return nil, nil
	}
	var out []setup.Profile
	for _, part := range strings.Split(line, ",") {
		i, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || i < 1 || i > len(found) {
			return nil, fmt.Errorf("invalid choice %q", part)
		}
		out = append(out, found[i-1])
	}
	return out, nil
}

func tilde(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || p == "" || !paths.Within(p, home) {
		return p
	}
	// Slice the canonical spellings: the raw ones may differ in length (an 8.3
	// short name, a Git Bash /c/ prefix, a trailing separator).
	cp, ch := paths.Canon(p), paths.Canon(home)
	if len(cp) < len(ch) {
		return p
	}
	return "~" + cp[len(ch):]
}

func runSetup(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "install into every discovered profile without asking")
	market := fs.String("marketplace", setup.DefaultMarketplace, "marketplace source: GitHub owner/repo, URL or local path")
	var profiles multiFlag
	fs.Var(&profiles, "profile", "profile directory to install into (repeatable); skips discovery")
	if err := fs.Parse(args); err != nil {
		return err
	}
	found, err := discover()
	if err != nil {
		return err
	}
	chosen, err := choose(found, profiles, *yes, "Install")
	if err != nil || len(chosen) == 0 {
		if err == nil {
			fmt.Println("Nothing installed.")
		}
		return err
	}
	failed := 0
	fmt.Println("Using claude:", claudeLine())
	for _, p := range chosen {
		if err := setup.Install(ctx, setup.ClaudeRunner, p, *market); err != nil {
			fmt.Println("  ✗", err)
			failed++
			continue
		}
		fmt.Println("  ✓", p.Name())
	}
	fmt.Println("\nRestart open sessions of these profiles to load the plugin.")
	fmt.Println("Allow its tools once per profile when Claude asks, or add \"mcp__plugin_cestafet_cestafet__*\" to permissions.allow.")
	if failed > 0 {
		return fmt.Errorf("%d profile(s) failed", failed)
	}
	return nil
}

func runUninstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "uninstall from every discovered profile without asking")
	var profiles multiFlag
	fs.Var(&profiles, "profile", "profile directory (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	found, err := discover()
	if err != nil {
		return err
	}
	chosen, err := choose(found, profiles, *yes, "Remove")
	if err != nil {
		return err
	}
	fmt.Println("Using claude:", claudeLine())
	for _, p := range chosen {
		if err := setup.Uninstall(ctx, setup.ClaudeRunner, p); err != nil {
			fmt.Println("  ✗", err)
			continue
		}
		fmt.Println("  ✓ removed from", p.Name())
	}
	db, _ := store.DefaultPath()
	pp, _ := projects.DefaultPath()
	fmt.Printf("\nData kept: %s and %s (delete them to remove everything).\n", tilde(db), tilde(pp))
	return nil
}

func runDoctor(ctx context.Context) error {
	found, err := discover()
	if err != nil {
		return err
	}
	self, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	onPath, _ := exec.LookPath("cestafet")
	if onPath != "" { // EvalSymlinks("") is "." on Windows
		if resolved, err := filepath.EvalSymlinks(onPath); err == nil {
			onPath = resolved
		}
	}
	fmt.Printf("Binary:\n  version     %s\n  this        %s\n  on PATH     %s\n  claude      %s\n\nProfiles:\n",
		appVersion(), tilde(self), orNone(tilde(onPath)), claudeLine())
	plugins := map[string]setup.PluginInfo{}
	for _, p := range found {
		state := "not installed"
		info, err := setup.PluginState(ctx, setup.ClaudeRunner, p)
		switch {
		case err != nil:
			state = "unknown (" + firstLine(err.Error()) + ")"
		case info.Installed && info.Enabled:
			state = "installed " + info.Version
		case info.Installed:
			state = "installed " + info.Version + ", disabled"
		}
		plugins[tilde(p.Dir)] = info
		fmt.Printf("  %-28s %s\n", tilde(p.Dir), state)
	}
	if warnings := setup.VersionWarnings(appVersion(), self, onPath, plugins); len(warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range warnings {
			fmt.Println("  ! " + w)
		}
	}
	if !reportData() {
		fmt.Println("\nThis directory: skipped, so doctor does not start the data migration.")
		return nil
	}
	app, err := core.Open()
	if err != nil {
		return err
	}
	defer app.Close()
	cwd, _ := os.Getwd()
	loc, project, err := app.Locate(cwd)
	if err != nil {
		return err
	}
	db, _ := store.DefaultPath()
	key, _ := loc.Member()
	fmt.Printf("\nThis directory:\n  repository  %s\n  project     %s\n  database    %s\n  projects    %s\n",
		key, project, tilde(db), tilde(app.ProjectsPath))
	return nil
}

// claudeLine says which claude CLI setup, uninstall and doctor use.
func claudeLine() string {
	exe := setup.Claude()
	if exe.Path == "" {
		return "(not found: not on PATH, no Claude desktop app Claude Code)"
	}
	return tilde(exe.Path) + " (" + exe.Source + ")"
}

func orNone(s string) string {
	if s == "" {
		return "(not found)"
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func projectArg(app *core.App, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	_, project, err := app.Locate(cwd)
	return project, err
}

func runSessions(args []string) error {
	app, err := core.Open()
	if err != nil {
		return err
	}
	defer app.Close()
	project, err := projectArg(app, args)
	if err != nil {
		return err
	}
	list, err := app.Store.List(project)
	if err != nil {
		return err
	}
	fmt.Printf("Project %s: %d session(s)\n", project, len(list))
	for _, s := range list {
		fmt.Printf("  %-11s %-5s %-40s %s  %s\n", s.Role, s.Status, core.Label(s), ago(s.LastSeen), s.Summary)
	}
	return nil
}

func runNotices(args []string) error {
	app, err := core.Open()
	if err != nil {
		return err
	}
	defer app.Close()
	project, err := projectArg(app, args)
	if err != nil {
		return err
	}
	list, err := app.Store.ProjectNotices(project, 20)
	if err != nil {
		return err
	}
	fmt.Printf("Project %s: %d recent notice(s)\n", project, len(list))
	for _, n := range list {
		fmt.Printf("  #%d %s  %s (%s, origin=%s)\n     %s\n", n.ID, ago(n.CreatedAt), n.FromProfile, n.FromRole,
			n.Origin, strings.ReplaceAll(n.Body, "\n", "\n     "))
	}
	return nil
}

// runSend sends a notice from the terminal (origin=agent, sender "terminal").
func runSend(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: cestafet send <project> <text | - for stdin>")
	}
	body := args[1]
	if body == "-" {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, store.MaxBodyBytes+1))
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		body = string(data)
	}
	app, err := core.Open()
	if err != nil {
		return err
	}
	defer app.Close()
	from := store.Session{ID: fmt.Sprintf("terminal-%d", os.Getpid()), Profile: "terminal",
		Project: args[0], Role: store.RoleMember}
	id, to, err := app.Store.Send(store.NewNotice{From: from, Origin: store.OriginAgent, Body: body})
	if err != nil {
		return err
	}
	fmt.Printf("notice #%d sent to %d session(s)\n", id, len(to))
	return nil
}

func ago(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
