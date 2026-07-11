# Design: the wiki-shaped model — tasks, epics, decisions, articles

**Status:** proposed (2026-07-10)
**Type:** article (design doc — the first one the model describes)
**Supersedes:** nothing yet; this is the founding spec for katra's evolution
from a dev *log* into a git-committed, markdown-native **project wiki**.

---

## 1. Why this exists

katra today does one thing well: it chronicles the **past** — one markdown
entry per chunk of work, stamped to the commit that produced it. But across a
month of real projects, a second, complementary habit kept appearing on its
own, in five mutually-incompatible shapes:

| Repo | Shape of the forward ledger | Status encoded as |
|---|---|---|
| Pod-Tool | one `.md` per task in `docs/future/` ↔ `docs/shipped/` | folder location + inline `Status:` line |
| NightGolf | numbered `docs/tasks/NN-slug.md` + README index | free-text `**Status:**` field |
| fitness | root `ROADMAP.md` + `BACKLOG.md`, phased | `## ✅ Phase 2 (SHIPPED)` emoji headings |
| holo-top | dated `docs/ROADMAP-2026-07.md` + milestone table | in-prose "NEXT / deferred" + gate table |
| chesscast | 4-tier ROADMAP → SPRINT → TASKS_* → .taskqueue | `[ ]/[x]` checkboxes + P0/P1/P2 tiers |
| caststudio | per-feature `<feature>-roadmap.md`, phased | checkboxes + effort tags + ASCII deps |
| gta-vr / skyhawk | prose `STATUS.md` / `backlog.md` **+ a katra** | CAPS tags, `~~strike~~`, `[NEXT]` |
| cc-cockpit | `.taskqueue/pipeline.md` stage-machine | queue enum: planning→…→done |

Two repos (**gta-vr**, **skyhawk**) had already paired a forward ledger with a
katra and coupled them **by hand** through the commit hash. skyhawk even wrote
the charter verbatim in its backlog footer:

> *"keep this file in sync with the session task list … the katra carries the
> history, this file carries the future."*

That sentence is the whole design. There are two genres — a **backward,
immutable record** and a **forward, mutable ledger** — and they want to be one
tool so the hand-sync between them can be mechanized.

## 2. The shape: a wiki with a spine

Every node is **one markdown file**. Edges come in two flavours:

- **Structured edges** live in frontmatter (`epic:`, `entry:`, `supersedes:`).
  They form the *spine* and drive generated views — the board, the roadmap, the
  epic rollup.
- **Freeform edges** are `[[wikilinks]]` in the body. They form the *wiki* and
  drive backlinks ("what links here").

The principle that keeps it navigable: **a wiki with a spine.** Pure freeform
linking becomes an unnavigable blob; the typed nodes + a handful of structured
edges are the skeleton, wikilinks are the connective tissue on top.

## 3. The node taxonomy

Folder = type. Frontmatter `status` = position in lifecycle. Five types:

```
katra/
  config.yml
  media/
  entries/     # the past — frozen, stamped to a commit  (exists today)
  tasks/       # the future — one .md per unit of intended work
  epics/       # the future — groups tasks under a goal/theme
  decisions/   # frozen-at-creation, evergreen, superseded in chains (ADRs)
  articles/    # timeless reference — owned by no chunk of work
```

They are the same file at different points on **one time axis**:

```
   future                 present                    past
  ┌──────────────────────────────────────────────────────────┐
  │  roadmap(view) → epic → task ───────────────────► entry   │   the spine
  │                                    decision ◄────┘         │
  └──────────────────────────────────────────────────────────┘
        article   (floats free — referenced by any, owned by none)
```

### 3.1 Entry (exists today) — *what I did and what happened*

Immutable narrative of a session, stamped to a commit. Unchanged.

```yaml
---
title: WS-A driving overhaul
date: "2026-07-09"
time: "14:02:11"
hash: 9807f98          # absent while a draft
stat: {f: 12, a: 340, d: 50}
tags: [physics]
---
```

A **draft** is an entry with no hash. This is the model for every other
"in-flight" state below.

### 3.2 Task — *a unit of intended work*

```yaml
---
title: Fade-assisted removal for welded fillers
type: task
status: todo            # todo | doing | done | cut
effort: M               # S | M | L   (the one field ~every repo already uses)
epic: filler-recall     # ← structured edge: belongs-to an epic (slug)
entry:                  # ← set on completion: the entry that recorded it
tags: [filler]
---
## Problem
## Approach
## Acceptance criteria
- [ ] …
```

- `status: doing` is the exact analog of a katra **draft** — work started, no
  record yet.
- `cut` is a first-class terminal state (not deletion) so the *reasons* survive
  — every repo kept a "deferred / cut with reasons" graveyard.
- The body reuses the `Problem / Approach / Acceptance` skeleton already written
  by hand in Pod-Tool and NightGolf.

### 3.3 Epic — *a goal that groups tasks*

```yaml
---
title: Filler recall — stop failing closed on welded fillers
type: epic
status: active          # planned | active | done | cut  (may be computed; see §5)
horizon: now            # now | next | later   ← drives the roadmap view
tags: [filler]
---
## Vision / why this matters
## Success looks like
```

The recurring "workstream" (chesscast WS1–6, holo-top W1–7) and "phase"
(fitness, caststudio) constructs are all this one thing.

### 3.4 Decision — *what we chose and why, until when* (an ADR)

The type that earns its keep through two properties entries lack: a
**supersession chain** and lookup **by question, not by date**.

