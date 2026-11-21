# Releasing

Releases are cut from `main` by pushing a `vX.Y.Z` tag. The `release` workflow runs GoReleaser:
binaries for macOS and Linux (amd64, arm64), `checksums.txt`, a GitHub release with generated
notes and — once the tap token exists — a Homebrew cask in `orkono/homebrew-tap`.

## Checklist

1. `main` is green in CI; `docs/roadmap.md` moves the shipped items to "Done for vX.Y.Z".
2. Bump `version` in `plugin/.claude-plugin/plugin.json` to `X.Y.Z` in the release PR. The release
   workflow refuses a tag that does not match it, because Claude Code only offers a plugin update
   when this version changes.
3. Tag the merge commit and push the tag:
   ```bash
   git tag -a vX.Y.Z <sha> -m "cestafet vX.Y.Z" -m "<one-paragraph summary>"
   git push origin vX.Y.Z
   ```
4. Watch the `release` workflow; check the assets and `checksums.txt` on the release page.
5. Smoke test: download the darwin/arm64 archive, verify its checksum, run `cestafet version`.
6. Users update with `go install github.com/orkono/cestafet/cmd/cestafet@vX.Y.Z` (or
   `brew upgrade --cask cestafet`), then `claude plugin marketplace update cestafet` and
   `claude plugin update cestafet@cestafet` per profile when the plugin changed (`cestafet setup`
   does both).

## Homebrew tap (one-time setup)

1. Create the public repository `orkono/homebrew-tap` (empty, default branch `main`).
2. Create a fine-grained personal access token with **Contents: read and write** on
   `orkono/homebrew-tap` only.
3. Add it to `orkono/cestafet` as the Actions secret `HOMEBREW_TAP_TOKEN`.

Until the secret exists, `skip_upload` makes GoReleaser skip the cask instead of failing the
release. After it exists, every release writes `Casks/cestafet.rb` to the tap, and users install
with `brew install --cask orkono/tap/cestafet`.

## Validating the configuration locally

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser check
goreleaser release --snapshot --clean --skip=publish   # writes dist/, publishes nothing
```
