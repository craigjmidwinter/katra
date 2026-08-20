---
title: The fold was on the wrong side, and other things writing the docs found
date: "2026-08-03"
time: "12:13:05"
tags:
    - branding
    - docs
    - release
summary: OSS scaffolding for katra, and the four bugs that only surfaced once someone had to write them down
closes:
    - bring-the-repo-up-to-the-open-source-scaffolding-standard
    - spec-phase-on-tasks-specced-status-spec-artifact-link
advances:
    - spec-phase-on-tasks-specced-status-spec-artifact-link
---

katra has 11 repositories and 237 entries behind it, and had a README, a
LICENSE, and three design notes. Its sibling `mail-muncher` had the full
open-source treatment — release automation, a docs site, a brand kit, a
contributor guide that argues with you. The gap was not that katra was less
finished. It was that nobody had ever had to explain katra to a stranger.

The job was to close that gap. The interesting part is what closing it found.

## The fold was on the wrong side

The mark is a page with its bottom-right corner turned up — an entry is a page,
and it keeps turning over, which is the one idea the tool has.

The first version put the accent on the near side of the crease, filling the
triangle *inside* the fold box. It looked plausible in the source and wrong on
the screen: the accent read as a wedge stuck onto the page rather than as a
corner lifting off it. The silhouette was notched, which made it worse — the
page looked bitten rather than folded.

Inverting it fixed the whole thing. Keep the silhouette a full rectangle, make
the crease a diagonal keyline, and put the accent *beyond* it. Then the accent
is the underside of the folded corner, and the eye reads a fold instantly.

```compare
before: media/wrong-fold.png
after: media/right-fold.png
caption: Accent inside the crease reads as a wedge stuck on; accent beyond it reads as a turned corner
```

The only reason this got caught is that the mark was rasterized and looked at,
three times, rather than reasoned about. The second attempt was still wrong and
also looked fine as code.

There is a related trap one size down. The full mark is a 24-unit grid, and 24
does not divide into 16 — so a 16px favicon resamples the page keyline away
entirely and leaves a grey smudge with no edges. Five ruled lines merge into a
solid block at that size too. Hence two grids: a 16-unit mark with three rules
for anything 32px and below, exact at 1:1 and 2:1, and the full one above that.

![The finished lockup, on the Open Graph card](media/social-preview.png)

## The accent is not a text colour

The palette came straight out of the viewer's stylesheet, on the principle that
the docs and the product should not disagree about what katra looks like. Then
the ratios got measured rather than eyeballed, and `--accent #b5502f` turned out
to fail WCAG AA for body text on its own background — 4.41:1 on paper, against a
4.5:1 floor.

```embed
src: media/contrast.html
height: 480
caption: Measured contrast against each background. The mid accent fails AA on both sides.
```

The part worth remembering is that it fails on *both* sides, and the fix
inverts. On paper the accent has to be darkened to `#8a3d24` to carry text; on
the dark side it has to be lightened to `#e08a5c`. There is no single accent
value that works as a link colour in both schemes, so anything that tries to
share one is wrong in one of them.

`#b5502f` stays exactly where it belongs: the fold of the mark, component
boundaries, large type. Never a paragraph.

## Two bugs that had been sitting there

**`make build` could not have worked.** The obvious Makefile writes the binary
to `./katra` — which is the directory katra keeps its own log in. `go build -o`
will not write over a directory, so the very first command in the contributor
guide would have failed for anyone who ran it. It had never been run, because
everyone building katra was building it with `go build -o ~/go/bin/katra`. The
`.gitignore` had a careful comment about exactly this collision, which is a
nice illustration of knowing a thing without following it through.

**`--version` had been lying since the rename.** It was a package-level
`var version = "0.1.0"` in `internal/cli`, hardcoded, with a matching literal in
`cmd/katra-mcp`. Every binary anyone had ever installed reported `0.1.0`. It is
now stamped at link time through `main.version` in both entrypoints, which is
what the Makefile and the release build need anyway.

## Documentation that tests itself

The lesson from writing the reference is that prose about behaviour rots
silently, so the parts that can be executed are.

`examples/entry.md` uses every one of the six components, and CI drops it into a
scaffolded katra with its media and runs `doctor` and `build`. `doctor` exits
non-zero on a dangling media reference or a body that fails to render, so a
component renamed without updating the example fails the build. The example
ships real assets rather than placeholders — generated PNGs, an actual two-second
h264 clip, a self-contained SVG chart — because a `video` block pointing at a
file that is not there proves nothing.

Each `examples/config/*.yml` is likewise copied in as a real `config.yml` and
rendered, so a stale key fails CI rather than someone's terminal.

Writing the config reference is also how three documented defaults turned out to
be wrong — `accent` is `#e0533d`, `commitPrefix` defaults to `katra:`,
`description` has a default — and how `.state/` turned out to be gitignored by
the memory layer on first use rather than by `katra init`, which is why the older
projects have no `katra/.gitignore` at all.

## The README claimed something nobody had checked

The Status section originally read "used daily across a dozen repositories —
which is the only reason to trust any of it." It was challenged, and it did not
survive being counted.

The real figures: **11 repositories, 237 entries, 44 active days out of 119.**
July was genuinely heavy — 193 entries on 24 of its 31 days — and April through
June produced 27 entries between them. So "lumpy" is the honest word and "daily"
was not.

