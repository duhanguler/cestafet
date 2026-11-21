# Spike results (fast path, 2026-11-21)

Claude Code 2.1.330, macOS 26 (Darwin 25.4). At the user's request for the fastest MVP, the full
referee-approved protocol (docs/spike-protocol.md) was cut down to the checks that could block the
MVP; the remaining items stay open and are listed below.

| Item | Result | Evidence |
|---|---|---|
| B1 PostToolUse delivery | **PASS** (headless) | Hook returned a random marker via `additionalContext` after each tool call; the next sequential tool call echoed exactly that marker (stream-json order). In the end-to-end run a session busy with `sleep` calls received two notices after a tool call and acknowledged both before continuing. |
| B2 SessionStart / UserPromptSubmit context | **PASS** (headless, small payloads) | Both markers were reported by the model; orientation text from SessionStart was used. 4 KB payload and interactive parity not yet tested. |
| B3 hook ↔ MCP linking | **PASS** (two concurrent sessions, same profile and cwd) | Hooks and the MCP server both resolve the nearest `claude` ancestor process; each session's MCP calls (`coordinate`, `notify`, `ack`) acted on its own session. `/clear`, `--resume` not yet tested. |
| B7 built-in ListAgents across profiles | not tested | — |
| B8 plugin install into a chosen profile | partly | `claude plugin validate` passes for marketplace and plugin; bundle loaded through `--plugin-dir` with hooks and MCP server working. Marketplace install into a temp profile not yet run. |

Hook process ancestry observed: `bash → claude → …` (the hook command runs through a shell whose
parent is the session's `claude` process).

Open: 4 KB/8 KB payloads, interactive parity, `/clear` and `--resume` churn, subagent
`session_id`, marketplace install/uninstall in a temp profile, B7.
