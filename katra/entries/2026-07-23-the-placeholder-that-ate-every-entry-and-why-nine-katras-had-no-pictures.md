---
title: The placeholder that ate every entry — and why nine katras had no pictures
date: "2026-07-23"
time: "17:49:21"
tags:
    - katra
    - skill
    - fix
hash: 761027b
stat:
    f: 6
    a: 408
    d: 15
closes:
    - hub-json-api-for-native-clients-api-hub-json
    - hooks-and-hub-stop-drifting-silently-live-registry-tracked-husky-hook
---

Craig opened with two complaints about the logs in `gta-vr` and everywhere else:
they read as flat technical docs instead of posts, and **they all say "Start
writing here."** Those turned out to be a bug and a habit, in that order.

## The placeholder was welded into 75 entries

`katra new` seeds a draft with `Start writing here.` so a fresh file isn't an
empty void. `AppendBody` then appended *underneath* it — forever. Every entry
ever written carried the prompt above its own first paragraph.

The reason nobody caught it: the viewer hides it. `app.js` strips the prefix at
render time and treats a placeholder-only body as "no body yet". So the log
looked fine on screen while every markdown file on disk was wrong — 75 of them
across 7 repos.

The fix is one function, because all seven append paths (CLI `append`,
`capture`, `compare`, and the three MCP handlers) funnel through `AppendBody`:
a body that is still just the placeholder now counts as empty, so the first
append *replaces* the prompt instead of writing below it. The string itself was
duplicated in two writers, so it became `core.DraftPlaceholder` alongside an
`IsDraftPlaceholder()` that tolerates the whitespace variants a `Save()`
round-trip produces.

```note
The test asserts the *file on disk* is clean after two appends, not just the
in-memory entry. An in-memory-only assertion would have passed against the
buggy code path the same way the viewer did.
```

The 75-entry backfill was deliberately timid: strip the placeholder only when
it's the first non-empty line of the body, never mid-body (that's quoted
content — this very entry contains the string several times), and never when
it's the *whole* body. That last rule matters: one gta-vr draft is genuinely
empty, and the viewer needs the placeholder there to keep showing it as
unwritten.

## The pictures problem was two problems

The obvious read is "the components are too hard to use." The data says
otherwise.

```embed
src: media/visual-coverage.html
height: 400
caption: Share of entries carrying at least one visual. skyhawk had the habit; nowhere else did.
```

`skyhawk` puts a visual in 82% of its entries using exactly the same CLI
everyone else has. The tooling isn't the blocker; the habit just never formed
elsewhere. Five katras have never shipped a single picture.

The second problem was **skill drift**. There were two copies of `SKILL.md` —
`skill/SKILL.md` and `internal/cli/embed/SKILL.md`, the one `katra setup`
actually installs — and they had diverged. Six repos were running a stale
3,253-byte skill, three had the 3,974-byte one. Guidance I'd edited months ago
never reached most projects. `skill/SKILL.md` is now a symlink to the embedded
copy, so it can't drift again.

## What the skill says now

The old skill listed the components as an API and hoped. The new one leads with
"you are writing a post, not a report" and makes one rule hard: **every entry
ships at least one visual**, with a preference ladder — screenshot the UI, else
render the structured output, else chart the numbers, else diagram the
architecture. A map generator's post should contain a picture of a map.

The anti-patterns are quoted from the real logs rather than invented, because
the specific failure mode here is an *agent* writing for the next agent:

- "Integrator notes: wire PlazaStage after DiagonalAvenueStage" — coordination
  aimed at a teammate, not a reader
- closing on tooling receipts (`ValidateScript: 0 diagnostics`)
- invariant checklists standing in for a body

```note
Charts stay hand-authored HTML rather than a new `chart` component — Craig's
call, and the right one. `katra capture` already recognizes `.html` and emits
an `embed` block automatically, so the whole feature was a documentation gap,
not a missing renderer. The skill now carries the constraints that actually
bite: no external requests inside the sandboxed iframe, style both themes,
label the units. The chart above was built that way as the first test of it.
```

## Rollout

The gap between "fixed" and "in effect" is wider than it looks here. The viewer
assets are embedded in the binary and the skill is embedded too, so a source
edit changes nothing until `go build -o ~/go/bin/katra` — and the installed
per-repo skills are *copies*, so all nine needed the new `SKILL.md` pushed to
them before any of this reaches a session. Hub daemon restarted with
`launchctl kickstart -k gui/501/com.katra.hub` to pick up the rebuild.

```warning
Existing entries are still flat. Every fix here is forward-looking: the 75
backfilled files only had a line removed, not their prose reconsidered.
gta-vr's MapGen posts still describe a map generator without showing a map.
```
