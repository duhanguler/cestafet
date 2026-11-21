# cestafet — Design

Status: draft · 2026-11-21

cestafet lets AI coding sessions on one machine — across different Claude Code
profiles (`CLAUDE_CONFIG_DIR`), and later Codex and others — know about each
other per project and pass notices between them. The leading "c" stands for
Claude / Codex / ChatGPT; "estafet" is the relay baton.

## Goals

1. Every session announces itself: which profile, which project, what it is
   working on.
2. A user can open a **coordinator** session for a project; it introduces
   itself to the other sessions in that project.
3. The user talks to the coordinator in plain language — e.g. *"tell the other
   sessions in the project: testing has started, do not deploy to staging"* —
   and the notice reaches every other session in that project, including
   sessions that are busy running tools.
4. No CLI needed for daily use. Everything happens inside Claude.

## Non-goals (MVP)

- Task delegation with lifecycle (`pass`/`done`). Notices only.
- Cross-machine messaging, network ports, accounts.
- Enforcing notices (blocking commands). Sessions are informed, not policed.

## Architecture

```
 session A (profile ~/.claude)       session B (profile ~/.claude-orkono)
   │ plugin: MCP + hooks + skill        │ plugin: MCP + hooks + skill
   ▼                                    ▼
 cestafet mcp (stdio)               cestafet mcp (stdio)
   └──────────────┬─────────────────────┘
                  ▼
   SQLite (WAL) at $XDG_DATA_HOME/cestafet/bus.db  (mode 0600, no network)
```

- One Go binary, `cestafet`: `cestafet mcp` (MCP stdio server),
  `cestafet hook <event>` (hook handler), plus a thin debug CLI
  (`cestafet sessions`, `cestafet notices`, `cestafet doctor`).
- Shipped as a **Claude Code plugin** bundling the MCP server config, hooks and
  a skill. The binary is installed via Homebrew / GitHub Releases / `go install`.
- Pure-Go SQLite (`modernc.org/sqlite`), no CGO; cross-compiled with
  goreleaser.

## Identity

| Field | Source |
|---|---|
| `session_id` | Claude Code hook payload (`session_id`) |
| `profile` | basename of `CLAUDE_CONFIG_DIR`, default `claude` |
| `project` | resolved as described in *Projects* below |
| `cwd`, `branch` | hook payload / git |
| `summary` | what the session is working on (set by the session) |
| `role` | `member` or `coordinator` |
| `last_seen` | updated by every hook call |

Worktrees and different profiles of the same repo share one `project`.
A session not seen for 15 min (configurable) is shown as `idle`; `SessionEnd`
marks it `gone`.

The `SessionStart` hook registers the session automatically: Claude Code runs
`cestafet hook session-start` before the first prompt and passes `session_id`
and `cwd` on stdin; the binary derives profile, project and branch itself.

## Projects

A *project* is the unit notices are scoped to. It can span several
repositories (e.g. a backend, a UI and two workers that make up one product).

Resolution order for a session's `cwd`:

1. **Explicit mapping** in the central config `$XDG_CONFIG_HOME/cestafet/projects.toml`,
   matched by the repo's normalized remote URL, then by path prefix.
2. **Git remote** `origin` of the repo root, normalized (`github.com/orkono/x`).
3. **Repo root** absolute path (repo without remote).
4. **cwd** (not a git repo).

```toml
# ~/.config/cestafet/projects.toml
[projects.tonton]
remotes = [
  "github.com/tontonkaraoke/backend-microservice",
  "github.com/tontonkaraoke/backend-microservice-ui",
  "github.com/tontonkaraoke/tonton-karaoke-analysis-worker",
  "github.com/tontonkaraoke/tonton-karaoke-fast-api-project",
]
paths = []   # for directories that are not git repos
```

Mappings live in one central file, never inside the repositories: shared team
repos stay untouched and every clone or worktree of a mapped remote joins the
project automatically.

Inside Claude, `/cestafet:project <name>` adds the current repo (its remote, or
its path when there is none) to project `<name>`, creating it if needed, and
moves the live session there. `/cestafet:project` without an argument shows the
current project and its member repositories. The same operations are MCP tools,
so plain language works too ("add this repo to the tonton project").

## Installation

```bash
brew install orkono/tap/cestafet   # or: go install github.com/orkono/cestafet/cmd/cestafet@latest
cestafet setup
```

`cestafet setup` is the only CLI step a developer needs:

1. **Discover profiles**: `~/.claude`, `~/.claude-*`, every `CLAUDE_CONFIG_DIR=`
   found in shell rc files and wrapper scripts on `PATH`, and the current
   `CLAUDE_CONFIG_DIR`. A directory counts as a profile only if it holds
   `settings.json` or `projects/`.
2. **Ask for consent**: list the profiles (with the wrapper that uses each) and
   let the developer pick; nothing is changed without confirmation.
3. **Install** into each chosen profile through Claude Code's own CLI
   (`CLAUDE_CONFIG_DIR=<profile> claude plugin marketplace add …` /
   `claude plugin install …`); cestafet never edits `settings.json` by hand.
