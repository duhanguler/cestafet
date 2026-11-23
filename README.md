<p align="center"><img src="assets/icon.svg" width="128" alt="cestafet icon"></p>

<h1 align="center">cestafet</h1>

**Your AI coding sessions, aware of each other.**

You run several Claude Code sessions at once — different terminals, different profiles
(`CLAUDE_CONFIG_DIR`), different repositories of the same product. cestafet lets them see each
other per project, and lets you tell one of them something that every other session then knows:

<p align="center"><img src="assets/demo.gif" alt="Two Claude Code sessions on different profiles: the coordinator is told to announce that testing started; the other session, busy running tests, receives the notice and acknowledges it"></p>

```
you (coordinator session):  tell the other sessions: testing has started, do not deploy to staging

claude:  → notify  "Testing has started. Do not deploy to staging until further notice."
         Delivered to 3 sessions.

another session, mid-task, before its next tool call:
         <cestafet-notice from="coordinator · claude-work" origin="user" …>
         Testing has started. Do not deploy to staging until further notice.
         </cestafet-notice>
         → ack  "ok, holding all staging deploys"
```

No CLI in daily use: you talk to Claude in plain language. The name is *estafet* — the relay
baton — with a **c** for Claude, Codex, ChatGPT.

## How it works

- **Automatic registration.** A `SessionStart` hook registers every session with its profile,
  repository, branch and project before your first prompt.
- **Projects.** A project is a git repository (its normalized `origin` URL, so every clone,
  worktree and profile of it meets in one place). Several repositories can be grouped into one
  project — e.g. a backend, a UI and two workers — with `/cestafet:project <name>`. Outside git,
  a session joins the nearest parent directory that already has a session.
- **Delivery.** Notices are injected through hooks: at the next prompt (`UserPromptSubmit`) and —
  for sessions busy on their own — after their next tool call (`PostToolUse`), before they act again.
  Subagent tool calls never consume a notice; it waits for the main agent.
- **Standing rules.** "No deploys to staging while testing" is kept as a rule: it stays in force
  until lifted ("testing finished, deploys are allowed again") or until it expires, reaches
  sessions that start later, and is listed to every session when it starts or compacts.
  A rule can carry narrow guard patterns (`*deploy* staging*`): before any session runs a matching
  Bash command, Claude Code asks you first, with the rule as the reason. cestafet never denies on
  its own.
- **Coordinator.** `/cestafet:coordinate` makes a session the project's coordinator and announces
  it to the others.
- **Local only.** One Go binary, one SQLite file (`~/.local/share/cestafet/bus.db`, mode 0600;
  `%LOCALAPPDATA%\cestafet\bus.db` on Windows),
  no daemon, no network. See [SECURITY.md](SECURITY.md).

## Install

```bash
brew install --cask orkono/tap/cestafet                      # macOS / Linux
# or: go install github.com/orkono/cestafet/cmd/cestafet@latest
cestafet setup
```

The binaries are not yet signed or notarized by Apple; the Homebrew cask removes macOS's
quarantine flag after install so Gatekeeper does not block it.

**Windows** (amd64, arm64): download `cestafet_<version>_windows_<arch>.zip` from the
[releases](https://github.com/orkono/cestafet/releases), put `cestafet.exe` in a directory on your
user `PATH` (for example `%USERPROFILE%\.local\bin`, where Claude Code's own installer puts
`claude.exe`), open a new terminal and run `cestafet setup`. The executable is not code-signed, so
SmartScreen may warn the first time. Claude Code runs the hooks through Git Bash, which Claude Code
on Windows needs anyway; `go install` works too.

`cestafet setup` finds every Claude Code profile on the machine (`~/.claude`, `~/.claude-*`, and
every `CLAUDE_CONFIG_DIR` in your shell rc files, PowerShell profiles, wrapper scripts and, on
Windows, your user environment variables), lets you choose, and
installs the plugin into each chosen profile with Claude Code's own `claude plugin` commands.
Restart open sessions afterwards. The binary must be on the `PATH` that Claude Code sees.

The first time Claude uses a cestafet tool it asks for permission; allow it, or add
`"mcp__plugin_cestafet_cestafet__*"` to `permissions.allow` in the profile's settings.

`cestafet doctor` shows which profiles have the plugin and how the current directory resolves.
`cestafet uninstall` removes it again.

## Use

| You say (any language) | What happens |
|---|---|
| `/cestafet:coordinate` or "coordinate this project" | this session becomes coordinator and is announced |
| "tell the other sessions: …" | `notify` — every other session of the project gets it |
| `/cestafet:who` or "who else is working on this?" | `sessions` — profile, repo, branch, status, summary |
| `/cestafet:project tonton` or "add this repo to the tonton project" | this repository joins project `tonton` |
| "did everyone get it?" | `notice_status` — delivered / acknowledged / replies |

Receiving sessions treat a notice as information: a user-origin notice (your own words, relayed)
is respected in their next steps; an agent-origin notice is weighed, never blindly obeyed.

## Debug CLI

```
cestafet sessions [project]   live sessions of a project
cestafet notices [project]    recent notices of a project
cestafet send <project> <text>  send a notice from the terminal (origin=agent)
cestafet doctor               binary and plugin versions, PATH, identity of this directory
```

## Status

v0.3.0. Claude Code only; macOS, Linux and Windows. Push delivery to idle sessions is planned next;
see [docs/roadmap.md](docs/roadmap.md). Design notes: [docs/design.md](docs/design.md),
[docs/v0.2-design.md](docs/v0.2-design.md).

## License

Apache-2.0
