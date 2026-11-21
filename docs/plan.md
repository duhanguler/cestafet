# Plan — v0.1.x and v0.2

Each task is test-first (RED → GREEN → refactor) and lands as its own PR unless noted.
Tracking: GitHub milestones `v0.1.1` and `v0.2.0`.

## v0.1.1 — patch

| # | Task | Owner | Notes |
|---|---|---|---|
| P1 | Cap the context a single hook injects (12 KB); the rest waits for the next hook | Claude | Body cap is 4 KB, but several pending notices are injected together. Test: five 4 KB notices → two injections, nothing lost, order kept. |
| P2 | Homebrew cask | Osman + Claude | Config is ready and skips while the secret is missing (`docs/release.md`). Needs `orkono/homebrew-tap` and the `HOMEBREW_TAP_TOKEN` secret. |
| P3 | Release guard: tag must match `plugin.json` version | Claude | Done in this PR. |
| P4 | GitHub Actions on Node 24 (checkout v7, setup-go v7, goreleaser-action v7) | Claude | Done in this PR. |
| P5 | README demo GIF: two terminals, coordinator notifies, busy session acknowledges | Claude | Record with a scripted tmux run; keep under 2 MB. |
| P6 | `cestafet doctor`: binary version vs. installed plugin version, PATH visible to Claude Code | Claude | Catches the "old binary on PATH" case seen after v0.1.0. |
| P7 | Announcement draft (Show HN, r/ClaudeAI, MCP community) | Claude | After P2 and P5. |

## v0.2.0 — standing rules, push, guard

Design: [v0.2-design.md](v0.2-design.md).

| # | Task | Depends on |
|---|---|---|
| R1 | Store: `kind`, `expires_at`, lifting columns; migration from v0.1 schema | — |
| R2 | Store: active-rule queries; backfill = active rules (any age) + recent info | R1 |
| R3 | MCP: `notify` with `standing` / `expires_in`; new `lift`, `rules` | R2 |
| R4 | Hooks: orientation lists active rules; re-list on `source: "compact"` | R2 |
| R5 | Skill text for rules and lifting; e2e: rule → lift with two live sessions | R3, R4 |
| C0 | **Spike**: channel payload, activation detection, behaviour without `--channels`, allowlist path | — |
| C1 | MCP: `claude/channel` capability, 2 s poll, `pushed_at`, dedup per C0 | C0, R2 |
| C2 | `cestafet setup`: print opt-in launch line per profile | C1 |
| G1 | Guard patterns on rules: validation, matching | R1 |
| G2 | `PreToolUse` hook (Bash): `ask` with reason on a match | G1, R2 |
| G3 | Skill: when to attach guards; e2e: guarded rule makes a staging deploy ask | G2 |
| X1 | Schema migration test from a v0.1.0 database | R1 |
| X2 | Release v0.2.0 (`docs/release.md`) | all |

Order: R1–R5 first (the core value), then G1–G3, then C0–C2 (depends on Claude Code's channel
constraints, may slip to v0.3 if the spike shows no reliable dedup).
