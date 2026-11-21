# cestafet spike protocol (v5, approved by referee round 5 on 2026-11-21)

Supersedes v4. Changes from v4 are marked **[v5]**; earlier [v3]/[v4] marks are kept for history.

## General

- Record `claude --version` and the OS version at the top of docs/spikes.md.
- Workspace: a scratch git repo created with `mktemp -d` outside every real project (no
  CLAUDE.md, no .claude/, no remote). All raw output (hook stdin dumps, MCP dumps, stream-json)
  lives only in that temp dir. docs/spikes.md gets redacted excerpts only.
- **[v3] Synthetic prompts only.** Every prompt is synthetic test text. Native session transcripts
  that Claude Code itself writes under the profile's `projects/` dir are listed separately from
  harness captures, and the spike's own transcript dirs (keyed by the scratch cwd) are deleted at
  the end.
- Auth: no credential is copied or created. Experiments run on one existing working profile with
  per-run flags only.
- **[v3] Profile choice.** Use a working profile with no other live session for the duration of an
  experiment; referee rounds are not run on that profile meanwhile. **[v4]** Profile ownership of
  live processes is checked with `ps -axww -E -o pid=,command=` **[v5]** (all of the user's
  processes, including terminal-less ones) filtered to `claude` processes, matching only the literal
  `CLAUDE_CONFIG_DIR=<dir>` name/value pair in-memory and printing pids only (no environment is
  written anywhere); `pgrep` alone is not used, since it matches argv only. **[v5]** A `claude`
  process with no `CLAUDE_CONFIG_DIR` at all counts as owning the default profile `~/.claude`.
  The same rule, flags and isolation check apply to every profile an experiment touches,
  including the second profile of B7 and the default profile's sibling `~/.claude.json`. Foreign writes, if any are seen, are attributed by
  timestamp and process before any conclusion.
- **[v3] Setting isolation.** Runs use `--setting-sources project,local` (the scratch repo has
  neither, so the profile's user hooks, plugins and settings are excluded) together with
  `--settings <tmp>/settings.json` and `--strict-mcp-config --mcp-config <tmp>/mcp.json`. The
  global CLAUDE.md of the profile still loads; it is recorded (name and size only). One control run
  without `--setting-sources` records what the profile would add, so its effect is known.
- **[v3] Isolation check, fixed before the first run.** Before/after each experiment:
  `settings.json` and the whole `plugins/` tree must be byte-identical (sha256 of every file).
  `.claude.json` is compared semantically: keys `mcpServers`, `enabledPlugins`, `hooks`,
  `oauthAccount`, `customApiKeyResponses` and every other key not on the allowlist must be
  unchanged. **[v4]** The allowlist is enumerated before launch from a dry run on the chosen
  profile (one headless `-p` "ok" run in the scratch dir, diffed key by key) and frozen into
  `<tmp>/allowlist.json`; it may contain only counters, caches, timestamps, tips/release-note
  fields, `backups/`, and `projects[<scratch cwd>]` under both its `/var/...` and `/private/var/...`
  spellings (macOS `mktemp -d` path). A diff on a key outside the frozen allowlist stops the spike.
  **[v4]** A write to `plugins/` is first attributed (timestamp + file): a marketplace auto-update
  of an already-installed marketplace is recorded and the run repeated; anything else stops.
- **[v3] Permissions.** Headless runs use `--permission-mode acceptEdits` plus
  `--allowedTools "Read,Write,Edit,Bash(echo *),Bash(cat *),mcp__spike__*"` with the prompt passed
  via stdin (because `--allowedTools` is variadic). **[v4]** Per-test additions only: `Agent` for
  the B1 subagent run; `ListAgents,SendMessage` for B7; for the B8 smoke test **[v5]** the exact
  documented form `mcp__plugin_cestafet_<server>__*`, where `<server>` is the server name in the
  bundle's `.mcp.json` (read from the bundle, not guessed); if the tool name seen in stream-json
  differs, the run is repeated once with that exact name before it can be labelled. `--dangerously-skip-permissions` and
  `bypassPermissions` are never used. Any permission denial in stream-json makes that run
  INCONCLUSIVE.
- **[v3] Evidence.** Every headless run uses `--output-format stream-json --verbose
  --include-hook-events`; conclusions come from stream-json event order and timestamped hook logs,
  never from the model's self-report. **[v4]** Interactive runs (B1 parity, `/clear`, interactive
  SessionEnd) do not use output-format flags, which apply to print mode only; their evidence is the
  session's transcript JSONL plus the timestamped hook logs. A model merely omitting a marker is INCONCLUSIVE, not FAIL.
