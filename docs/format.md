---
title: On-disk format
layout: default
nav_order: 6
description: >-
  The contract for what katra writes — the directory layout, every frontmatter
  key on every node type, the wikilink graph, and the compatibility rules a
  consumer can rely on.
---

# On-disk format

**The files katra writes are its public API.** They are markdown in your
repository, readable and editable without the tool, and they outlive it. This
page is the contract.

## Layout

```
katra/
  config.yml        title, accent, hook behaviour, memory settings
  entries/          one .md per post
  tasks/            one .md per task
  epics/            one .md per epic
  decisions/        one .md per decision record
  articles/         longer-form writing not tied to a commit
  media/            images, gifs, video, html embeds
  .state/           machine state — ledger, receipts, sessions (gitignored)
```

A directory is a katra if and only if it contains `config.yml`. The
conventional name is `katra/` at the repo root; `devlog/` is the pre-rename
name and is still discovered, so repositories created before the rename keep
working without migration.

Filenames are `YYYY-MM-DD-slug.md`. The date prefix orders the directory; the
slug is derived from the title. Neither carries identity — the file *is* the
node, and renaming it renames the node.

## Frontmatter

YAML, then the markdown body. Every node type shares one schema; which keys are
meaningful depends on `type`.

```yaml
---
title: Reworked the swing arc
date: "2026-08-03"
tags: [physics, gameplay]
hash: 5ddc0f5
summary: tuned the magnus model
---

The magnus model was fighting the animation, not the physics.
```

`hash` is the only key doing real work above: its presence is what makes this
entry logged rather than a draft. The full key set, by what it applies to:

### Common to every node

| Key | Type | Notes |
| --- | --- | --- |
| `title` | string | Required. |
| `date` | string | `YYYY-MM-DD`. Quoted, because YAML would otherwise parse it as a date. |
| `time` | string | `HH:MM:SS`. Orders nodes created on the same day. |
| `type` | string | `entry` (or absent), `task`, `epic`, `decision`, `article`. |
| `tags` | list | |
| `summary` | string | One line, shown in the index. |

An absent `type` means `entry`. That is the back-compatibility hinge: every
entry written before the node model existed still parses.

### Entries

| Key | Type | Notes |
| --- | --- | --- |
| `hash` | string | The commit this entry describes. **Its presence is what makes the entry not a draft.** |
| `hashes` | list | For a chapter spanning several commits. Use instead of `hash`. |
| `stat` | mapping | `{f: files, a: added, d: deleted}` — the computed diffstat. |
| `cover` | string | Path to a banner image. |
| `featured` | boolean | Lands in the "Deep Dives" zone. |
| `pinned` | boolean | Held at the top of the index. |
| `closes` | list | Task slugs this entry completes. Consumed at stamp time. |
| `advances` | list | Task slugs this entry moves forward (`todo` → `doing`). |

```yaml
---
title: Reworked the swing arc
date: "2026-08-03"
time: "14:22:07"
tags: [physics, gameplay]
hash: 5ddc0f5
stat: {f: 12, a: 340, d: 50}
summary: tuned the magnus model
---
```

### Tasks, epics and decisions

| Key | Applies to | Values |
| --- | --- | --- |
| `status` | task | `todo`, `specced`, `doing`, `done`, `cut` |
| `status` | epic | `planned`, `active`, `done`, `cut` |
| `status` | decision | `proposed`, `accepted`, `superseded`, `deprecated` |
| `spec` | task | Reference to the committed spec artifact. Resolved as a node slug in the katra first, otherwise a path relative to the **repository root** (not the katra directory). Legal in any status; setting it never moves a status backwards. |
| `effort` | task | `S`, `M`, `L` |
| `horizon` | task | `now`, `next`, `later` |
| `epic` | task | Parent epic slug. |
| `entry` | task, decision | The entry slug that recorded or occasioned it. |
| `supersedes` | decision | Slugs this decision replaces. |
| `superseded-by` | decision | The mirror of the above. |
| `author` | any | An **opaque** actor token, stamped at creation from `$KATRA_ACTOR`. See below. |

### `author`

Recorded when a node is created, from the `KATRA_ACTOR` environment variable,
so that work can be attributed to whoever produced it.

