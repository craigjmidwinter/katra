# Releasing

The runbook for cutting a `craigjmidwinter/katra` release. Tagging a commit
`vX.Y.Z` and pushing the tag is the only manual step — everything after that
is [`.github/workflows/release.yml`](.github/workflows/release.yml) and
[`.goreleaser.yml`](.goreleaser.yml).

- [ ] 0. Preconditions
- [ ] 1. Pick the version
- [ ] 2. Dry run (optional but cheap)
- [ ] 3. Tag and push
- [ ] 4. What happens automatically
- [ ] 5. Verify
- [ ] 6. Yank a bad release

---

## 0. Preconditions

- `main` is clean and pushed — `git status` empty, `git log origin/main..main`
  empty.
- CI is green on that commit. Three jobs gate this in
  [`.github/workflows/ci.yml`](.github/workflows/ci.yml): `build / vet / test`,
  the `scaffold / stamp / build` smoke test (which builds `katra` and drives it
  through init → entry → stamp → doctor → build in a scratch repo), and
  `release dry run`, which already runs `goreleaser check` and a full
  `--snapshot` build on every push. If that job is green, the release config
  itself is not going to be the thing that breaks step 3.
- There is something worth releasing — `git log <last tag>..HEAD --oneline`.
  If no tag exists yet, that's every commit; see "Pick the version" below for
  what that means for the first one.
- `CONTRIBUTING.md`'s checklist is honestly true for what landed since the
  last release: docs updated where a command/flag/config key/component/format
  changed, and no on-disk shape broken that an existing katra couldn't still
  parse.

## 1. Pick the version

```bash
git tag        # what exists already — check the convention before inventing one
```

No release has shipped yet — `git tag` is currently empty. The first one is
**`v0.1.0`**, matching the "Pre-1.0" framing in README's *Status and scope*:
command names and flags may still move, so nothing here should claim `1.0.0`
yet. After that, ordinary semver judgment: patch for a fix, minor for a new
command, component, or config key, and `1.0.0` only once the CLI surface and
on-disk format are a promise rather than a draft.

The tag must carry the `v` prefix — every consumer expects it:

- `.goreleaser.yml`'s `ldflags` stamps the binary as `v{{ .Version }}` (goreleaser
  strips the `v` for `.Version` and this puts it back), matching what
  `make build` stamps locally from `git describe`.
- README's install snippets (`Download a binary`, `go install`) all construct
  `v${VERSION}` URLs and module paths.
- `release.yml` only triggers on `push: tags: v*`.

**Commit message prefixes affect the generated changelog**, not just style:
`.goreleaser.yml`'s `changelog.filters.exclude` drops any commit whose subject
starts with `docs:`, `test:`, `chore:`, `ci:`, or `katra:` (this repo's own
devlog commits — bookkeeping, not a change to the tool), plus merge commits.
Anything else since the last tag shows up verbatim in the release notes, so a
scan of `git log <last tag>..HEAD --oneline` before tagging is the cheapest
way to catch a commit message that reads fine in `git log` but wrong on a
release page.

## 2. Dry run (optional but cheap)

Locally, without touching the network:

```bash
make release-check   # `goreleaser check` — validates .goreleaser.yml only
make snapshot         # full local build: both binaries × 4 platform archives,
                       # checksums, into ./dist — no publish, no signing
```

