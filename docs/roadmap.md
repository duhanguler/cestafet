# Roadmap

Open work, roughly in priority order. Done items move to the release notes.

## Next

### Sessions in non-git directories meet automatically

**Problem (seen 2026-11-21):** a directory that is not a git repository falls back to its own
path as the project. Two sessions opened in `~/work/app` and `~/work/app/docs` (neither a git
repo) end up in two different projects and cannot see each other until the user runs
`/cestafet:project <name>`.

**Proposal:** when the cwd is not inside a git repository and no explicit mapping matches,
resolve to the nearest ancestor directory that is already a project key of a live session (or a
mapped path), before falling back to the cwd itself. A session opened in `~/work/app/docs` then
joins the project of a session already running in `~/work/app`.

**Open points:**
- Order of start: if the `docs` session starts first and `app` later, the parent should join the
  child's project? Probably not; instead, on registration, offer (via the orientation text) to run
  `/cestafet:project <name>` when a live session exists in a parent or child non-git directory.
- Stop the upward walk at `$HOME` and never treat `$HOME` or `/` as a project.

**Done when:** the two-session case above works without any command, the reverse start order
gets a clear suggestion, and tests cover parent/child, siblings (must not merge) and `$HOME`.

## Spike items still open (see docs/spikes.md)

- 4 KB / 8 KB notice payloads delivered intact (B2).
- Interactive-mode parity for hook context injection (B1/B2).
- `/clear` and `--resume`: session id churn while the MCP server keeps running (B3). Known gap:
  until the next hook call registers the new id, MCP tools can report "not registered".
- Subagent tool calls: which `session_id` PostToolUse reports.
- Claude Code's built-in ListAgents / SendMessage across profiles (B7).

## Later

- First release: tag `v0.1.0`, goreleaser binaries, Homebrew tap.
- Codex adapter (its own hook/MCP surface).
- Push delivery to idle sessions (MCP channel notifications).
- Notice expiry ("until tests finish").
- Optional guard hook that warns before a command matching an active notice (e.g. a staging deploy).
