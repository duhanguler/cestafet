# Windows support — plan

Source: a user's code-reading review (nothing compiled or run) reporting that cestafet does not
work on Windows, checked against `f6d2cf2` and accepted after a three-referee round
(2026-11-22, all APPROVE_WITH_CHANGES). Two referees cross-compiled `f6d2cf2` for windows/amd64 and
windows/arm64 with `CGO_ENABLED=0`; runtime on Windows is unverified.

## Why it fails today

1. **Session identity.** The owning `claude` process is found with `ps -o ppid=,comm=`
   (`internal/identity/identity.go:139-153`) and matched by the exact name `claude` (`:131`).
   Windows has no `ps`, so `ClaudePID()` is 0 and every MCP tool fails to find its session;
   `/clear` cannot retire the old session either (`internal/store/sessions.go:94-117`).
2. **Paths.** Containment and root checks use a literal `/` (`internal/core/core.go:77-80`,
   `internal/projects/projects.go:143-146`) and the store filters path projects with
   `LIKE '/%'` (`internal/store/sessions.go:122`), so `C:\...` paths never match.

## Evidence from macOS (Claude Code 2.1.330)

- Hooks' environment has `CLAUDE_CODE_SESSION_ID`, equal to the payload `session_id`, and it
  follows `/clear`. The MCP server's copy stays the launch id after `/clear`.
- The direct parent of both hook and MCP server processes is the `claude` process.
- `CLAUDE_CODE_SESSION_ID` is undocumented: best effort only.

## Phase 0 — Windows evidence first (before any identity design)

A diagnostic command, `cestafet diag`, also run as a hook and as an MCP tool, that records: the
ancestor chain (pid, parent pid, exe name, creation time), `CLAUDE_CODE_SESSION_ID`, the payload
`cwd` and `session_id`, `os.Getwd`, `git rev-parse --show-toplevel`, and the resolved data paths.
Run it on a real Windows machine with Claude Code installed natively, from PowerShell and from
Git Bash, and with an npm install if available, before and after `/clear`. CI adds
`windows-latest` for build and unit tests in this phase.

## Phase 1 — identity (designed from Phase 0 results)

- The hook payload's `session_id` stays authoritative.
- A parent pid is trusted only after verifying the process is Claude Code (name `claude` /
  `claude.exe`, case-insensitive; an npm `node.exe` only with evidence that it hosts Claude Code,
  otherwise document native install only). Walk ancestors when the parent is a shell.
- Store the owner's process **creation time** with its pid; `ByClaudePID` matches both, so a
  reused pid never inherits a session or the coordinator role.
- `/clear`: persist an explicit old → new link in the same transaction as `supersede`. Until that
  exists the MCP server's launch id is a startup-only shortcut, and the server returns an error
  instead of acting as a retired session.
- Windows process listing: `CreateToolhelp32Snapshot`, tolerating `OpenProcess` denials and pid
  cycles. `git`/`ssh` children start with `CREATE_NO_WINDOW`.

## Phase 2 — paths, setup, data

- One canonicalisation function used **before storage and lookup** (including CLI project
  arguments and `App.Home`), a no-op on darwin/linux so existing keys and `projects.toml` paths
  keep matching. Windows: `filepath.Clean`, Git Bash `/c/...` → `C:\...`, case-insensitive
  comparison without rewriting the stored spelling. Containment via `filepath.Rel` guarded against
  cross-volume errors and `..foo` names; root check via `VolumeName`; UNC, `subst` and junction
  cases tested. Replace the SQL `LIKE '/%'` with a Go-side `filepath.IsAbs`.
- `setup`: `os.UserHomeDir` everywhere; expand only a leading `~` (`admin.go:40`; `~1` short
  names must survive); case-insensitive path comparisons (`discover.go:37-39`, `version.go:47`,
  `admin.go:44,87`) and `CLAUDE_CONFIG_DIR` strip (`install.go:39`); PowerShell profiles (incl.
  OneDrive-redirected Documents), `HKCU\Environment`, `%VAR%` / `$env:VAR` (unresolved ones
  rejected), `set "CLAUDE_CONFIG_DIR=..."`, `REM` / `::` comments, quoted paths with spaces; PATH
  scan limited to `.cmd`, `.bat`, `.ps1` and extensionless scripts.
- Data: `%LOCALAPPDATA%\cestafet` and `%APPDATA%\cestafet` when `XDG_*` / `CESTAFET_*` are unset;
  document the cross-shell split if Git Bash sets `XDG_*`. SQLite DSN built with
  `filepath.Abs` + `url.URL` (spaces, `#`, `%`, UNC). `projects.toml`: unique temp file per write,
  serialised read-modify-write, rename retry. Permissions: document that Windows protection comes
  from the profile ACL.

## Phase 3 — guard, release, docs

- Guard: `PreToolUse` matcher `Bash|PowerShell` and `guardReason` accepting both
  (`tool_input.command`, per the Claude Code hook reference).
- Hook latency: measure on Windows with Defender; the 5 s `PostToolUse` timeout may need raising.
- GoReleaser windows/amd64 + arm64 as zip; `.gitattributes` `* text=auto eol=lf`; CI
  `windows-latest` with `shell: bash` for `gofmt`; Unix-path tests ported to `t.TempDir()` and
  Windows tables (`C:\`, `.cmd` wrappers, `claude.exe`, home with spaces and non-ASCII letters).
- Docs: Windows install, data paths, `cestafet.exe` on the PATH Claude Code sees, unsigned binary
  (SmartScreen).

## Release gate

No Windows release before a full manual test on a real Windows machine, Claude Code native and Git
Bash (PowerShell tool too): setup, MCP start, `whoami`, two sessions, notify / ack, coordinator
across `/clear`, standing rule and guard.
