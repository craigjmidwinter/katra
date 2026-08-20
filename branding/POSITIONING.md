# katra — positioning summary

Self-contained brief for anyone presenting katra elsewhere (the personal
site, a talk, a directory listing). Everything needed to describe the project
accurately is on this page; visual identity details are in
[BRAND.md](BRAND.md).

## The name

**katra** — always lowercase in the wordmark and in prose. The word is the
Vulcan term for the living essence that survives the body: the katra is the
part of the work that survives the diff.

## One-liner

> A committed dev log you write as you build — and the memory that makes
> spec-driven agentic development work.

Shorter, for a card or a title attribute:

> The dev log that lives in your repo and publishes itself when you commit.

## Elevator pitch (~80 words)

katra is a dev log that lives inside the repository as markdown, accumulates
screenshots, comparisons and reasoning *while* you work, and is published by
the commit you were going to make anyway — a draft is just an entry with no
commit hash. Around the log sits a light node model: tasks, epics, decisions,
and committed specs. Agents implement from the spec, the gate makes them
declare what their work was for, and the record survives the session that
produced it.

## The two claims (in priority order)

1. **Spec-driven agentic development.** A task can be `specced`: linked to a
   committed design artifact before implementation starts. An agent — this
   session or one six weeks from now — picks up the task, reads the spec, and
   implements from it instead of re-deriving intent from a cold repo. The
   spec, the task, and the entries recording how it actually went are all
   markdown in the same repository, so context survives across sessions,
   agents, and time.

2. **The log is a side effect of working, not a job after it.** Draft created
   when you start, media captured as you go, stamped by the commit itself.
   Nothing to remember, no "promote" step to skip, and the dead ends get
   recorded — the half a squashed history always loses.

## Honest boundaries (say these, they are part of the brand)

- Local-first, no hosted service, no account, no sync. Markdown in the repo
  is the product; it outlives the tool.
- Pre-1.0, built by one person, dogfooded across 11 repositories and 237
  entries over four months. Rough edges exist and are documented.
- It will not write your entry from the diff — an auto-generated entry is
  exactly the paraphrase it exists to avoid.

## Audience

Developers who work with coding agents (Claude Code first-class: skill,
seven hooks, MCP server) and solo/small-team builders who want a durable
"why", not another project-management tool.

## Voice

Specific and measured over promotional. Claims are checkable numbers or
they are cut. Tradeoffs and refusals are named in the open ("Deliberately
out of scope" is a load-bearing section, not an apology).

## Visual identity (summary — BRAND.md is authoritative)

- **Mark:** a page with its bottom-right corner turned up; the fold carries
  the accent. Pixel-grid SVG, crisp edges, no dark variant needed.
- **Wordmark:** Fraunces 72pt Black, outlined SVG only, lowercase "katra".
- **Palette (Field Notebook):** paper `#f4efe4` / dark `#1c1a17`, ink
  `#2c2823` / `#ece5d8`, accent `#b5502f` (UI accent only — fails AA at body
  size; links darken to `#8a3d24` on light, lighten to `#e08a5c` on dark).
- **Assets:** `docs/assets/brand/` — `lockup.svg` / `lockup-dark.svg` for
  headers, `mark.svg` (≥48px) / `mark-small.svg` (≤32px), `social-preview.png`
  (1280×640) for link cards. Raster fallbacks alongside. Raw URLs:
  `https://raw.githubusercontent.com/craigjmidwinter/katra/main/docs/assets/brand/<file>`.
- Don't recolour the mark, don't re-type the wordmark, don't put the mark on
  a mid-tone background.

## Links

- Repo: <https://github.com/craigjmidwinter/katra>
- Docs: <https://craigjmidwinter.github.io/katra/>
- Install: `brew install craigjmidwinter/tap/katra`
- License: MIT
