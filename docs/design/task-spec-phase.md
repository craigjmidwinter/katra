---
title: The spec phase on tasks
layout: default
parent: Design notes
nav_order: 4
description: >-
  Adding a specced status and a spec frontmatter key to the task lifecycle -
  the artifact an implementer should read before a task moves to doing.
---

# Design: the spec phase on tasks

- **Status:** accepted, implemented
- **Date:** 2026-08-20

## Problem

Katra positions itself for spec-driven agentic development: an agent works
best from a committed spec, and the katra is where that spec should live —
next to the tasks it governs and the entries that record how it went. But the
task lifecycle (`todo → doing → done | cut`) has no place to *stand* between
"we should do this" and "someone is doing it". The moment a task has a design
worth implementing from, the model cannot say so, and nothing links the task
to the artifact an implementer should read first.

## Decision

Add one status and one frontmatter key. Nothing else changes shape.

### Lifecycle

```
todo → specced → doing → done | cut
```

`specced` means: *a design exists, committed, and the task points at it.
Implementation has not started.* It is an optional station — `todo → doing`
remains legal; not every task deserves a spec.

### Frontmatter

```yaml
type: task
status: specced
spec: docs/design/task-spec-phase.md   # or a node slug
```

`spec` is a **reference to the spec artifact**, resolved in this order:

1. A node slug in the katra (decision, article, or entry) — same identity rule
   as a `[[wikilink]]`.
2. Otherwise, a path relative to the **repository root** (not the katra
   directory), so `docs/design/foo.md` works and survives the katra directory
   being renamed.

`spec` is legal on a task in any status: a task already `doing` or `done` may
have its spec recorded retroactively, and setting it never moves a status
backwards.

Per the format contract, an unknown frontmatter key is ignored by older
consumers and an unknown status string falls outside their switches — both
degradations are acceptable and must stay silent (a `specced` task in an old
viewer may render in the todo bucket; it must not crash or disappear from
counts it previously appeared in).

## CLI

| Command | Behaviour |
| --- | --- |
| `katra task spec <slug> <ref>` | Set `spec: <ref>`. If status is `todo` or empty → `specced`; a status of `doing`/`done`/`cut` is left alone. |
| `katra task new "Title" --spec <ref>` | Create the task already `specced`. |
| `katra task list --status specced` | The existing generic filter; `specced` becomes a documented value. |
| `katra task start <slug>` | Unchanged (`doing`); legal from `specced`. |

`katra task spec` with a ref that does not resolve (no node slug, no file at
the repo-relative path) **warns but writes** — the spec may be authored in the
same change, and a blocking check belongs in `doctor`, not in the write path.

## Touch points

- `internal/core/entry.go` — add `Spec string` to `Frontmatter`
  (`yaml:"spec,omitempty"`); update the `Status` comment to
  `todo|specced|doing|done|cut`.
- `internal/core/publish.go` — `ReconcileAdvance`: `"" | todo | specced` →
  `doing`.
- `internal/core/store.go` — `EpicRollupStatus`: `specced` counts as **not
  started** (a specced-only epic stays `planned`; a spec is thinking, not
  work). Add a `ResolveSpec(nodes []Entry, repoRoot, ref string)` helper the
  doctor and viewer share.
- `internal/cli/cmd_node.go` — the `task spec` subcommand, `--spec` on
  `task new`, `specced` in the `--status` flag help text.
- `internal/cli/cmd_doctor.go` — new check: a task whose `spec:` ref resolves
  to neither a node nor a file. Report, never fail the exit code harder than
  existing checks do.
- `internal/viewer/hub.go` — `hubBoardHTML` gains a **Specced** column
  between Doing and Todo. A `specced` task must appear there (today's switch
  would drop it silently — that is the regression to guard with a test).
- `internal/viewer/api.go` — `spec` field on task items in the JSON snapshot
  (omit when empty).
- Project viewer (`internal/viewer/assets/app.js` + styles) — show the status
  chip for `specced` and, when `spec` is set, a "spec" link on the task:
  node refs link to the node's page; path refs render as plain code text (the
  viewer cannot serve arbitrary repo files, and must not try).
- `internal/mcpserver/server.go` — mirror `task spec` wherever task lifecycle
  tools exist.
- `internal/cli/embed/SKILL.md` — teach the workflow: write the spec, commit
  it, `katra task spec`, then implement from it.

## Out of scope

- No gate ever *requires* a spec. The commit gate stays about declaring what
  work was for, not about process enforcement.
- No spec templates, no spec node type. A spec is any committed artifact;
  decisions and articles already cover the in-katra cases.
- No epic-level spec field. An epic's body is its own narrative.