- **[v3] Budget.** Every headless run sets `--max-budget-usd 1`; a run cut by budget or quota is
  INCONCLUSIVE and repeated once.
- **[v3] Environment logging.** Environment variables are logged by name only; no value is ever
  written, except an explicit allowlist of non-secret values (`CLAUDE_CONFIG_DIR`, `PWD`,
  `CLAUDE_PROJECT_DIR`). Secret-bearing names such as `CLAUDE_CODE_OAUTH_TOKEN` or `*_API_KEY` are
  never logged with values and never appear in excerpts.
- Stop rule: confirmed failure of B1, B2, B3 or B8 stops the spike and is reported. Every result is
  PASS / FAIL / INCONCLUSIVE with the reason.

## B1 — PostToolUse delivery

- Each hook invocation generates an unpredictable 128-bit hex marker visible only through
  `hookSpecificOutput.additionalContext`.
- Sequential, dependent chain: step n+1 must write the marker injected by step n's PostToolUse into
  file n+1. PASS = every next call carries the previous PostToolUse marker, in stream-json order.
- Parallel batch: several calls in one turn; measure when a notice lands after calls were already
  issued. Recorded as a known limitation, not a failure.
- Interactive parity: once, driven through tmux with the same flags. **[v3]** The workspace trust
  dialog for the fresh scratch dir is expected and is not counted as a plugin trust prompt.
  **[v4]** Evidence from transcript JSONL and hook logs (see Evidence). If an
  interactive run is not possible, B1-interactive is INCONCLUSIVE.
- Subagent: one Agent/Task subagent with tool calls; record whether PostToolUse fires for its calls
  and with which session_id.

## B2 — SessionStart / UserPromptSubmit context and limits

- **[v3]** SessionStart and UserPromptSubmit each generate their own independent unpredictable
  marker; the first tool call must carry both.
- **[v3] Payload test per hook:** a 4 KB body wrapped in the `<cestafet-notice>` envelope, written
  by the model verbatim into a file and compared byte-for-byte with the fixture (sha256); an intact
  end marker alone does not count. 8 KB and 16 KB are exploratory only. **[v4]** The injected body
  is first cross-checked in the hook events / transcript JSONL (what the model received). FAIL only
  when the received body itself is cut (clean truncation); a match in the received body with
  scattered diffs in the model's copy (newline or formatting drift) is INCONCLUSIVE for transcription,
  PASS for delivery. **[v5]** Delivery is judged on the model's input, not on the hook's output: the
  transcript JSONL record that carries the injected context into the model's turn (the attachment /
  system-reminder entry following the hook) is located and its body compared with the fixture.
  PASS = an exact model copy, or a located model-input record that is byte-identical. FAIL = a
  located model-input record that is verifiably cut. If only the hook lifecycle event is available
  (no model-input record found) and the model's copy differs, delivery is INCONCLUSIVE.
- Verify UserPromptSubmit stdin carries the raw prompt text (needed for origin="user").
- SessionEnd fires on normal exit of `-p` and of an interactive session.

## B3 — linking a hook session_id to the MCP server process

- Two concurrent sessions in the same profile and the same cwd.
- From every hook and from the dummy MCP server: pid, the full ancestor chain (pid/ppid/comm up to
  launchd), env names only (see Environment logging), session_id, cwd, transcript_path,
  timestamps — so MCP-vs-SessionStart start order is known.
- **[v3] Nonces.** Each client's prompt carries a distinct call nonce that it passes to the dummy MCP
  tool. PASS = for each nonce, the session_id the MCP server resolves equals the session_id the hooks
  recorded for that same client — so a consistent swap of the two sessions is caught. The same check
  is repeated after every churn step and for the fallback.
- Churn: `--resume`, `/clear` in an interactive session, and an MCP server restart. If the rule
  breaks, the fallback (hook writes a per-ancestor marker file that the MCP server reads on every
  call, not only at startup) is tested in the same run.

## B8 — plugin install into a chosen profile

