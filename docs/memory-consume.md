---
title: Memory ingest
layout: default
parent: Design notes
nav_order: 3
description: >-
  The design of the three-stage memory pipeline.
---

{: .warning }
**Historical design note.** The design of the three-stage memory pipeline. See [Agents](agents#memory-ingest) for the current behaviour and [Configuration](configuration#memory-ingest) for the keys.

# Memory-consume: ingesting agent project-memory into katra

## The problem

The agents doing the work are Claude Code, and Claude Code keeps native
per-project *memory* automatically — typed markdown under
`~/.claude/projects/<repo>/memory/*.md` (`type:project`, `type:reference`,
`type:user`, `type:feedback`). That's where the play-by-play actually lands: what
was tried, what broke, what got decided.

katra, meanwhile, needs the agent to *also* write prose into a draft. In
practice that second write rarely happened. Memory filled up with the real
story while the katra log went stale — exactly the failure mode katra exists to
prevent. Nagging the agent to re-type memory into katra didn't fix it; it just
asked the agent to do the same work twice.

## The reframe

**Consume memory, don't fight it.** Writing to memory is native agent behavior
and it's good behavior — it's context the agent reuses. So katra should treat
memory as an *input*: read what the agent already wrote, and let that feed the
log. katra consumes memory; it does not replace it. The agent keeps journaling
to memory exactly as before.

What the agent still owns in katra is the part memory can't reconstruct: the
assets (screenshots, before/after, galleries, rendered artifacts) and the
reasoning behind decisions.

## Three transitions — keep them separate

The temptation is to collapse "read memory" straight into "publish a log entry."
That's wrong. There are three distinct transitions and each has a different
owner and different risk.

1. **INGEST** — deterministic, no LLM. A plain scan of `type:project` memory
   files into a private *ledger* that tracks which memory files/sections exist,
   their hashes, and their state (new / imported / ignored / quarantined).
   Ingest **creates no entries** and writes nothing to the published log. It is
   pure bookkeeping: "here is new memory the log hasn't accounted for yet."

2. **AUTHOR** — the live agent. At the commit checkpoint the agent is *nudged*:
   "you have N new memory notes since the last stamp; fold what matters into your
   draft." The agent — which still has the working context — decides what's
   worth saying, rewrites it in its own voice, adds the assets and the *why*,
   and leaves out the noise. This is the only step that produces log prose, and
   a human/agent judgment call is exactly what it should be.

3. **PUBLISH** — the existing `katra stamp` + commit. Unchanged. The draft
   becomes a stamped entry with hash + diffstat and drops into the log.

### Why collapsing them is wrong

- **Raw dumps.** Auto-piping memory into an entry would paste unedited,
  half-formed notes into a public-facing log. Memory is a scratchpad; the log is
  a post. Only AUTHOR turns one into the other.
- **Privacy.** Memory is candid and local (see below). Machine-copying it into a
  committed, served artifact leaks by default. Keeping INGEST asset-free and
  ledger-only means nothing reaches the log without a deliberate AUTHOR step.
- **Stamp targeting.** `ActiveDraft` is defined as the *newest unstamped entry* —
  that's what the next `stamp` lands on. If INGEST auto-created a draft entry, it
  would silently become the active draft and **steal the next commit's stamp**
  from the entry the agent was actually writing. Ingest must never create
  entries, precisely to avoid hijacking stamp targeting.

## Privacy

Memory is written to be candid and it lives only on the developer's machine. The
katra log is committed to the repo *and* served by the hub. Two consequences:

- **"Uncommitted" is not "unpublished."** The hub reads the working tree, so a
  draft is visible the moment it's on disk — before any `git commit`. There is no
  safe staging area that's private-by-default.
- **Don't trust a scrub.** Rather than run memory through a redactor and assume
  it's clean, INGEST **quarantines** any file that hits a secret pattern or a
  configured sensitive term — it's flagged in the ledger and withheld from the
  AUTHOR nudge until a human resolves it. Publication stays an explicit
  human/agent action, never an automatic side effect of scanning.

## Commands

Some land in a follow-up phase (noted):

- `katra memory scan [--snapshot]` — INGEST. Scan `type:project` memory into the
  ledger; report new / changed / quarantined files. `--snapshot` copies the
  current memory state so it survives the ephemeral session store.
- `katra memory status` — show the ledger: what's new since the last stamp,
  what's imported, what's ignored, what's quarantined.
- `katra memory ignore <file> --reason "..."` — mark a memory file as
  permanently not-for-log (with a recorded reason).
- `katra memory resolve <file> --imported` — mark a file's content as folded into
  the draft (or otherwise handled), clearing it from the "new" list.
- **SessionEnd snapshot hook** — on session end, snapshot memory so nothing is
  lost when Claude Code rotates its session store.
- **Commit-guard nudge** *(future phase)* — at the commit checkpoint, if the
  ledger shows new memory since the last stamp, remind the agent to AUTHOR before
  it publishes.

## Config

A `memory:` block in `katra/config.yml`:

```yaml
memory:
  enabled: true
  types: [project]          # which memory types to ingest; project only, by default
  sensitiveTerms:           # extra terms that force quarantine on match
    - internal-codename
    - customer-name
```

- `enabled` — master switch for the whole feature.
- `types` — which typed memory files to scan. Default is `project` only;
  `reference`/`user`/`feedback` are excluded unless opted in.
- `sensitiveTerms` — project-specific strings that, in addition to the built-in
  secret patterns, force a file into quarantine.