`make snapshot` needs [goreleaser](https://goreleaser.com) installed. It skips
`publish,sign,announce`, so nothing is signed and nothing reaches GitHub; the
Homebrew cask step becomes a no-op unless you export
`HOMEBREW_TAP_GITHUB_TOKEN` yourself.

To dry-run through the actual GitHub Actions environment before committing to
a real tag:

```bash
gh workflow run release.yml -f dry_run=true   # dry_run defaults to true
gh run watch
```

This runs the same job as a real release with `--snapshot --clean --skip=publish,sign`
and uploads `dist/*.tar.gz` + `checksums.txt` as a workflow artifact (7-day
retention) so you can inspect exactly what a tag would produce.

## 3. Tag and push

```bash
git tag v0.1.0
git push origin v0.1.0
```

Pushing the tag — not the workflow_dispatch dry run — is what fires the real
release: `release.yml`'s `push: tags: v*` trigger runs `goreleaser release
--clean` with nothing skipped.

## 4. What happens automatically

| Step | What it does |
| --- | --- |
| Homebrew tap token check | Reports in the run summary whether `HOMEBREW_TAP_GITHUB_TOKEN` is set. If it isn't, the release still ships — this only decides whether the cask update below runs — but the job posts a loud `::warning` and a summary block explaining the fix, rather than silently skipping. |
| `goreleaser release --clean` | Builds `katra` and `katra-mcp` for darwin/linux × amd64/arm64 (`CGO_ENABLED=0`, `-trimpath`), version-stamped `v{{ .Version }}`. |
| Archives | One `tar.gz` per platform (`katra_<version>_<os>_<arch>.tar.gz`), each containing **both** binaries plus `README.md`, `LICENSE`, `examples/`, and `contrib/**` (the launchd/systemd hub-daemon files) — a binary download is a complete install, not just the tool. |
| Checksums | `checksums.txt`, sha256 over every archive. |
| Cosign signing | Keyless `cosign sign-blob` over `checksums.txt`, using GitHub OIDC (`id-token: write`) rather than a stored key. Produces `checksums.txt.sig` and `checksums.txt.pem`, and the signature is logged to the public Rekor transparency log. |
| GitHub release | Created from the tag. Changelog is generated from commits since the last tag (see the filter rules in step 1). `prerelease: auto` — a tag with a semver prerelease segment (`v0.2.0-rc1`) is flagged prerelease and won't become "latest". |
| Homebrew cask | If `HOMEBREW_TAP_GITHUB_TOKEN` is set, bumps `Casks/katra.rb` in [`craigjmidwinter/homebrew-tap`](https://github.com/craigjmidwinter/homebrew-tap) via a cross-repo PAT (the default `GITHUB_TOKEN` can't write to another repo). The cask's post-install hook also strips the macOS quarantine attribute from both binaries — this is why the README tells `go install`/manual-download users to do that step by hand, but Homebrew users don't need to. |
| `docs-note` job | Posts a note to the run summary that the docs site (`craigjmidwinter.github.io/katra`) is served continuously from `/docs` on `main`, not pinned to the tag — nothing to do here unless the release changed something the docs don't yet reflect. |

## 5. Verify

- Release page has all five expected files per platform-archive plus the two
  checksum-signing files: four `.tar.gz`, `checksums.txt`, `checksums.txt.sig`,
  `checksums.txt.pem`.
- Run the verification snippet already documented in README's
  [Verify what you downloaded](README.md#verify-what-you-downloaded) section —
  don't duplicate it here, just confirm it against the new tag.
- If `HOMEBREW_TAP_GITHUB_TOKEN` was set: confirm the commit landed in
  `craigjmidwinter/homebrew-tap`, then `brew update && brew upgrade katra` (or
  a fresh `brew install craigjmidwinter/tap/katra`) and check `katra --version`.
- `go install github.com/craigjmidwinter/katra/cmd/katra@vX.Y.Z` in a scratch
  `GOBIN`, confirm `--version` reports the tag.
- Skim the run summary for the Homebrew-tap warning — if it fired and you
  didn't expect it to, the token secret needs attention before the next
  release, not this one (the release itself already shipped fine without it).

## 6. Yank a bad release

There is no real "unpublish" here — treat this as damage control, not undo.

1. **GitHub release and tag.** `gh release delete vX.Y.Z --yes`, then
   `git push --delete origin vX.Y.Z && git tag -d vX.Y.Z`. Do not reuse the
   tag name for the fix — once a tag has been fetched by anyone (including the
   Go module proxy, next), treat it as burned and move to the next patch
   version.
2. **The Go module proxy remembers.** `proxy.golang.org` caches module
   versions the first time anyone resolves them, and there is no delete API —
   `go install .../katra@vX.Y.Z` can keep working for people who already
   resolved that version even after the GitHub tag and release are gone. If
   the bug is serious, add a [`retract`](https://go.dev/ref/mod#go-mod-file-retract)
   directive for the bad version to `go.mod` as part of the fix release, so
   `go install .../katra@latest` and `go list -m -u` both steer people away
   from it.
3. **Homebrew tap.** If `HOMEBREW_TAP_GITHUB_TOKEN` was set, the bad version
   is now also a commit in `craigjmidwinter/homebrew-tap`'s `Casks/katra.rb`
   pointing at archive URLs that no longer resolve. Revert that commit (or let
   the next good release overwrite it — but don't leave the tap pointing at a
   dead release in the meantime, since `brew install` will fail loudly for
   anyone who hits it).
4. Ship the real fix as the next patch tag through the normal process above.