- An empty temporary CLAUDE_CONFIG_DIR. **[v3]** Snapshot S0 (empty). `claude plugin marketplace add
  <tmp>/marketplace` → snapshot S1. `claude plugin install cestafet@<marketplace>` → snapshot S2.
  Every command must exit 0. **[v5]** Both commands run in a new session with no controlling
  terminal: `python3 -c` with `subprocess.run(cmd, start_new_session=True, stdin=subprocess.DEVNULL,
  capture_output=True)` (calls `setsid()`; macOS ships no `setsid` binary); stdout, stderr and exit
  code are captured to the temp dir. `nohup` is not used.
- **[v3] Registration evidence:** `claude plugin list` in the temp profile shows the plugin
  installed and enabled, and the written files (`installed_plugins.json`, `enabledPlugins`) are
  listed in docs/spikes.md.
- **[v4]** Real profiles are snapshotted immediately before `marketplace add` and immediately after
  `plugin install`; PASS = only the temp profile changed in that window. **[v5]** For real profiles
  this comparison covers `settings.json`, the `plugins/` tree and the non-allowlisted `.claude.json`
  keys only — transcripts written by live sessions in other profiles during the window are out of
  scope and cannot fail it. The later smoke test is
  excluded from this comparison (it legitimately writes a transcript to the working profile) and is
  covered by the General isolation check instead.
- Load check: `--plugin-dir` smoke test on the working profile, headless and once interactively,
  confirms the bundle's hooks and MCP server run without a plugin trust prompt. **[v4]** This run uses
  no `--strict-mcp-config`, no separately configured dummy MCP server and no test hooks in
  `--settings` (strict mode would skip plugin MCP servers, and duplicates would blur the evidence);
  PASS needs a hook event emitted by the plugin's own hook and a successful call to the plugin's own
  MCP tool. **[v3]** This does
  not exercise the installed-plugin load path: automatic loading from the marketplace registry is
  labelled INCONCLUSIVE unless it is observed without `--plugin-dir` (which needs auth in the temp
  profile and is out of scope).
- **[v3] Uninstall:** `claude plugin uninstall` → compare with S1 (marketplace still registered);
  then remove the test marketplace → compare with S0, listing any remaining caches.

## B7 — built-in ListAgents / SendMessage across profiles

- **[v3]** SendMessage is used only toward sessions the spike itself started in the scratch repo,
  never toward a real session. **[v4]** Enforced by a PreToolUse hook in the B7 `--settings` that
  denies any SendMessage whose target is not one of the spike session names, not by the prompt alone. ListAgents output in docs/spikes.md has every real session and
  project name redacted.
- Positive control: two spike sessions in the same profile; PASS needs the receipt of a uniquely
  marked message to be observed in the receiver's stream-json. If headless sessions cannot see each
  other, B7 is INCONCLUSIVE.
- Cross-profile: two spike sessions in two profiles, same uniquely marked message. **[v3]** A
  negative headless cross-profile result stays INCONCLUSIVE even when the control passes.

## Cleanup

- **[v5]** For every profile an experiment touched (the working profile, B7's second profile, and
  the temp profile of B8 before it is deleted), run `CLAUDE_CONFIG_DIR=<profile> claude purge
  <scratch cwd>` — for both `/var/...` and `/private/var/...` spellings if both exist — then the
  final isolation check on that profile; only afterwards are the temp dirs deleted. `claude purge`
  is present in the installed CLI (verified with `claude purge --help` on 2026-11-21: "Delete a
  project's Claude Code data (transcripts, tasks, file history, and its entry in ~/.claude.json)").
  If a future version lacks it, fall back to deleting only the scratch-keyed `projects/` dirs and
  `projects[...]` entries, and record the fallback.

Timebox: half a day; B7 is dropped first if time runs out.

## Implementation notes (round-5 non-blocking risks)

- The process scan filters by the current UID (`ps -axww -E -U "$(id -u)"`), so other users'
  sessions on a shared Mac cannot mark a profile as occupied.
- `CLAUDE_CONFIG_DIR` values are normalized in memory (resolved symlinks, no trailing slash)
  before matching.
- The default profile is purged with `CLAUDE_CONFIG_DIR` unset, so its sibling `~/.claude.json`
  entry is handled by the CLI's own default lookup.
- `claude purge` is run with stdin from `/dev/null` and its non-interactive flag if the help lists
  one; if it cannot run unattended, the recorded fallback applies.
- Redacted evidence excerpts (B1 interactive transcript, B2 model-input record) are copied into
  docs/spikes.md before any purge, since purge deletes them.
