package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/orkono/cestafet/internal/core"
	"github.com/orkono/cestafet/internal/datadir"
	"github.com/orkono/cestafet/internal/hooks"
	"github.com/orkono/cestafet/internal/identity"
	"github.com/orkono/cestafet/internal/mcpserver"
	"github.com/orkono/cestafet/internal/migrate"
)

// runHook handles one hook event. A hook must never break the user's session,
// so failures are logged and the process still exits 0.
func runHook(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: cestafet hook <%s>", "session-start|prompt|pre-tool|post-tool|session-end")
	}
	app, err := core.Open()
	if err != nil {
		logHookError(args[0], err)
		return nil
	}
	defer app.Close()
	if err := hooks.Handle(app, args[0], os.Stdin, os.Stdout, identity.Claude()); err != nil {
		logHookError(args[0], err)
	}
	return nil
}

// logHookError appends to a small log next to the database instead of
// printing to stderr, which Claude Code would surface in the session.
func logHookError(event string, err error) {
	dir, derr := datadir.LogDir()
	if derr != nil {
		return
	}
	path := filepath.Join(dir, "hook-errors.log")
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	f, ferr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if ferr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s: %v\n", time.Now().Format(time.RFC3339), event, err)
}

func runMCP(ctx context.Context) error {
	app, err := openForServer(ctx)
	if err != nil {
		return err
	}
	defer app.Close()
	err = mcpserver.New(app, identity.Claude()).Run(ctx, appVersion())
	if err == io.EOF || ctx.Err() != nil {
		return nil
	}
	return err
}

// openForServer opens the app for the MCP server. A data migration that is
// deferred (another process holds the old database for a moment) is retried
// for a while, since the server lives for the whole session; a blocked one
// needs the user ('cestafet doctor'), so it fails at once.
func openForServer(ctx context.Context) (*core.App, error) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		app, err := core.Open()
		if err == nil || !errors.Is(err, migrate.ErrDeferred) || time.Now().After(deadline) {
			return app, err
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(2 * time.Second):
		}
	}
}
