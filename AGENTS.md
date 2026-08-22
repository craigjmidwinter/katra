# AGENTS.md

katra is a committed, markdown-native dev log: entries live as markdown files
under `katra/`, a draft is an entry with no commit hash, and stamping (adding
the hash + diffstat) is what publishes it. Around the log sits a node model —
tasks, epics, decisions, articles — and a viewer/hub that renders it all.
Written in Go; two binaries, `katra` (CLI) and `katra-mcp` (MCP server).

## Commands

All green on a clean checkout — if one isn't, that is a bug worth reporting.

```bash
make build            # both binaries into ./bin/  (never `go build -o katra` — see below)
go test ./...         # full suite; CI runs it with -race
golangci-lint run     # config in .golangci.yml; falls back to: go vet ./... && gofmt -l .
```

## Style

- `gofmt` clean, no exceptions; match the surrounding comment voice (full
  sentences, why-not-what).
- No new dependencies without a stated reason.
- Error messages teach the next step ("no active draft — pass --entry <slug>
  or `katra new`" is the house pattern).

## Invariants a change must not break

- **The on-disk format is the public API** (`docs/format.md`): an unknown
  frontmatter key is ignored, an unknown fence degrades to a plain code
  block, an absent `type` means `entry`, an unknown `status` degrades
  silently. Breaking any of these needs a migration note.
- **`./katra` is the data directory, not a build target.** This repo dogfoods
  itself; `go build -o katra` would write over the log. Build with
  `make build` (outputs to `./bin/`).
- **Viewer assets are `go:embed`ded.** Editing `internal/viewer/assets/*`
  does nothing until the binary is rebuilt — rebuild before `serve`/`build`.
- **The OCI image is MCP Registry packaging, not a supported container install
  path.** `server.json`, `Dockerfile.registry`, and the manifest annotation in
  `.goreleaser.yml` must keep the server name
  `io.github.craigjmidwinter/katra` in lockstep. Validate metadata with
  `mcp-publisher validate`; never publish it outside a tagged release. Both
  workflows install the publisher through `scripts/install-mcp-publisher.sh`,
  whose pinned version and SHA-256 are the single source of truth.
- **Task lifecycle** is `todo → specced → doing → done | cut`; `spec:` refs
  resolve as a node slug first, then a repo-root-relative path. Setting a
  spec never moves a status backwards.
- **This repo runs its own commit gate.** The Claude Code hooks installed
  here block a stop or a `git commit` until the work is declared:
  `katra reconcile --advance|--close <task>` (slugs from `katra task list`).

## Security notes

- `katra serve` and `katra hub serve` bind all interfaces with no auth, by
  documented design — never point them at untrusted networks in examples.
- `katra/.state/` (memory ledger, receipts) is local machine state and must
  stay gitignored; the memory-ingest pipeline quarantines secret-shaped
  strings and its test fixtures deliberately contain fake credentials
  (`internal/core/memory_test.go`).
- New HTTP handlers in the viewer/hub must keep the existing path-containment
  idiom (`Clean` + `Join` + prefix check) and `html.EscapeString` discipline.
