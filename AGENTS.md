# AGENTS.md

katra is a committed, markdown-native dev log: entries live as markdown files
under `katra/`, a draft is an entry with no commit hash, and stamping (adding
the hash + diffstat) is what publishes it. Around the log sits a node model —
tasks, epics, decisions, articles — and a viewer/hub that renders it all.
Written in Go; two binaries, `katra` (CLI) and `katra-mcp` (MCP server).

## Commands

All green on a clean checkout — if one isn't, that is a bug worth reporting.

```bash
make all              # both binaries into ./bin/  (never `go build -o katra` — see below)
go test ./...         # full suite; CI runs it with -race
golangci-lint run     # config in .golangci.yml; falls back to: go vet ./... && gofmt -l .
```

`make build` builds only `./bin/katra`; `make build-mcp` builds only
`./bin/katra-mcp`.

## Harness-neutral Katra workflow

The committed CLI workflow is canonical; a Claude skill or hook may automate
it but must not be the only place it is taught. In every harness:

1. Inspect durable work with `katra task list --status specced`,
   `katra task list --status doing`, and `katra epic rollup`.
2. For a larger outcome, create an epic, then a child task with
   `katra epic new "…"` and `katra task new "…" --epic <slug>`.
3. When design is warranted, author a spec artifact and attach it with
   `katra task spec <task> <node-slug-or-repo-path>`. Commit the artifact and
   task pointer together before implementation, verify the task is `specced`,
   read the artifact, then run `katra task start <task>`.
4. Before implementation edits, open the running chronicle with
   `katra new "…"`. Use `katra append` for decisions and rejected alternatives,
   `katra decide` for a durable choice, and `katra capture`/`compare` for at
   least one visual whenever the work can be shown.
5. Commit the bounded implementation. Publish with
   `katra stamp --closes <task> --commit`; this closes and links the task and
   rolls up its epic. If the portable post-commit hook is installed, run
   `katra reconcile --close <task>` before the implementation commit so the
   closure is already attached to the draft.
6. Finish with `katra doctor`, task/epic status checks, `katra list`, and a
   clean working tree.

The full public contract and examples live in `docs/workflow.md`. Claude Code
memory ingest is an optional adapter, not an assumption: Codex and plain-shell
sessions must append the reasoning worth publishing themselves.

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
- **The `.mcpb` bundle is the second listing, and it must not drift from the
  first.** `packaging/mcpb/manifest.json.tmpl` copies its `description` from
  `server.json` verbatim, and its `compatibility.platforms` from the
  `katra-mcp` build's `goos` list. `go test ./packaging/...` enforces both, so
  a palette-style silent divergence is a failing test rather than two listings
  making different claims. Bundles are assembled in the build post-hook, not an
  `after` hook, so `checksums.txt` covers them.
- **Task lifecycle** is `todo → specced → doing → done | cut`; `spec:` refs
  resolve as a node slug first, then a repo-root-relative path. Setting a
  spec never moves a status backwards.
- **This repo has a portable post-commit stamp hook and may also have a
  Claude-specific commit gate.** Declare closure durably on the active draft
  with `katra reconcile --close <task>` before committing, or stamp explicitly
  with `katra stamp --closes <task> --commit`. A Claude gate, when active, also
  accepts `katra reconcile --advance|--close <task>` receipts.

## Security notes

- `katra serve` and `katra hub serve` bind all interfaces with no auth, by
  documented design — never point them at untrusted networks in examples.
- `katra/.state/` (memory ledger, receipts) is local machine state and must
  stay gitignored; the memory-ingest pipeline quarantines secret-shaped
  strings and its test fixtures deliberately contain fake credentials
  (`internal/core/memory_test.go`).
- New HTTP handlers in the viewer/hub must keep the existing path-containment
  idiom (`Clean` + `Join` + prefix check) and `html.EscapeString` discipline.
