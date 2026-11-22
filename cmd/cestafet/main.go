// Command cestafet lets AI coding sessions on one machine — across Claude Code
// profiles — see each other per project and pass notices between them.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const usage = `cestafet — notices between AI coding sessions on one machine

Usage:
  cestafet setup [--yes] [--marketplace <source>] [--profile <dir>]...
                           find Claude Code profiles and install the plugin into the chosen ones
  cestafet uninstall [--yes] [--profile <dir>]...
                           remove the plugin from profiles
  cestafet doctor          show profiles, plugin state and this directory's identity
  cestafet sessions [<project>]
                           list live sessions (default: this directory's project)
  cestafet notices [<project>]
                           list recent notices of a project
  cestafet send <project> <text | ->
                           send a notice from the terminal (origin=agent)
  cestafet mcp             run the MCP server (started by the plugin)
  cestafet hook <event>    run a hook handler (started by the plugin):
                           session-start | prompt | pre-tool | post-tool | session-end
  cestafet version
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cestafet:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "hook":
		return runHook(rest)
	case "mcp":
		return runMCP(ctx)
	case "setup":
		return runSetup(ctx, rest)
	case "uninstall":
		return runUninstall(ctx, rest)
	case "doctor":
		return runDoctor(ctx)
	case "sessions":
		return runSessions(rest)
	case "notices":
		return runNotices(rest)
	case "send":
		return runSend(rest)
	case "version", "--version", "-v":
		fmt.Println("cestafet", appVersion())
		return nil
	case "help", "--help", "-h":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
	}
}
