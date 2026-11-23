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

## Phase 0 results (2026-11-23)

Windows 11 ARM64 (build 22000) in UTM, Claude Code 2.1.330 native install, Git for Windows 2.56.1,
`cestafet` windows/arm64 diagnostic build; sessions started from cmd and from Git Bash, `-p` and
interactive, with `/clear`.

| Question | Windows | macOS |
|---|---|---|
| MCP server's parent | `claude.exe` (depth 1) | `claude` (depth 1) |
| Hook's ancestors | `bash.exe` → `bash.exe` → `claude.exe` (depth 3): hooks run through Git Bash | `claude` (depth 1) |
| `claude.exe` pid across `/clear` | unchanged (1332) | unchanged |
| Payload `cwd` (also when started from Git Bash) | `C:\Users\u\ce-test` | `/Users/u/...` |
| `git rev-parse --show-toplevel` | `C:/Users/u/ce-test` (forward slashes) | same as cwd |
| Hook env `CLAUDE_CODE_SESSION_ID` | equals payload `session_id`, follows `/clear` | same |
| MCP env `CLAUDE_CODE_SESSION_ID` | launch id | launch id (stale after `/clear`) |
| Bash tool name in `PreToolUse` | `Bash` | `Bash` |
| Current `ClaudePID()` | 0 (no `ps`) | correct |

Conclusion for phase 1: the existing design holds on Windows. Hooks and the MCP server find the
same `claude.exe` by walking ancestors, and its pid survives `/clear`, so the pid-based
`supersede` chain works unchanged once the walk is cross-platform. No launch-id fallback is
needed. The PowerShell tool was not exercised (Claude Code chose `Bash`); the guard change stays
in phase 3 with its own check.

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

## Status (2026-11-23)

Phases 0–3 are merged (PRs #24–#30) and the release gate is passed.

On Windows 11 ARM64 (UTM VM), Claude Code 2.1.330, the plugin from `main` loaded with
`--plugin-dir`, two interactive sessions:

- every package's tests pass; tool hooks take 14–18 ms, prompt and session start ~110 ms;
- `setup` finds a profile set in the PowerShell profile under a path with a space and installs the
  plugin from the GitHub marketplace; `doctor` is correct;
- `/cestafet:coordinate`, then "do not deploy to staging until I say so, guard ./deploy.sh staging
  and .\deploy.ps1 staging" became a guarded standing rule; the other session received it at its
  next prompt and acknowledged it; "has everyone seen it?" showed the reply;
- after `/clear` on both sessions the member still listed the rule and the coordinator role moved
  to the new session id;
- the guarded `.\deploy.ps1 staging`, run through the **PowerShell** tool, made Claude Code ask
  with the rule as the reason; answering No left it unrun;
- "testing is finished, deploys are allowed again" lifted the rule and the member was told.

Found and fixed during the run (#30): after `/clear` a session was backfilled the notices its own
previous id had sent.

## Release gate

No Windows release before a full manual test on a real Windows machine, Claude Code native and Git
Bash (PowerShell tool too): setup, MCP start, `whoami`, two sessions, notify / ack, coordinator
across `/clear`, standing rule and guard.
