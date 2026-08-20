# Contributing

Thanks for looking. This is a small Go project with a deliberately shallow
dependency graph, and the conventions below are most of what you need to send a
PR that lands.

The one thing to understand before anything else: **a draft is an entry with
no commit hash.** That's the whole state machine — no scratch file, no
"publish" step, nothing to promote. Every convention below (the on-disk
format contract, the package boundaries, the gate) exists to keep that
property true, so it's worth having in your head before you touch anything.

## Get set up

Requires Go 1.25 or newer. No other toolchain, no code generation, no build
tags, and the viewer has no build step — its assets are plain files embedded
into the binary.

```bash
git clone https://github.com/craigjmidwinter/katra
cd katra
make build          # -> ./bin/katra, and with `make all`, ./bin/katra-mcp
make test           # go test ./...
make lint
```

That should be green on a clean checkout. If it is not, that is a bug — open an
issue.

| Target | What it runs |
| --- | --- |
| `make build` | `go build` into `./bin/katra`, stamping the version from `git describe`. |
| `make build-mcp` | The same for `./bin/katra-mcp`. |
| `make all` | Both of the above. |
| `make install` | Both binaries into `GOBIN`, version-stamped. This is the one to use locally. |
| `make test` | `go test ./...` |
| `make lint` | `golangci-lint run` **if it is installed**, otherwise `go vet ./...` |
| `make fmt` | `go fmt ./...` |
| `make tidy` | `go mod tidy` |
| `make release-check` | `goreleaser check` — validates `.goreleaser.yml` only, no build. |
| `make snapshot` | A full local release build into `./dist` (needs goreleaser). |
| `make clean` | Removes `bin/` and `dist/`. |

Cutting an actual release is a separate runbook, not a `make` target:
[RELEASING.md](RELEASING.md).

**`golangci-lint` is not required and is probably not installed.** `make lint`
prints `golangci-lint not found; falling back to go vet` and runs `go vet`. That
fallback is the supported path — do not add a hard dependency on golangci-lint,
and do not commit a change whose only justification is a linter you have locally
and nobody else does. `go vet` and `gofmt` are the floor.

Run these before you push:

```bash
make fmt
make lint
make test
```

### The binary you are testing is probably not the one running

This bites everyone once. Three things resolve `katra` from your `PATH`, not
from your checkout:

- every project's `.git/hooks/post-commit` auto-stamp,
- every project's Claude Code hooks (`katra agent-hook …`),
- the hub launchd agent, which runs a fixed absolute path.

So `make build` alone changes nothing about how katra behaves in your other
repos. Use `make install`, and restart the hub afterwards:

```bash
make install
launchctl kickstart -k "gui/$(id -u)/com.katra.hub"   # macOS, if you run the hub
```

The viewer's CSS and JS are embedded with `go:embed`, so editing
`internal/viewer/assets/` and refreshing the page shows you the *old* asset
until you rebuild. Livereload reloads the page, not the binary.

## Repository layout

```
cmd/katra/               the CLI entrypoint — main() and the version var, nothing else
cmd/katra-mcp/           the MCP server entrypoint
internal/
  core/                  the engine: store, entries, rendering, git, memory, reconcile
  cli/                   cobra command tree, flags, output — no behavior
    embed/SKILL.md       the Claude Code skill `katra setup` installs into a repo
  viewer/                the live server, the static build, the cross-project hub
    assets/              plain HTML/CSS/JS, embedded — no bundler, no build step
  mcpserver/             stdio MCP server over the same core operations
docs/                    the documentation site (GitHub Pages, /docs on main)
examples/                worked configs, and an entry exercising every component
contrib/                 launchd agent and systemd unit for the hub daemon
skills/katra/SKILL.md    symlink to the embedded skill, at the plugin path
.claude-plugin/          Claude Code plugin + marketplace manifests
katra/                   this project's own dev log — do not edit by hand
```