Worse, the strongest counter-evidence was three days old and in this same
repository. The audit that produced [[hooks-and-hub-stop-drifting-silently-live-registry-tracked-husky-hook]]
found 40 unstamped drafts, three projects that had never stamped an entry at
all, and the agent hooks broken or absent in 9 of the 11. That is not what daily
use looks like, and a README that claimed otherwise while the fix for it sat in
the same working tree was making a promise the log itself disproved.

The Status section now states the countable version and lists the drift audit as
evidence *against* the tool rather than omitting it. The number will age, so it
wants re-checking before a release rather than being left to rot.

### And counting it found a bug

Deriving those figures meant reading the hub registry, which turned out to
contain five entries pointing at `mktemp -d` directories — scratch katras left
behind by the smoke tests run earlier in the same session. They were showing on
the board as real projects.

`katra init` registers every store it creates with the machine-global registry,
which is right for a real project and wrong for a fixture. Nothing distinguishes
the two. `hub list` prunes entries whose store has vanished, and would have
cleaned these up eventually — but a `mktemp -d` directory survives until reboot,
so "eventually" was days away.

`KATRA_REGISTRY` already existed as an override and is now set in the CI job and
documented for local use. The deeper fix — having `init` notice it is
scaffolding into a temp directory, or not registering unless asked — is not
made here.

## What was left out, on purpose

No Dockerfile and no MCP-registry listing, and the README says why rather than
leaving it as an absence. katra operates on your working tree, your git history
and your hooks; containerising it means mounting all three to arrive at a worse
local install. The registry indexes servers by distributable package, so with no
image there is nothing to list. `mail-muncher` has both because it talks to a
network service and genuinely benefits.

```warning
The docs site has not been rendered. Jekyll builds on GitHub Pages, and nothing
here has run `bundle exec jekyll serve` against the `just-the-docs` remote
theme, so the colour scheme, the nav ordering and the callout retint are correct
by construction and unconfirmed by observation. The first push to `main` is the
first real build. The contrast numbers above are measured and the SVGs are
checked well-formed, but the page they sit on is not.
```

## The spec phase lands as a spec

The directive for today's pass: position katra for spec-driven agentic development, and — if the model has no place for a task to stand between "we should" and "someone is" — build one. It didn't. `todo → doing` jumps straight from intention to motion, and nothing links a task to the design an implementer should read first.

So the feature arrives the way it says features should: the design went into [docs/design/task-spec-phase.md](https://github.com/craigjmidwinter/katra/blob/main/docs/design/task-spec-phase.md) *before* any code, and the implementation task points at it. One new status (`specced`), one new frontmatter key (`spec:`), and the deliberate refusals: no gate ever requires a spec, no spec node type, no epic-level spec.

Also written while the implementation agents run: [branding/POSITIONING.md](https://github.com/craigjmidwinter/katra/blob/main/branding/POSITIONING.md) — a self-contained brief (one-liner, pitch, the two claims, voice, asset URLs) so the personal-site session can present katra without re-deriving the identity from the whole repo. The two claims got an explicit priority order for the first time: spec-driven agentic development first, log-as-side-effect second.

The feature closed its own loop: the task that tracked building the spec phase is now the first task to carry one — `spec: docs/design/task-spec-phase.md`, attached with the freshly built `katra task spec`. Status stayed `doing` rather than sliding back to `specced`, which was the rule worth testing: recording a spec retroactively must never un-start work. One integration catch from running two lanes in parallel: the docs lane documented doctor's dangling-spec check as a hard failure while the implementation made it a warning — the implementation had the better reading of the spec's "report" verb, so the docs moved to match, not the code.

The site lane found the kind of defect only reading the theme's compiled source finds: just-the-docs bakes `logo:` into a build-time Sass variable with no `prefers-color-scheme` awareness, so dark-mode readers had been getting the light lockup's ink wordmark on a dark sidebar the whole time — despite a perfectly good `lockup-dark.svg` sitting next to it. One cascade-ordered override in `head_custom.html` and BRAND.md's own usage rule is finally honoured on the docs site itself.

Closing the day: four parallel lanes, all landed. The feature (specced status, spec: ref, task spec command, Specced board column, doctor check, MCP mirror — suite green), the copy pass (spec-driven agentic development now leads the README and threads through cli/format/agents/skill docs), the site polish (dark-lockup fix, nav for the new design page), and the audit's must-fixes (first screenshots the repo has ever had, repo topics + homepage, RELEASING.md, SECURITY.md with the reporting toggle actually enabled). The audit's verdict worth keeping: the prose already met the getvect bar — what was missing was anything to *look at*. Staged for a hardware-key commit; the stamp will close this task.

The fleet standard arrived mid-pass and formalized the bar. Walking its checklist found what the informal pass had missed: no CHANGELOG, screenshots nobody could regenerate, a repo description that wasn't the one-liner — and one collision that forced a real decision. The wordmark was set in Silkscreen, shared with mail-muncher on purpose; the standard forbids fleet projects sharing a display face, and honestly the pixel face was always mail-muncher's register. So the wordmark is now Fraunces 72pt Black — a masthead, not a terminal — outlined as before, mark untouched. The palette stays with a recorded exception: paper and ink aren't decoration here, they're the concept. Also: the store title finally went lowercase, which the name-treatment rule had been quietly violated by since the day it was created.