**katra never interprets the token.** It does not parse it, validate its shape,
map it to a role, or resolve it to anything — validation would be interpretation,
because a rule about what a token may look like is a rule about what tokens mean.
Whoever reads the store decides that. An environment variable rather than a
lookup, so the field works under any harness, in CI, and on a machine with none
of the infrastructure that issues the tokens.

**Absent is a value.** An unset `KATRA_ACTOR` leaves the key off entirely; it is
never written empty and never defaults to a role. A person running `katra task
new` by hand is not a manager, and a default would flatter whoever forgot to set
the variable. Anything reporting authorship must report its unattributed count
beside it — `katra doctor` does.

Nodes created before this field existed carry no author and cannot be attributed
retroactively. That is a permanent gap in the record, not a bug to fix.

An explicit `author` in frontmatter wins over the environment, so importing or
reconstructing history is not overwritten by whatever happens to be set.

`specced` sits between `todo` and `doing`: a design exists, committed, and the
task points at it, but nothing has been built yet. It is optional — `todo →
doing` is still a legal transition, and `katra epic rollup` treats a
`specced`-only epic the same as `planned`, because a spec is thinking, not
work.

## A draft is an entry with no hash

That is the entire state machine, and the most important thing to know about
the format.

- No `hash` and no `hashes` → a **draft**. It renders in the *In Progress*
  panel.
- Either present → **logged**. It renders in the log, with its diffstat.

There is no `status: draft`, no separate directory, and no scratch file. Adding
a hash is what publishes an entry, which is why stamping is the only publish
step and why nothing can get stranded.

A consumer deciding whether an entry is a draft should test for the absence of
both keys, not for the presence of a status field.

## Wikilinks

`[[slug]]` in any body links to another node. The renderer resolves the slug
against the node set and marks unresolvable links with a
`dl-wikilink-missing` class rather than failing — a link to a node you have not
written yet is a valid thing to write.

Links are the graph. There is no separate index file to keep in sync, and no
link database: the graph is recomputed from the bodies on every render.

## Rich components

A fenced code block whose language is registered renders as a component; see
[Components](components) for the six built in and the keys each takes.

**An unregistered language renders as an ordinary code block.** This is a
compatibility guarantee, not a fallback: an entry using a component your katra
does not have still renders, showing the block's YAML as source. Nothing errors,
and nothing is lost. It is why adding a component is a safe, additive change.

## Media

`katra/media/` holds everything an entry references. Paths in frontmatter and
in component bodies are relative to the katra directory (`media/shot.png`), not
to the entry file.

`katra capture` imports a file, giving it a collision-free name. `katra doctor`
reports references that point at files which are not there — the one integrity
check the format needs, because everything else is just markdown.

## The compatibility rules

For anyone writing a tool that reads or writes a katra, and for anyone changing
katra itself:

- **An unknown frontmatter key is ignored, never an error.** Adding an optional
  key is a safe change. A consumer older than `spec` ignores it and shows the
  task with no reference at all — only the prose written from it is lost.
- **A new status value degrades silently, the same way.** A consumer older
  than `specced` finds it outside every switch it wrote; it must fall into
  whatever default bucket it already has (a `specced` task rendering as
  `todo`, say) rather than erroring or vanishing from a count it used to
  appear in.
- **An unknown fence language degrades to a code block.**
- **An unknown `type` is not an entry.** Consumers filtering for entries should
  match `type` absent or `entry`, rather than excluding the types they know.
- **Renaming or repurposing a key is a breaking change**, and so is making an
  optional key required. Somebody's 2026 entry has to render next year.

The static build's `data.json` is a *rendering*, not the format. It is
regenerated from the markdown on every build, its shape follows the viewer's
needs, and it carries no compatibility promise. Read the markdown.

## What is deliberately not in the format

- **No entry ids.** The file path is the identity. This makes `git mv` a rename
  and makes a hand-edited file still valid, at the cost of making a slug change
  break inbound wikilinks — which `katra doctor` will tell you about.
- **No ordering field.** Date plus time plus filename is the order. An explicit
  `order:` would be a second source of truth that drifts.
- **No author field.** Git already knows. A katra is per-repository, and the
  commit it is stamped with carries the author.
