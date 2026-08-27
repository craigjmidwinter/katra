# Design: a threshold that measures the thing, not the record of it

Status: accepted, 2026-08-27.
Follows `docs/design/checkpoint.md`. Found by katra's steward on the first live
use of `checkpoint`, in the repo whose session had the most to lose.

## The finding

`HasOpenLoops` decided whether the pre-compact hook speaks. Its three inputs
were `Doing`, `InFlight` and `Memory` — and, as the report put it, **all three
are derived from the record the feature exists to repair**. A session that
worked all day without maintaining a task board scores zero on the one that
depends on discipline, at exactly the moment it is fullest.

On the live run, one owed task and one memory against a day that produced
sixteen artefacts. The pending memory was the only thing that tripped the
threshold. Without it the hook would have written nothing into a session at 97%
of its context.

## The cause is broader than the report diagnosed

The report attributed the blindness to prose repos, where `InFlight` is
structurally near-zero. That is real, and it is the smaller half. Reproduced
here in a repository of ordinary files:

    six artefacts written, each committed as it went
    commits on branch: 8
    working tree:      0 dirty

    ### Checkpoint — 2026-08-27 09:35
    **Where**
    - draft: `hello-katra`
    - branch: `main` at `baf4a72`

    (nothing in flight — a hook would stay silent here)

**`InFlight` is the dirty working tree, so it measures uncommitted work, not
undocumented work.** A session that commits as it goes — which is the habit
katra otherwise encourages — empties the one honest signal it had. This is not a
prose-repo problem. It is every repo, and it is worst for the most disciplined
committers.

The prose half is real too and independent: work whose product lives inside
`katra/` is excluded by `IsWorkProduct`, so a hand-written entry is invisible
even while dirty. That exclusion is correct and stays — a hook that fired while
someone was mid-sentence in an entry would be interrupting the very act it wants.

## The fix: count what git records regardless of discipline

A fourth input, `UndocumentedCommits`: commits on this branch that touch
non-store paths, since the newest commit any chronicle entry is stamped with.

- **Discipline-independent.** Git records commits whether or not anyone made a
  task, wrote an entry, or left the tree dirty.
- **It measures the actual gap.** The reference point is the last time work was
  chronicled, so the number is literally "work done since anyone wrote it down".
- **Store-only commits do not count**, and that is the property that makes it
  correct rather than merely loud: a commit that only touches `katra/` *is*
  chronicling. Excluding it means the counter falls to zero exactly when someone
  does the thing the hook is asking for.

A session with twelve commits and no tasks is not quiet. It is undocumented,
which is the case with the most to lose.

`Doing` stays, because a started task is a real signal when someone did maintain
one. The report is right that it lies when nobody did; the answer is not to
remove it but to stop it being load-bearing.

## Second: a checkpoint must not land in an unrelated draft

`--entry` defaulted to the active draft. On the live run that was an entry
opened hours earlier on a different subject, so a session-wide checkpoint landed
inside unrelated writing.

"With no active draft, one is created" was right; a *stale* active draft is the
common case for exactly the long session this targets. The conflation is the
bug: **a checkpoint is session-scoped, a draft is subject-scoped.**

So a checkpoint gets its own entry. In order: an explicit `--entry`, else
today's existing checkpoint entry, else a new one. That keeps one checkpoint
entry per day rather than a scatter, and never writes into someone's unrelated
draft. The target is printed either way, because a tool that writes somewhere
should say where.

## Third: `decide` created what `append` could not address

    katra decide "…"                        -> katra/decisions/<slug>.md
    katra append --entry <that exact slug>  -> error: no entry with slug "…"

`Store.Get` resolved through `List()`, which is `ListNodes("entry")`. So every
non-entry node — decisions, tasks, epics, articles — could be created with a
summary and then never composed with the tool that created it, and the body had
to be written outside katra. That is the exact displacement this all exists to
remove.

`Get` now resolves across every node type. Nothing else changes: creating a node
is unaffected, and `append` with no `--entry` still means the active draft.
