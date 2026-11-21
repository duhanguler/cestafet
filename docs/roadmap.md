# Roadmap

Open work, roughly in priority order. Done items move to the release notes.

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

## Later

- Homebrew tap (`brew install orkono/tap/cestafet`): needs an `orkono/homebrew-tap` repo and a token.
- Codex adapter (its own hook/MCP surface) — deferred.
- Push delivery to idle sessions (MCP channel notifications).
- Notice expiry ("until tests finish").
- Optional guard hook that warns before a command matching an active notice (e.g. a staging deploy).
