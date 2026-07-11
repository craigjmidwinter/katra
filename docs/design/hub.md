# Design sketch: a global devlog hub (`devlogd`)

**Status:** sketch (2026-07-10) — spit-balling, not yet a committed plan
**Type:** article (design note)
**Related:** [[wiki-model]] — the node model this hub aggregates across projects.

---

## The problem

To use devlog across many repos today you run `devlog serve` **once per
project** and juggle the resulting ports/tabs. With the tool spreading to ~40
repos under `~/workspace`, that juggling is the dominant operational tax. There
is no single place to see "everything."

## The idea

One long-running service on a **stable URL** (e.g. `localhost:4200`, started by
launchd on login) that discovers and serves **every** project's devlog. Bookmark
it once. Each project's log is a sub-view; the existing per-repo viewer becomes a
tenant.

```
~/.config/devlog/registry.yml          # project roots (+ optional scan roots)
        │
   devlogd  ──serves──►  localhost:4200/             project index (cards)
        │                              /p/skyhawk/    skyhawk's existing viewer
   watches each devlog/ (fsnotify)     /p/gta-vr/     …live-reloads on write
```

This is cheaper than it looks: devlog already renders `data.json` + static HTML
and already has a file-watch + live-reload loop. The hub is that machinery made
**multi-tenant** — watch N `devlog/` dirs, reload the affected project's view.

## Why it's worth building (the real prize)

If it were only about ports it'd be a nice-to-have. Combined with the five-node
[[wiki-model]], the hub stops being "N logs side by side" and becomes a
**cross-project dashboard**:

- one **board** of every `status: doing` task across all projects
- one **roadmap** rolled up by `horizon` across repos
- global **search / backlinks**, and eventually **cross-repo `[[wikilinks]]`**
  — already done by hand today (Pod-Tool's synergy roadmap points at caststudio
  by relative path)

The hub is where the node model pays off at the *portfolio* level.

## Design decisions to pin

1. **Discovery — layered.** `devlog init` auto-appends the repo to the global
   registry, *plus* optional scan roots (`~/workspace/*/devlog/`) so existing
   logs are found with zero registration. (Beats pure-explicit = tedious, or
   pure-scan = surprising.)
2. **Two modes, one renderer.** `devlogd` (live daemon, watches — the dev-loop
   tool) and `devlog build --all` (one static aggregate site — the publish/share
   artifact).
3. **A third binary.** Ships alongside `devlog` + `devlog-mcp` as **`devlogd`**
   (or `devlog hub`). Keeps the core CLI simple and offline-first; `devlog serve`
   stays for single-repo / LAN / offline use.
4. **Read-only first.** MVP serves; writing stays in the CLI/MCP per repo.
   Editing through the hub UI is a tempting but much larger later step.

## New engineering (everything else is the current viewer ×N)

- Multi-tenant **watch + reload** across many `devlog/` dirs.
- Per-project **media path** isolation and **slug namespacing** (slugs are only
  unique within a repo; the hub must namespace by project).
- The **registry** format + lifecycle (launchd plist, port/host config).
- A top-level **project index** view (cards: title, accent, last-updated, open
  draft/`doing` counts).

## In the target model

This whole note is an **epic** (`horizon: later`) once the model exists, with
tasks roughly: *registry format & auto-register* · *devlogd multi-tenant
watch/serve* · *project index view* · *cross-project board & roadmap* · *static
`build --all`* · *launchd install (`devlogd install`)*.

## Open questions

- **Cross-repo links & identity.** Do `[[wikilinks]]` stay repo-local, or can
  they cross projects (`[[caststudio:kb-two-tier-roadmap]]`)? The synergy
  roadmaps want this; it's scope-creepy. Defer, but design slugs to allow it.
- ~~**Auth / exposure.**~~ **RESOLVED → network-reachable (LAN).** The author
  reads the dashboard from a phone, so localhost-only is out; the daemon binds to
  the LAN like `devlog serve` does today. *Open sub-question:* a cross-project
  dashboard is more sensitive than one repo's log, so LAN exposure likely wants
  optional auth / a bind-address setting — deferred, but don't ship it wide-open
  by default.
- **Performance** at ~40 repos: eager-render all on start vs lazy-render per
  project on first view.
