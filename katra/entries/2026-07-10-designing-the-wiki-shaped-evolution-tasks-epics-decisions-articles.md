---
title: 'Designing the wiki-shaped evolution: tasks, epics, decisions, articles'
date: "2026-07-10"
time: "21:56:06"
tags:
    - design
    - architecture
    - dogfood
hashes:
    - fd14581
    - 7b42188
    - 9f2f6cf
    - 0dd49ee
stat:
    f: 73
    a: 3218
    d: 239
---

devlog chronicles the **past**. Today, sitting down to plan this evolution, the
tool's own repo didn't even have a `devlog/` dir — the classic cobbler's-children
gap. So the first act was `devlog init` here, and this entry is the record of the
design session that follows. We're following our own practice.

## The itch

I've been keeping past/present/future task notes in `.md` files across every
project for a month — and doing it five incompatible ways. A sweep of the last
month's active repos turned up a forward-looking ledger in **11 of 12** of them:
Pod-Tool's `future/`↔`shipped/` folders, NightGolf's numbered `docs/tasks/NN-*.md`,
fitness's phased `ROADMAP.md`, holo-top's dated roadmap + milestone table,
chesscast's four-tier ROADMAP→SPRINT→TASKS→.taskqueue, caststudio's per-feature
roadmaps, gta-vr/skyhawk's prose `STATUS`/`backlog` beside a devlog, and
cc-cockpit's `.taskqueue` stage-machine. The inconsistency lived entirely in two
dimensions: *where the file lives* and *how status is encoded*.

## The thing I already knew but hadn't named

Two repos — **gta-vr** and **skyhawk** — had independently paired a forward
ledger with a devlog, coupled *by hand* through the commit hash. skyhawk wrote
the charter in its backlog footer without prompting:

```note
"keep this file in sync with the session task list … the devlog carries the
history, this file carries the future."
```

That one sentence is the whole design. Two genres — a backward immutable record
and a forward mutable ledger — that want to be one tool so the hand-sync can be
mechanized.

## Where we landed

devlog evolves from a *log* into a git-committed, markdown-native **project
wiki** — "a wiki with a spine." One `.md` per node, five node types on a single
time axis:

- **entry** — the past, frozen, stamped to a commit *(exists)*
- **task** / **epic** — the future spine (`status`, `effort`, `horizon`)
- **decision** — an ADR: frozen at creation but evergreen and superseded in
  chains; looked up *by question, not by date*
- **article** — timeless reference, owned by no chunk of work

Structured edges (`epic:`, `entry:`, `supersedes:`) form the spine and drive
*generated* views (board, roadmap-by-horizon, epic rollup); freeform
`[[wikilinks]]` form the wiki and drive backlinks.

## Decisions made this session

- **One `.md` per task**, not a single checkbox ledger — matches devlog's
  "no giant file to corrupt," and the board view reassembles them.
- **Status as a frontmatter field, not folder location** — avoids Pod-Tool's
  `future/`↔`shipped/` git-move noise; one file carries its whole history.
- **`decision` is its own type**, reversing an earlier call to fold it into
  entries. Technical-decision-with-rationale earns a type because it supersedes
  and is queried by "why is it this way?" — an access pattern entries don't have.
- **Roadmap is a view, not a file.** The recurring failure across every repo was
  a hand-written `ROADMAP.md` drifting from the task files. A view can't drift.
- **`.taskqueue` stays out of the core model** — it's an execution engine, a
  different genre from a human ledger.

Full spec written up as the tool's first *article*:
`docs/design/wiki-model.md`. Open questions parked there: the tool's **name**
(is "devlog" still right when the log is 1 of 5 things?), computed vs hand-set
epic status, wikilink identity, the CLI surface, and MCP parity.

## An operational requirement surfaced too

Spit-balling the ergonomics raised the next problem: using devlog across ~40
repos means running `devlog serve` per project and juggling ports. The emerging
answer is a **global hub daemon** (`devlogd`) on one stable URL that discovers
and serves every project's log — and, crucially, once the five-node model exists,
becomes a *cross-project dashboard* (one board of everything `doing`, a portfolio
roadmap, global backlinks). Sketched separately in `docs/design/hub.md`; in the
target model it's a `horizon: later` epic.

## The tool has a name: Almanac

Settled the name — **Almanac**. It won because a historical almanac *is* the five
node types bound together: a diary (entries), predictions & calendars (tasks /
epics / roadmap), and reference tables (articles / decisions). It describes the
whole, not just the log. Rejected: keeping `devlog` (undersells it), Cairn (great
brand, less descriptive), Ledger (naming collision). Recorded as the project's
first ADR — `docs/decisions/0001-name-the-tool-almanac.md` — written in the
`decision` node format we just designed, so we're already dogfooding it. Rename
execution is deferred behind a `devlog/`→`almanac/` compatibility shim.

One requirement fell out of the naming chat: the hub gets read **from a phone**,
so the daemon must be **network-reachable (LAN)**, not localhost-only. Resolved
in `docs/design/hub.md` (with a note that a cross-project dashboard on the LAN
should not ship wide-open).

## And then we built it

Same session: a workflow team implemented the MVP (§7 step 1 of the spec) —
`task`/`epic`/`decision`/`article` in per-type dirs, `[[wikilink]]` resolution +
mirrored backlinks, and a board/roadmap view — ~880 lines across the Go core,
CLI, MCP, and viewer, with the existing `entry` flow untouched. Verified
independently: `go build ./...` green, `go test ./...` passing, and an
end-to-end smoke run creating one node of each type and confirming the link graph
in `data.json`. Two follow-ups the team flagged are fixed: `IsDraft()` is now
entry-only (tasks/epics/decisions/articles are never "drafts"), and a
`viewer.BuildData` contract test locks the `data.json` shape.

Then we ate our own dogfood: this repo's store now holds the real roadmap as
nodes — the [[almanac-node-model]] epic (active) with its tasks, the
[[global-hub-almanacd]] epic (later), and the naming call migrated in as the
[[name-the-tool-almanac]] decision. The graph resolves: every task backlinks its
epic, the decision backlinks this very entry via its `entry:` edge. The tool now
plans and records its own development.

## Postscript: Almanac → Katra, and the loop closes

Two things landed after the above. First, the name: an availability check killed
"Almanac" — it collides with an existing *codealmanac* codebase-wiki tool — so the
tool is now **Katra** (Vulcan: a being's preserved memory and knowledge). Recorded
as [[name-the-tool-katra-almanac-was-taken]], which supersedes
[[name-the-tool-almanac]]; the whole rename ships behind a `devlog/`→`katra/`
compat shim so no existing repo needs migrating.

Second, the roadmap this entry filed as nodes is fully burned down — the MVP,
`stamp --closes`, the epic rollup, and the rename all shipped, tested, and
dogfooded. The [[almanac-node-model]] epic is done in all but name.

```note
And now this entry stops being a draft. It's stamped as a chapter of the whole
arc — the future→past loop the model describes, finally run *by the tool*:
[[stamp-closes-the-linked-task]] is real, and this is the last time it's done by
hand.
```
