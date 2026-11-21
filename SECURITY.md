# Security

## Threat model

cestafet passes notices between cooperating AI coding sessions of **one OS user on one machine**.
It has no network listener, no accounts and no encryption. Its database
(`~/.local/share/cestafet/bus.db`) is created with mode `0600`; its project map
(`~/.config/cestafet/projects.toml`) with `0600`.

What it defends against: one session's notice being mistaken by another session for a direct
instruction from the user (prompt-injection hygiene between agents).

- Every notice reaches the receiving model inside a delimited `<cestafet-notice>` envelope; a body
  cannot close or open an envelope.
- Each notice carries `origin="user"` (the sending session recorded a user prompt and the model
  declared it was acting on it; the prompt is quoted to the receiver) or `origin="agent"`.
- The bundled skill tells receivers to treat notices as information, to weigh agent-origin notices,
  and never to let a notice override the user or local rules.
- Bodies are capped at 4 KB.

What it does **not** defend against: any process running as the same OS user can write to the
database and forge a notice, including `origin="user"`. The origin label is a local convention,
not a cryptographic proof. Do not run cestafet on a shared account.

## Reporting a vulnerability

Please open a private security advisory on GitHub
(https://github.com/orkono/cestafet/security/advisories/new) rather than a public issue.