4. Print a summary; `cestafet doctor` re-checks it later.

`cestafet setup --add <dir>` / `--remove <dir>` change single profiles;
`cestafet uninstall` removes the plugin from every profile and, on request,
the database and config. Non-interactive use: `--yes --profile <dir>…`.

## User experience

All interaction is natural language; Claude picks the MCP tool. The bundled
skill tells Claude when to use cestafet.

```
user (coordinator): be the coordinator for this project
claude:             → coordinate()  "Announced to 3 sessions in orkono/x."

user (coordinator): tell the other sessions: testing started,
                    no deploys to staging
claude:             → notify(body="Testing started. Do not deploy to staging.")
                    "Delivered to 3 sessions; 2 acknowledged."

user (any session): who else is working on this project?
claude:             → sessions()
```

Three slash commands exist only as shortcuts: `/cestafet:coordinate`,
`/cestafet:who` and `/cestafet:project [name]`. Notices are never sent through
a slash command.

## MCP tools

| Tool | Purpose |
|---|---|
| `whoami()` | this session's identity record |
| `introduce(summary)` | set/update what this session is working on |
| `sessions(project?)` | live sessions in the project with role, summary, last_seen |
| `project(name?)` | show the current project, or add this repo to project `name` and move the session there |
| `leave_project()` | remove this repo's mapping; it falls back to automatic resolution |
| `coordinate()` | become coordinator of the project; announces to all members |
| `notify(body, to?)` | send a notice to every other session in the project (or to listed session ids) |
| `notices(unread_only?)` | notices received by this session |
| `ack(notice_id, reply?)` | acknowledge a notice, optionally with a short reply |
| `notice_status(notice_id)` | delivery/ack state per recipient |

One coordinator per project. A new `coordinate()` call takes over and the
previous coordinator is told.

## Delivery

MCP is pull-based, so delivery rides on hooks. Each hook call is a short
`cestafet hook <event>` process that reads the hook JSON on stdin and replies
with `additionalContext` when there is something new.

| Hook | Action |
|---|---|
| `SessionStart` | register session; inject coordinator identity and unread notices |
| `UserPromptSubmit` | heartbeat; inject unread notices; record the user's prompt for provenance (see below) |
| `PostToolUse` | heartbeat; inject unread notices — this reaches sessions in the middle of an autonomous run, before their next tool call |
| `SessionEnd` | mark session `gone` |

Injection format (always delimited, always labeled as data):

```
<cestafet-notice id="n_42" from="coordinator · profile claude-orkono"
                 origin="user" project="github.com/orkono/x" at="2026-11-21T14:03Z">
Testing started. Do not deploy to staging.
</cestafet-notice>
```

A notice is marked delivered once injected; the session acknowledges with
`ack` (the skill asks it to). Hook overhead budget: < 20 ms per call.

To verify during implementation: exact `additionalContext` support for each
hook event in the current Claude Code release, and the optional MCP channel
push for idle sessions (post-MVP).

## Provenance and safety

- A notice is **user-originated** only if its body is backed by the user's
  own prompt: the `UserPromptSubmit` hook stores the raw prompt of the
  coordinator session, and `notify` records the id of the prompt it was
  issued under. Recipients see `origin="user"` (with the quoted instruction
  available) or `origin="agent"`.
- The skill tells recipients: an `origin="user"` notice from the coordinator
  is the user's instruction relayed; an `origin="agent"` notice is a request
  to weigh, never an instruction that overrides the user or local rules.
- This is a local label, not cryptographic proof: any process running as the
  same OS user can write to the database. The threat model is
  prompt-injection hygiene between cooperating agents, not hostile local
  users. Stated plainly in `SECURITY.md`.
- Database file mode 0600; no network listener; bodies capped (4 KB).

## Data model (SQLite)

```
sessions(id PK, profile, project, cwd, branch, summary, role,
         status, started_at, last_seen)
prompts (id PK, session_id, text, at)              -- provenance, 7-day TTL
notices (id PK, project, from_session, origin, prompt_id, body, created_at)
deliveries(notice_id, to_session, delivered_at, acked_at, reply,
           PRIMARY KEY(notice_id, to_session))
```

## Repository layout

```
cmd/cestafet/          main
internal/store/        SQLite schema, migrations, queries
internal/identity/     profile + project resolution
internal/mcpserver/    MCP tools (official Go SDK)
internal/hooks/        hook handlers and injection rendering
plugin/                .claude-plugin/plugin.json, hooks.json, .mcp.json,
                       commands/, skills/cestafet/SKILL.md
docs/                  design.md, plan.md
```

## Open source

Apache-2.0 · Go · goreleaser (GitHub Releases + Homebrew tap) · plugin
published via a marketplace manifest in the repo · README with a two-terminal
demo · `SECURITY.md` · CI: `go test`, `golangci-lint`, race detector.

## Later

Codex adapter (its own hook/MCP surface), MCP channel push for idle sessions,
optional guard hook that warns before commands matching an active notice
(e.g. `deploy.sh staging`), notice expiry ("until tests finish").
