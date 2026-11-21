# Roadmap

Open work, roughly in priority order. Done items move to the release notes.
Task breakdown: [plan.md](plan.md) · v0.2 design: [v0.2-design.md](v0.2-design.md) · releasing: [release.md](release.md).

## Done for v0.1.0

- Sessions in non-git directories meet automatically: an unmapped directory outside git joins
  the nearest ancestor directory that is already a live session's project (stops below `$HOME`,
  never `/`); a child that started first joins the parent at its next prompt and keeps the
  project if the parent leaves.
- `/clear` keeps the session's identity chain: the new session id retires the old one and
  inherits the coordinator role.
- Notices are no longer consumed by subagent tool calls.
- `cestafet send <project> <text>` for sending a notice from the terminal.

## Open checks

- 8 KB / 16 KB notice payloads (4 KB verified).
- Claude Code's built-in ListAgents / SendMessage across profiles (inconclusive, not needed).

## v0.1.1 (patch)

- Cap the context one hook injects, so several pending 4 KB notices cannot flood a session.
- Homebrew cask (config ready; waits for the `orkono/homebrew-tap` repo and its token).
- `cestafet doctor` reports binary vs. plugin version.
- README demo GIF and the announcement.

## v0.2.0

- Standing rules: a notice that stays in force until lifted or expired ("no deploys while
  testing"), re-shown to new and compacted sessions, lifted with plain language.
- Optional guard: a `PreToolUse` hook asks before a Bash command matching an active rule.
- Push delivery to idle sessions through Claude Code channels (opt-in; see the design for the
  allowlist constraint).

## Later

- Codex adapter (its own hook/MCP surface) — deferred.
