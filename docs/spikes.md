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

## Follow-up checks before v0.1.0 (2026-11-21)

| Item | Result | Evidence |
|---|---|---|
| B2 4 KB payload | **PASS** | A 3993-byte notice (begin/end markers, 60 numbered lines) sent with `cestafet send` reached the main agent; the model-input record in the session transcript holds the full body, and the model's verbatim copy written to a file is byte-identical. |
| Subagent tool calls | **FIXED** | PostToolUse for a subagent's tool call carries the parent `session_id` plus `agent_id` / `agent_type`. Before the fix a notice was injected into the subagent's context and marked delivered, so the main agent never saw it. Now subagent calls only refresh `last_seen`; the notice waits for the main agent's next hook. Re-run: main agent received and acknowledged it. |
| `--resume` | **PASS** | Same `session_id`; the session row is refreshed, no duplicate. |
| Interactive parity (B1) | **PASS** | Interactive session in tmux: a notice sent during a `sleep` arrived after the next tool call and was acknowledged. The workspace trust dialog for the fresh directory is Claude Code's own, not a plugin prompt. |
| `/clear` (B3 churn) | **FIXED** | `/clear` fires SessionEnd with `reason: "clear"`, then SessionStart with `source: "clear"` and a new `session_id` in the same claude process. SessionEnd(clear) is now ignored; registering the new id retires the old one and carries the coordinator role over. Re-run: `whoami` after `/clear` returned the new id with role coordinator; `/exit` then ended it. |
| B7 built-in ListAgents across profiles | INCONCLUSIVE | ListAgents works headless in two profiles but returns different lists (25 vs 13 rows), suggesting per-profile or per-account scope; rows carry no profile or path, so cross-profile visibility could not be confirmed. Not needed by cestafet. |

Still open: 8 KB / 16 KB exploratory payloads.
