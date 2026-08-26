# Design: the pre-commit gate that could not be satisfied

Status: accepted, 2026-08-25.
Task: `fix-the-unsatisfiable-pre-commit-gate`.
Reported from `~/workspace/chesscast`, where it blocked seven legitimate
commits in one session before that session worked around it.

## Why this is the worst class of gate

A gate that fires wrongly costs a person one argument. A gate that *cannot
pass* costs them the practice. Every session it blocks either loses its commits
or learns to reach for `--no-verify`, and the second outcome is the one that
quietly ends chronicling — in a tool whose entire pitch is chronicling work as
you build.

So this is not filed as a bug in a hook. It is filed as a bug in the product's
core loop.

## Two units of work, not one

The gate and `reconcile` each compute "the work", and they compute it
differently:

| | source of the unit |
|---|---|
| `katra reconcile` | paths the session authored via Edit/Write, **intersected with the dirty working tree** |
| pre-commit gate | **the git index** — every staged non-store path |

A receipt's identity is `fingerprintChangeRecords` over *the exact set*. So the
two agree only when the attributed working-tree set is byte-identical to the
staged set. Any divergence and the id the gate looks up is one no receipt will
ever carry — not now, not after re-declaring, not ever.

The report's read was that `reconcile` samples after staging has moved the work.
That is close, and it is not quite it: staging is incidental. The trigger is
**attribution**. Induced rather than reasoned about, two sequences reach it.

### Sequence 1 — a second commit in the same turn

Session edits `x.go` through Edit, declares it, commits it. Still in the same
turn, more work appears from a shell command — a generator, `gofmt`, `sed`, a
dependency bump — and is staged.

`x.go` is now committed, so it is no longer dirty; the intersection with the
session's touched set is empty. Because the session *has* touches,
`mostRecentTouchedSession` returns it and the repo-wide fallback never runs, so
the new file is invisible:

    reconcile: no changed code in the working tree — nothing to reconcile
    katra: staged code isn't covered by a reconciliation receipt.   [exit 2]

Mutually exclusive, exactly as reported. Worse: the remedies *report success*
and change nothing.

    $ katra reconcile --no-task --reason "declaring the staged work"
    ✓ recorded: this work advances no task
    gate exit after declaring: 2

    $ katra reconcile --skip --reason "just let me commit"
    ✓ skipped this unit of work
    gate exit after --skip: 2

Both wrote a receipt keyed
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` — the
SHA-256 of the empty string, because the unit was empty. A receipt covering
nothing, written cheerfully, forever.

### Sequence 2 — an ordinary split commit

More common and more misleading. The session edits `a.go` and `b.go` through
Edit, declares both, then stages only `a.go` to commit that part first:

    reconcile status: ✓ reconciled — the gate will allow a stop
    gate:             blocked (exit 2)

Here `reconcile` does not merely fail to help, it actively says the work is
reconciled while the gate refuses it. Re-declaring with `a.go` staged does not
help, because the working-tree unit is still `{a.go, b.go}`.

## The fix: coverage is per path, not per set

A receipt declaring `{a.go, b.go}` genuinely does cover a commit of `{a.go}`.
Committing a subset of declared work is still declared work. The whole-set
fingerprint cannot express that, so the receipt gains a per-path record:

    pathRecords: { "a.go": "<fingerprint of that path's change record>" }

A staged path is covered when some resolving receipt carries an identical
`(path, record)` pair. Because the record is the existing `changeRecord`
— op, mode, HEAD blob, new blob — this keeps every content-sensitivity property
the whole-set id had: change the content after declaring and the record changes,
so the gate blocks again. It only stops demanding that the *set* match.

Receipts written before this change carry no `pathRecords`; those fall back to
the whole-set comparison, so an existing ledger keeps working.

Second, `reconcile`'s unit now includes staged non-store code. The index is
definitionally what a commit will record and what the gate will judge, so
reconcile must see at least that much. This is what makes sequence 1's empty
unit impossible.

## The message is the part that outlives this bug

Both fixes above close the two sequences found. Neither proves there is no
third, and the failure mode of a third would be silent and identical: a person
staring at a gate, doing what it says, watching nothing happen.

So the gate now distinguishes two states it previously conflated:

**Receipt required** — normal, satisfiable, the message it always had.

**Receipt required, and reconcile cannot see the work** — the broken state.
When the gate is about to block, it asks what `reconcile` would compute. If
reconcile's unit does not cover the staged paths, the gate says so, names the
paths that are invisible, and says plainly that this is a katra bug rather than
something the person did wrong:

    katra: staged code isn't covered by a reconciliation receipt, and
    `katra reconcile` cannot see it either — declaring will not help.
      invisible to reconcile: y.go
      This is a katra bug, not something you did wrong.
      Commit with:  git commit --no-verify
      Please report: https://github.com/craigjmidwinter/katra/issues

The difference between the two messages is the difference between "you have a
step left" and "this tool is broken, here is the way out". Only the second one
lets a person stop trying.
