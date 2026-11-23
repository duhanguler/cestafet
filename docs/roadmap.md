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

## Done for v0.1.1 – v0.1.3

- A notice backlog is delivered over successive hooks, at most 12 KB per hook.
- `cestafet doctor` reports binary and plugin versions and the `cestafet` on PATH; `setup`
  updates an installed plugin; `go install` builds report their version.
- Homebrew cask (`brew install --cask orkono/tap/cestafet`), clearing macOS quarantine.
- README demo GIF; announcement drafts in [announcement.md](announcement.md).

## Done for v0.2.0

- Standing rules: a notice that stays in force until lifted or expired ("no deploys while
  testing"), re-shown to new and compacted sessions, lifted with plain language.
- Guard patterns on rules: a `PreToolUse` hook asks before a Bash command a rule forbids.

## Next

- Push delivery to idle sessions through Claude Code channels (opt-in; spike first, see
  [v0.2-design.md](v0.2-design.md) §2 for the allowlist constraint).

## Done for v0.3.0

- Windows support (amd64, arm64): identity through Git Bash hooks, path keys, data directories,
  setup discovery (PowerShell, cmd, user environment), guard for the PowerShell tool. Plan and
  release-gate run: [windows-plan.md](windows-plan.md).
- The database URI is percent-encoded (a `#` or `%` in the path opened another database).
- A session no longer receives its own notices back after `/clear`.

## Later

- Codex adapter (its own hook/MCP surface) — deferred.

## Not planned

- Apple Developer ID signing and notarization: no developer account. Release binaries stay
  unsigned; the Homebrew cask clears the quarantine flag, `go install` builds are unaffected.
