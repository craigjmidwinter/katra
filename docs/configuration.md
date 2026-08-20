---
title: Configuration
layout: default
nav_order: 5
description: >-
  Every key in katra/config.yml — title, accent, the two git-hook knobs, and
  the memory-ingest settings — with its default and its failure mode.
---

# Configuration

`katra/config.yml` is small on purpose: presentation, the two git-hook knobs,
and the memory-ingest settings. Everything else about an entry lives in the
entry.

```yaml
title: HellHole Country Club
description: Night golf, in VR, on a course that hates you
accent: "#b5502f"

autoCommit: false
commitPrefix: "katra:"

memory:
  enabled: true
  types: [project]
  sensitiveTerms: [acme-internal, staging-key]
```

The file's presence is what marks a directory as a katra — `FindStore` looks
for `config.yml`, which is why an empty `katra/` is not one.

## Keys

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `title` | string | the repo name | Shown in the viewer and in the hub's project list. |
| `description` | string | `A living, committed chronicle of development.` | Subtitle in the viewer. |
| `accent` | colour | `"#e0533d"` | Viewer accent colour. Also tints this project's card in the hub. |
| `autoCommit` | boolean | `false` | Whether the post-commit hook commits the stamp itself. See below. |
| `commitPrefix` | string | `katra:` | Prefix for the hook's own stamp commits. |
| `memory.enabled` | boolean | `true` | Whether Claude Code memory is ingested at all. |
| `memory.types` | list of strings | `[project]` | Which memory `metadata.type` values are admitted. |
| `memory.sensitiveTerms` | list of strings | none | Extra strings that quarantine a memory on match, case-insensitive, layered on the built-in secret detectors. |

Quote a colour. Unquoted, `#b5502f` is a YAML comment and the key silently
becomes empty.

## `autoCommit`, and why it defaults to false

The post-commit hook stamps the active draft with the commit you just made.
That stamp is itself a change to a tracked file, so something has to decide what
happens to it.

| Value | Behaviour |
| --- | --- |
| `false` (default) | The stamp is left as a working-tree change. You include it in your next commit. |
| `true` | The hook commits the stamp itself, as a second commit, using `commitPrefix`. |

The default is `false` because a hook that creates commits behind you is
surprising the first time, and because it interacts badly with anything watching
your working tree — an interactive rebase, a `git bisect` run, a CI script that
asserts a clean tree. Turn it on once you trust it; it removes the "stamp is
sitting uncommitted" state entirely.

The hook does not stamp its own bookkeeping commits, and does not stamp a commit
that only touched the katra directory. Without that, `autoCommit: true` would
be a loop.

## Memory ingest

katra can read Claude Code's own project memory and fold it into a private
ledger, so the log gets the play-by-play without you re-typing it. The
mechanism is a three-stage pipeline — ingest, author, publish — and the config
only governs the first stage.

- **`enabled: false`** turns the whole thing off. Nothing is read, and no ledger
  is written.
- **`types`** is the allowlist of memory `metadata.type` values. The default is
  `[project]` — the type that describes ongoing work — deliberately excluding
  `user` (who you are) and `feedback` (how you like to be worked with), because
  neither belongs in a log you commit.
- **`sensitiveTerms`** are quarantine triggers, layered on top of built-in
  detectors for things shaped like secrets. A memory generation that matches is
  held in the ledger rather than offered for ingest, and `katra memory status`
  reports it as quarantined.

The ledger lives in `katra/.state/`. See [the design note](memory-consume) for
the full pipeline.

## What is not configurable

Some absences are deliberate:

- **No theme system.** The viewer is one design. `accent` is the knob, and the
  static build is a directory you can restyle yourself if you must.
- **No entry template.** `katra new` writes frontmatter and an empty body. A
  template would produce entries shaped like the template rather than like the
  work.
- **No per-entry output settings.** An entry renders the same everywhere it
  renders — live, static, and in the hub — because a component that only worked
  in one of them would be a trap.

## The state directory

`katra/.state/` holds the agent-session records, the memory ledger and the
reconciliation receipts. It is machine state, not log content: deleting it
loses in-flight reconciliation bookkeeping, not any entry.

It is excluded from git by a `katra/.gitignore` that the memory layer writes the
first time the ledger is created — **not** by `katra init`. So a katra that has
never run an agent session has no `.gitignore` and no `.state/`, which is
correct but does mean the ignore rule appears later than you might expect. If
you are committing for the first time after enabling the hooks, check that
`.state/` is not staged.