Two directories with rules attached:

- **`katra/`** is katra's own chronicle, written by the tool as the tool was
  built. Entries, decisions, epics and task specs. It is a historical record,
  not a spec to keep in sync — where it disagrees with the code, **the code is
  right**. Do not hand-edit it; if you need an entry, use the CLI.
- **`internal/cli/embed/SKILL.md`** is the canonical skill — it has to live
  there so `go:embed` can pick it up and `katra setup` can install it into a
  repo. `skills/katra/SKILL.md` is a **symlink** to it, at the path the Claude
  plugin packaging expects. Edit the embedded copy; the symlink is not a second
  file, and adding one is how the installed skill and the published skill drift
  apart.

## Package boundaries

The dependency graph is acyclic and shallow, and keeping it that way is most of
the design.

- **`internal/core` imports no command framework.** It is plain data,
  filesystem and git. The CLI, the git hook, the MCP server and the viewer are
  four front ends over the same operations, and core is what makes them agree.
  If a behaviour lives in `cmd_*.go`, the MCP server does not have it.
- **`cmd/` holds no behavior.** It parses flags, calls core, maps errors to exit
  codes, and prints. If you find yourself writing logic in a `RunE`, it belongs
  in core where it can be tested without the CLI.
- **A new operation is added to `core` first**, then exposed. Adding it to the
  CLI only is how the two surfaces drift, and they have drifted before.
- **The viewer renders; it does not decide.** `internal/core/render.go` turns
  markdown and fenced components into HTML and `data.json`. The viewer serves
  that. A component whose behaviour depends on the viewer cannot be produced by
  `katra build`, which has to work offline as static files.
- **Errors carry the fix, not just the fault.** The house style is an error that
  tells the user what to do: `no active draft — start one with katra new "…"`,
  `not a katra repo (no katra/ directory); run katra init`. Match it.

## Adding a rich component

This is the most common change, and it is deliberately a small one. A component
is a fenced code block whose language names it:

````markdown
```compare
before: media/a.png
after:  media/b.png
```
````

Adding one means:

1. A `ComponentFunc` registered in `internal/core/render.go`. That is the entire
   extension surface — one function that takes the fence body and returns HTML.
2. A row in `docs/components.md`, with a worked example.
3. A block in `examples/entry.md`, which CI renders on every push. A component
   that is not in that file is a component nobody notices breaking.

Two constraints on what a component may do:

- **It renders to self-contained HTML.** No external requests, no CDN scripts,
  no web fonts. `katra build` produces a directory you can open from a USB stick
  on a plane, and that is a property worth keeping.
- **An unknown language degrades to a plain code block.** Never error on a fence
  you do not recognise: an entry written against a newer katra must still render
  in an older one, just less prettily. This is the compatibility rule for the
  on-disk format generally.

## The on-disk format is the contract

An entry is a markdown file with YAML frontmatter, and people's git history is
full of them. The compatibility rule is one-directional and strict:

- **Adding an optional frontmatter key is fine.** Older katras ignore it.
- **Renaming or repurposing a key is not**, and neither is making an optional
  key required. Someone's 2026 entry has to render in next year's binary.
- **A draft is an entry with no `hash`.** That is the whole state machine, and
  it is why nothing gets stuck in a scratch file. Do not add a second way to be
  a draft.

## Testing

`go test ./...` is the whole story. Some conventions the existing tests hold to:

**Table-driven, always.** Named cases in a slice, one subtest each:

```go
tests := []struct {
	name    string
	input   string
	want    string
}{
	{name: "husky shim dir redirects to tracked", input: ".husky/_", want: ".husky"},
	// ...
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		// ...
	})
}
```

**Filesystem tests use `t.TempDir()`.** No writing into the repo, no shared
fixture directories, no cleanup code. `newTestStore(t)` in
`internal/core/store_test.go` gives you an initialized store in a temp dir.

