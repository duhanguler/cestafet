# Claude plugin directory submission

Prepared answers for the developer portal at <https://claude.ai/directory/manage>
(**Submit new → Plugin bundle**). Submitting needs a paid claude.ai plan; the maintainer submits
from their own account.

`claude plugin validate ./plugin --strict` passes with no findings (icon, license, manifest).

## 1. Source

| Field | Value |
|---|---|
| Repository | `orkono/cestafet` (public) |
| Plugin path | `plugin` |
| Branch or tag | `main` |

## 2. Listing details

Read from `plugin/.claude-plugin/plugin.json` and the repository `README.md`. The icon
(`plugin/.claude-plugin/icon.png`, 512 × 512) becomes the listing icon only the first time the
plugin is saved or submitted; changing it later does not change the listing.

## 3. Data handling

| Question | Answer |
|---|---|
| Reads or stores personal data? | Yes, locally only. A SQLite file in the user's home directory (`~/.local/share/cestafet/bus.db`, mode 0600) stores each session's profile directory name, working directory, git remote and branch, the notices sessions send each other, and the text of the user's prompts in a session (kept 7 days, to label notices the user asked for). |
| Sends data to services other than declared connectors? | No. cestafet makes no network connections. |
| How long is data kept? | Prompt text: 7 days. Sessions and notices: until the user deletes the database file. |
| Intended for people under 18? | No. |

## 4. Compliance

Contact email and the four acknowledgements: the maintainer's own; read each before ticking.

## 5. Review and submit

- Update delivery: GitHub push webhook.
- Auto-publish: off for the first version (a reviewer publishes it).

## Expected review questions

- **The plugin runs a binary outside its folder.** Hooks and the MCP server run `cestafet` from
  `PATH`; the plugin folder only holds manifests, hooks config, commands and a skill. The binary
  is a compiled Go program of more than 10 MB and cannot be bundled (the directory holds compiled
  executables and caps plugin files at 5 MiB). Its source is the same public repository
  (`cmd/`, `internal/`); releases are built by GitHub Actions with GoReleaser from tags, with
  checksums. The README and `plugin.json` description state the binary requirement. The
  directory's checklist does not say how it treats commands from `PATH`, so a hold or a
  rejection on this point is possible.
- **Claude Code only.** Hooks and the local MCP server need Claude Code (or Cowork on the
  user's machine); claude.ai chat ignores them.
- **Hooks inject context.** `SessionStart`, `UserPromptSubmit` and `PostToolUse` add notices
  from the user's other sessions to the context; `PreToolUse` (Bash) asks the user before a
  command that a standing rule guards. cestafet never denies a tool call and never changes
  permission settings.

## What a listing does not give

A directory listing is served as `cestafet@synced`; it does not put the plugin into
`claude-plugins-official`, which is the default allowlist for channel plugins. Push delivery to
idle sessions (v0.3) therefore still needs `--dangerously-load-development-channels` or an
organisation's `allowedChannelPlugins` setting. An official-marketplace listing goes through an
Anthropic partner contact, not the portal.
