package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/orkono/cestafet/internal/identity"
	"github.com/orkono/cestafet/internal/projects"
	"github.com/orkono/cestafet/internal/store"
)

// diagRecord is what `cestafet diag` observes about the process it runs in.
// It holds process names, ids and paths, never environment values other than
// the few listed below.
type diagRecord struct {
	At        time.Time         `json:"at"`
	Role      string            `json:"role"`
	Version   string            `json:"version"`
	OS        string            `json:"os"`
	Payload   map[string]string `json:"payload,omitempty"`
	Env       map[string]string `json:"env"`
	Getwd     string            `json:"getwd"`
	GitRoot   string            `json:"git_root,omitempty"`
	Chain     []identity.Proc   `json:"chain"`
	ClaudePID int               `json:"claude_pid"`
	DBPath    string            `json:"db_path"`
	Projects  string            `json:"projects_path"`
	CacheDir  string            `json:"cache_dir"`
}

var diagEnv = []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_CONFIG_DIR", "CLAUDE_PROJECT_DIR",
	"CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_GIT_BASH_PATH", "SHELL", "ComSpec", "MSYSTEM", "HOME", "USERPROFILE",
	"XDG_DATA_HOME", "XDG_CONFIG_HOME", "CESTAFET_DB"}

// runDiag records one observation. Roles: "cli" (default) prints it; "hook"
// reads the hook payload from stdin; "mcp" records and then serves MCP as usual,
// so a diagnostic setup keeps working tools.
func runDiag(ctx context.Context, args []string) error {
	role := "cli"
	if len(args) > 0 {
		role = args[0]
	}
	if role != "cli" && role != "hook" && role != "mcp" {
		return fmt.Errorf("usage: cestafet diag [cli|hook|mcp]")
	}
	rec := observe(role)
	if role == "hook" {
		rec.Payload = readPayload(os.Stdin)
		rec.GitRoot = gitRoot(rec.Payload["cwd"])
	} else {
		rec.GitRoot = gitRoot(rec.Getwd)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	path := diagLogPath()
	if err := appendLine(path, line); err != nil && role == "cli" {
		fmt.Fprintln(os.Stderr, "cestafet diag: write log:", err)
	}
	switch role {
	case "cli":
		var pretty []byte
		pretty, _ = json.MarshalIndent(rec, "", "  ")
		fmt.Printf("%s\n(appended to %s)\n", pretty, path)
	case "mcp":
		return runMCP(ctx)
	}
	return nil
}

func observe(role string) diagRecord {
	rec := diagRecord{At: time.Now().UTC(), Role: role, Version: appVersion(),
		OS: runtime.GOOS + "/" + runtime.GOARCH, Env: map[string]string{},
		Chain: identity.Chain(os.Getpid(), 12), ClaudePID: identity.ClaudePID()}
	for _, k := range diagEnv {
		if v, ok := os.LookupEnv(k); ok {
			rec.Env[k] = v
		}
	}
	rec.Getwd, _ = os.Getwd()
	rec.DBPath, _ = store.DefaultPath()
	rec.Projects, _ = projects.DefaultPath()
	rec.CacheDir, _ = os.UserCacheDir()
	return rec
}

func readPayload(r io.Reader) map[string]string {
	var raw map[string]any
	if err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&raw); err != nil {
		return map[string]string{"decode_error": err.Error()}
	}
	out := map[string]string{}
	for _, k := range []string{"hook_event_name", "session_id", "cwd", "source", "reason", "tool_name", "agent_id"} {
		if v, ok := raw[k].(string); ok {
			out[k] = v
		}
	}
	if ti, ok := raw["tool_input"].(map[string]any); ok {
		if c, ok := ti["command"].(string); ok {
			out["tool_input.command"] = c
		}
	}
	return out
}

func gitRoot(dir string) string {
	if dir == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func diagLogPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "cestafet", "diag.jsonl")
}

func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}