**Set `KATRA_REGISTRY` before running the CLI by hand.** `katra init` registers
the new store with the machine-global hub registry, so scaffolding throwaway
katras to try something out leaves dead entries in *your* registry and on your
hub board:

```bash
export KATRA_REGISTRY="$(mktemp -d)/registry.yml"
```

CI does this for the same reason. `katra hub list` prunes entries whose store is
gone, so the damage self-heals once the temp directory is deleted — but a
`mktemp -d` directory survives until you reboot, and until then the board shows
projects that are really test fixtures.

**Tests that need git build a real repo.** `git init` into a `t.TempDir()`, set
`user.email` and `user.name`, and drive the real binary's code paths — see
`initRepoWithStore` in `internal/core/git_hook_test.go`. Do not mock git; the
things that break are its actual behaviours, like `rev-parse --git-path`
resolving relative to the working directory.

**No network. Ever.** A test that talks to the internet is not accepted. The
viewer's tests drive handlers directly rather than binding a port, and the hub's
project set is injected as a loader function precisely so it can be faked:

```go
set := newHubSet(func() ([]HubProject, error) { return projects, nil })
```

**Concurrency gets `-race` and a real test.** The hub refreshes its registry on
a ticker while handlers read it. CI runs `go test -race ./...`; if you add
shared state, add the test that would catch it.

**Test the error text you promise.** Where an error message is part of the user
interface — and in this project most of them are — assert on it.

## Documentation

Documentation that describes behavior the code does not have is worse than no
documentation. If you change any of these, update the docs in the same PR:

| You changed | Update |
| --- | --- |
| A command or flag | `docs/cli.md` and the README's command table |
| A `config.yml` key, default, or validation rule | `docs/configuration.md` |
| A rich component | `docs/components.md`, `examples/entry.md`, and the README's component list |
| The entry frontmatter or the on-disk layout | `docs/format.md` |
| An MCP tool's name, arguments, or return shape | `docs/agents.md` |
| The skill, the hooks, or the commit gate | `internal/cli/embed/SKILL.md` and `docs/agents.md` |
| The hub, the registry, or the launchd agent | `docs/hub.md` and `contrib/` |
| Anything about the seams | `docs/architecture.md` |
| The viewer's visual output (layout, the Overview spine, the Board columns, an entry page) | Regenerate the screenshots: `scripts/capture-screenshots.sh` |

**The examples are tested, not decorative.** CI scaffolds a katra, drops each
`examples/config/*.yml` in as the real config, and builds the site; then it
renders `examples/entry.md` and checks the build is clean. An example that has
drifted fails the build, which is the point.

**The screenshots are not CI-gated, on purpose.** Headless font rendering
(hinting, subpixel AA, fallback-font selection) differs enough across CI
runners that a byte-for-byte staleness check would flap on font metrics, not
on real drift in the viewer. `scripts/capture-screenshots.sh` is the
regeneration contract instead of a CI check: rerun it when the viewer's
visual output changes and commit the new PNGs in the same PR as the change
that caused them.

## Pull requests

- **One concern per PR.** A new component and a refactor of the hub are two PRs.
- **Tests with the change**, in the same commit. New component, new example
  block. New error path, a test that hits it.
- **`make fmt lint test` green** before you push.
- **Say what you verified.** Especially for the hooks and the hub: unit tests do
  not exercise a real Claude Code session or a login-time daemon, so if you ran
  it for real, say so and paste the (redacted) output.
- **No new dependencies without a reason in the PR description.** The current
  set is four direct dependencies and that is deliberate.

## A note on the katra in this repo

katra is used to develop katra, so a PR may arrive with an entry in `katra/`
describing it. That is welcome and not required. If you do include one, it is
subject to the same rule as the rest: it should read as a post for a person, not
a handoff note for the next agent.

## License

MIT. See [LICENSE](LICENSE). By contributing you agree your contributions are
licensed under it.
