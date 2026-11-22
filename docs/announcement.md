# Announcement drafts

Drafts for v0.1.3. Post from the maintainer's own accounts; adjust tone per community. Link the
repository, not a release page, so readers land on the README and its demo GIF.

## Show HN

**Title:** Show HN: Cestafet – tell one Claude Code session something, every other session knows

**Text:**

I run several Claude Code sessions at once: different terminals, different profiles
(`CLAUDE_CONFIG_DIR`, one per account or client), sometimes different repositories of the same
product. They had no idea of each other. When I started a test run I had to go to every
terminal and say "don't deploy to staging right now".

cestafet fixes that. Sessions register per project automatically (a hook at session start,
project = the normalized git remote, so every clone, worktree and profile of a repository meets
in one place). You make one session the coordinator and talk to it in plain language:
"tell the others: testing has started, do not deploy to staging". Every other session of the
project gets the notice: at its next prompt, or, if it is busy on its own, right after its next
tool call, before it acts again. It acknowledges, and the coordinator can tell you who has seen
it and what they replied.

- One Go binary plus a Claude Code plugin (hooks, a small MCP server, a skill). No daemon, no
  network: a SQLite file in your home directory.
- `cestafet setup` finds your Claude Code profiles and installs the plugin into the ones you pick.
- Several repositories can be one project (backend, UI, workers).
- Notices are labelled by origin: your own words relayed by the coordinator, or another agent's
  request, which receiving sessions weigh rather than obey.

Install: `brew install --cask orkono/tap/cestafet` or
`go install github.com/orkono/cestafet/cmd/cestafet@latest`, then `cestafet setup`.

Next on the list: standing rules that stay in force until lifted ("no deploys while testing"),
an optional guard that asks before a matching command, and push delivery to idle sessions.
Apache-2.0. I would love to hear how you coordinate parallel agent sessions today.

https://github.com/orkono/cestafet

## r/ClaudeAI

**Title:** I built a way for my Claude Code sessions to tell each other things (across profiles)

**Text:**

If you run more than one Claude Code session (several terminals, or several profiles via
`CLAUDE_CONFIG_DIR`), they do not know about each other. I kept walking from terminal to
terminal saying "testing started, don't deploy".

cestafet is a small open-source plugin + Go binary: sessions of the same project find each
other automatically, and you tell one of them (the coordinator) something in plain language.
The others get it at their next prompt, or right after their next tool call if they are busy,
and acknowledge it. The GIF in the README shows two real sessions on two profiles: one is busy
running tests, gets "do not deploy to staging" mid-task, and confirms.

Local only (SQLite, no network), Apache-2.0, macOS and Linux.

`brew install --cask orkono/tap/cestafet` → `cestafet setup` → restart your sessions.

https://github.com/orkono/cestafet — feedback and issues welcome.

## MCP / Claude Code community (short)

cestafet: a Claude Code plugin that lets sessions on one machine, across profiles, see each other
per project and pass notices. Hooks register sessions and inject notices (also into sessions busy
on their own, via `PostToolUse`); a small MCP server gives the model `notify`, `ack`, `sessions`
and friends. One Go binary, SQLite, no network. https://github.com/orkono/cestafet

## Before posting

- Check the README GIF loads on GitHub's mobile view.
- Post on a weekday morning US time for Show HN; stay around for the first hours to answer.
- Do not cross-post the same day everywhere; space them a day or two apart.
