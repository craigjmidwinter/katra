# Design: capture at the clearing moment

Status: accepted, 2026-08-27.
Epic: `capture-at-the-clearing-moment`.
Reported by katra's steward, who measured the friction by doing it deliberately
rather than recalling it.

## The evidence, which indicts this repo too

The steward's chronicle had no entries for 2026-08-26 or 2026-08-27 — the two
heaviest days on its record. Two context clears in that window wrote their
durable state into ad-hoc markdown instead, one of them into *another project's
tree*, because that was easier than using the tool built for durable state.

katra's own log is worse. `katra list` here stops at 2026-08-25. **There is no
entry for the day katra shipped v0.1.0.** The tool that exists to chronicle
development has no record of its own first release, written by the sessions that
did it, in the repo that is supposed to dogfood it.

Two independent maintainers routed around it in the same week. That is a product
finding, not a discipline problem.

## Speed is not the problem

Worth stating because it is the plausible answer and it is wrong: draft creation
is instant. Optimising it would have been effort spent on the thing that was
already fine.

## What is actually wrong

**The shape of the entry motion.** `katra append` takes `--file` with `-` for
stdin. `katra new` takes `--body` as a string and has nothing equivalent. So the
natural mid-task motion — dump what I know before I lose it — is two commands
and a temp file when the store already has the right shape one command over.

**Nothing speaks at the moment of need.** `katra check` gates commits. But
knowledge is destroyed *between* commits, when a session runs out of room. A
tool that only speaks at commit time cannot help with a failure that happens
between them.

**The record is narrative-shaped; a clearing session must save status.** An
entry says what happened. A clear needs what is in flight, what is owed, and by
whom. `task` and `reconcile` are nearer that shape and nothing connects them to
"I am clearing now, capture the open loops".

**And the word is already in the tool, on the wrong moment.** `reconcile` is
described as an *agent checkpoint*, and it is anchored to just-finished work.
So it is not that nobody considered agent checkpoints; there is one, aimed
behind rather than ahead.

**The sharpest part, which the report did not have.** The right moment is
already wired. `.claude/settings.json` runs `katra agent-hook snapshot --event
pre-compact` — the hook fires immediately before context is compacted away, and
its entire body is `s.ScanMemory()`. It captures nothing and says nothing. The
moment katra most needs to speak is one it is already standing on, in silence.

## What gets built

### 1. `new --file`, matching `append`

`readChunk` already handles a path, `-`, positional text, and piped stdin.
`new` gains the same flag and calls it. Four lines, no design risk, and it
collapses the two-commands-plus-a-temp-file motion into a pipe.

### 2. `katra checkpoint` — status-shaped, not narrative

The insight is that katra already holds nearly everything a clearing session
must save; nothing assembles it. So `checkpoint` derives the status block rather
than asking a person to write it:

- tasks in flight (`doing`) and owed (`specced`)
- the active draft, or that there is none
- in-flight code paths, and whether they are declared, from `reconcile`
- unresolved memory obligations
- the branch and HEAD it all sits on

Prose the session alone knows is accepted the same way `append` takes it —
positional text, `--file`, or stdin — and lands under the status block. If there
is no active draft, one is created, because a clearing session must not be asked
to make a second decision at the moment it is running out of room.

It writes into the chronicle rather than a new file type. The steward's
`PROJECT-STANDARDS` now says a session's durable state belongs in the project's
chronicle, and a checkpoint that invented its own location would be the
ad-hoc-markdown failure with a nicer name.

### 3. The pre-compact hook writes it

Compaction destroys context whether or not the session cooperates, so the
derived half is written automatically — that is precisely the half that needs no
judgement. The hook then tells the session what it captured and asks for the
prose only it can supply.

**With a threshold, because a hook that always fires is a hook people mute.**
Nothing in flight — no `doing` task, no dirty work-product path, no memory
obligation — means nothing is written and nothing is said. That threshold is
what the report asked a harness to be able to surface; it belongs in the tool,
where every harness gets it, rather than in one harness's configuration.

## What is deliberately not built

No new node type. `docs/format.md` makes an unknown `type` degrade to an entry,
so adding one is a format change needing a migration note, and a checkpoint is
an entry — it is a thing that happened, written at the time.

No threshold configuration. A knob here would be a way to turn the feature off
while believing it is on, and the honest default is "speak when there is
something to lose".
