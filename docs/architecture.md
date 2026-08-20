---
title: Architecture
layout: default
nav_order: 9
description: >-
  How katra is put together — the core engine and its four front ends, the
  render pipeline, the git seam, and where a change belongs when you go to
  make one.
---

# Architecture

Written for someone about to change something and deciding where the change
goes.

## The shape

```
                        ┌──────────────────────────┐
   katra (CLI) ────────▶│                          │
   katra-mcp ──────────▶│      internal/core       │──▶ katra/*.md
   git post-commit ────▶│  store · entries · git   │    (the log)
   Claude Code hooks ──▶│  render · memory         │
                        └────────────┬─────────────┘
                                     │
                             internal/viewer
                          serve · build · hub
```

**`internal/core` is the engine and imports no command framework.** It is plain
data, filesystem and git. The four front ends are thin, and that is the design:
they cannot disagree about what stamping an entry means, because none of them
implements it.

That property is load-bearing rather than decorative. The same publish path runs
whether you typed `katra stamp`, an agent called `katra_stamp` over MCP, or your
own `git commit` fired the post-commit hook. A behaviour added to a `cmd_*.go`
is a behaviour the MCP server does not have — that is the drift to watch for,
and it has happened.

## Where does my change go?

| You want to change | Go to |
| --- | --- |
| What a command is called, its flags, its output | `internal/cli/cmd_*.go` |
| What an operation *does* | `internal/core` |
| How markdown and components become HTML | `internal/core/render.go`, `components.go` |
| A new rich component | `internal/core/render.go` — one `ComponentFunc` in `Registry` |
| How entries are found, parsed, written | `internal/core/store.go`, `entry.go` |
| Hashes, diffstats, git hooks | `internal/core/git.go` |
| Stamping and its side effects | `internal/core/publish.go` |
| The agent gate and receipts | `internal/core/reconcile.go` |
| Claude memory ingest | `internal/core/memory.go` |
| The page — layout, CSS, JS | `internal/viewer/assets/` |
| The live server or static build | `internal/viewer/serve.go`, `viewer.go` |
| The cross-project hub | `internal/viewer/hub.go`, `internal/cli/cmd_hub.go` |
| MCP tools | `internal/mcpserver/server.go` |
| The Claude Code skill | `internal/cli/embed/SKILL.md` |

## The seams

### The store

`core.Store` is a katra on disk. It is found by walking up for a `config.yml`
in `katra/` (or the legacy `devlog/`), or taken from `$KATRA_DIR`. Everything
else takes a `*Store`.

Nodes — entries, tasks, epics, decisions, articles — are one type, `Entry`,
with a `type` discriminator. An absent type means `entry`, which is what makes
pre-node-model files still parse. `ListNodes` is the single read path.

### Rendering

Markdown goes through goldmark with GFM, Typographer, and two custom
extensions: a wikilink parser/renderer, and a component renderer that maps a
fenced language to a `ComponentFunc` in `Registry`.

Two properties the pipeline has to keep:

- **Unknown fences degrade to code blocks.** Never error on a fence you do not
  recognise. This is what lets an entry written against a newer katra render in
  an older one.
- **Output is self-contained.** A component that fetched anything at render
  time or runtime would break `katra build`, which must produce a directory
  that works offline.

A `Renderer` is bound to a wikilink resolver so `[[links]]` to unknown nodes get
a `dl-wikilink-missing` class. Build one per data build and reuse it — the
resolver needs the whole node set, so it cannot be per-entry.

### Git

`internal/core/git.go` shells out to `git`. Two things there are subtler than
they look:

- **`RepoRoot` and the store are not the same directory.** The store is usually
  a subdirectory, so every git invocation has to be explicit about which
  directory it runs in. `s.git` runs in the store; `gitRoot` runs at the repo
  root so repo-relative pathspecs resolve. Mixing them up produces paths one
  level off, which is exactly how the hook installer once leaked a directory.
- **The hook path is not `.git/hooks`.** `core.hooksPath` moves it — husky
  points it at `.husky/_`, which husky *regenerates*. `hookPath()` resolves
  through `rev-parse --git-path hooks` and then redirects husky's generated
  directory to the tracked one beside it.

### Publish

`publish.go` is the single operation behind both `katra stamp` and the
post-commit hook: attach the hash and diffstat, apply any `closes:` to the
linked tasks, and roll the parent epic up. It is idempotent, because the hook
can retry.

### Reconcile

`reconcile.go` implements the agent gate. The unit of work is *this session's
turn* — the paths authored this turn, intersected with their net change against
the working tree. That scoping is what keeps a conversational turn, an
edit-then-revert, and someone else's pre-existing dirt from all counting as
work. Every predicate fails open.

### The viewer

Plain HTML, CSS and JavaScript in `internal/viewer/assets/`, embedded with
`go:embed`. No bundler, no framework, no build step.

The consequence to remember while developing: **editing an asset and reloading
the page shows you the old one.** Livereload reloads the page, not the binary.
Rebuild.

`serve` and `build` render the same `data.json`; the hub serves many stores from
one process and re-reads the registry on a ticker, so its project set is behind
an `RWMutex` and every handler reads through a snapshot.

## Dependencies

Four direct, and that is deliberate:

| Dependency | For |
| --- | --- |
| `spf13/cobra` | The command tree. |
| `yuin/goldmark` | Markdown, with the GFM and Typographer extensions. |
| `mark3labs/mcp-go` | The MCP server. |
| `gopkg.in/yaml.v3` | Frontmatter, config, and component bodies. |

No web framework, no template engine, no CSS toolchain. A new dependency needs
a reason in the pull request.

## Design notes

Longer-form reasoning about particular subsystems, written while they were
built:

- [The hub](design/hub) — why a registry, and what the cross-project views are for.
- [The wiki model](design/wiki-model) — nodes, wikilinks, and why tasks live in the log.
- [The spec phase on tasks](design/task-spec-phase) — `specced`, `spec:`, and how the reference resolves.
- [Memory ingest](memory-consume) — the three-stage pipeline and its safety rules.