```yaml
---
title: Signed speed over a velocity vector for the car
type: decision
date: "2026-07-09"
status: accepted        # proposed | accepted | superseded | deprecated
supersedes: []          # slugs; mirrored by superseded-by
entry: 2026-07-09-ws-a-driving-overhaul   # ← the entry where it happened
tags: [physics, vehicles]
---
## Context
## Decision
## Alternatives considered
## Consequences
```

Relationship to entries mirrors task↔entry: you narrate the work in the entry,
and **promote** a load-bearing choice to a decision that links back via
`entry:`. Cheap, optional, no upfront "which file?" friction — you extract the
decision when you realize someone will later ask "why is it this way?"

### 3.5 Article — *how a thing works*

Evergreen reference — external resources, internal constructs, glossaries.
Owned by no chunk of work; off the time axis.

```yaml
---
title: ffmpeg filter graphs, the parts we use
type: article
tags: [reference, ffmpeg]
---
```

### 3.6 The discriminator (so the five stay distinct)

- **Entry** → *what I did and what happened* (narrative, tied to commits)
- **Decision** → *what we chose and why, until when* (one durable, supersede-able choice)
- **Article** → *how a thing works* (reference; no decision, no dated work)
- **Task / Epic** → *what we intend to do* (the future spine)

An entry may *contain* decisions inline; you promote one to a decision node
only when it's load-bearing enough to be asked about later.

## 4. Edges

| Edge | Where | Meaning | Powers |
|---|---|---|---|
| `epic:` on a task | frontmatter | belongs-to | epic rollup, board grouping |
| `entry:` on a task | frontmatter | recorded-by (set on stamp) | past↔future bridge |
| `entry:` on a decision | frontmatter | occasioned-by | trace a choice to its session |
| `supersedes:` / `superseded-by:` | frontmatter | decision chain | "is this still true?" |
| `horizon:` on an epic/task | frontmatter | when | the roadmap view |
| `[[wikilink]]` | body | see-also / references | backlinks panel |

Structured edges are frontmatter so views can compute them; everything else is
a wikilink with a backlink.

## 5. Generated views (not files)

The roadmap's recurring failure across the sweep was **drift** — a hand-written
`ROADMAP.md` that fell out of sync with the task files. So the roadmap, board,
and rollups are **rendered from the nodes, never a separate copy**:

- **Board** — tasks grouped by `status` (todo / doing / done), and/or by epic.
  This *is* past/present/future, rendered.
- **Roadmap** — epics (and loose tasks) grouped by `horizon: now | next | later`,
  ordered by dependency. Cannot drift; it's a view. Narrative "why this order"
  prose, if wanted, is just an **article** the view links to.
- **Epic rollup** — an epic page shows its child tasks with a computed status
  (see open question below).
- **Backlinks** — every node shows "what links here." This is the payoff that
  makes it feel like a wiki rather than a pile of files.

The viewer already renders a page per entry; the delta is: resolve wikilinks,
add a backlinks panel, and add the board / roadmap groupings.

## 6. The loop this mechanizes

Today, in gta-vr and skyhawk, this is done by hand:

```
epic (future) → task (todo)
   → start work        task: doing            (≙ a katra draft opens)
   → write the entry   entry drafted
   → commit + stamp    entry frozen to hash
        └─ stamp also: task → done, task.entry ← <entry-slug>
   → promote choices   decision(s) created, decision.entry ← <entry-slug>
   → roadmap/board reflect the new state automatically (they're views)
```

`stamp` growing a "close the linked task(s)" step is the one new piece of
automation; everything else falls out of the shared frontmatter.

## 7. Build order (don't boil the ocean)

1. **MVP** — the five node types (folder + `type`/`status` frontmatter),
   wikilink resolution, backlinks panel, board grouped by `status` / `horizon`.
   Delivers past/present/future immediately.
2. **Next** — epic rollup + `stamp` closes the linked task.
3. **Later / optional** — an actual graph visualization. Demos well; you'll
   navigate by backlinks + board ~95% of the time. Do not gate the design on it.

Serving this across many repos motivates a **global hub daemon** — see the
companion sketch [[hub]] (`docs/design/hub.md`). That's where the node model pays
off at the portfolio level (a cross-project board / roadmap / backlinks) rather
than per-repo.

## 8. Open questions

1. ~~**Naming.**~~ **RESOLVED → "Katra"** (see `docs/decisions/0001-name-the-tool-katra.md`).
   The five node types map onto a traditional katra's sections. Rename
   execution is deferred to its own task with a `katra/`→`katra/`
   compatibility shim; the name itself is settled.
2. **Epic status: computed or hand-set?** Rolling up from child tasks is clean,
   but an epic is sometimes "done enough" with tasks still open. Proposal:
   compute a *suggested* status, allow an explicit override field.
3. **Wikilink identity.** Slug-based `[[slug]]` (Obsidian-style, matches the
   `[[…]]` already used in fitness). Confirm slugs are the stable node ID and
   are unique across types, or namespace them (`[[task:foo]]`).
4. **CLI surface.** Likely `katra task new|start`, `katra epic new`,
   `katra decide "…"`, `katra article new` — plus `stamp` learning to close
   tasks. Exact verbs TBD.
5. **MCP parity.** The `katra-mcp` server needs the same new operations so
   agents can drive the full model, not just entries.

## 9. What we are *not* doing

- Not adopting cc-cockpit's `.taskqueue` stage-machine into the core model. That
  is an *execution engine* (orchestrator queue), a genuinely different genre
  from a human ledger. It can stay a separate concern that *feeds* tasks in.
- Not encoding status via folder moves (Pod-Tool's `future/`↔`shipped/`). A
  single `status` frontmatter field avoids the git-move noise and lets one file
  carry its whole history.
- Not hand-authoring the roadmap as a file (§5).
