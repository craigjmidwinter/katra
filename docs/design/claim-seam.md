# Design: katra's half of the stem/katra seam

**Status: proposed. Not implemented, and deliberately not committed as code.**
Blocked on Craig's ruling in `oss-steward/board/2026-08-27-two-charters-both-claim-the-roadmap.md`.

Evidence: `oss-steward/SEAM-2026-08-27-stem-katra-boundary.md`.

## What is being held and why

The steward routed four items in a fixed order, and this is item 3. Item 1 is
Craig's ruling on whether ordered `on_deck` moves out of stem into katra — a
Director proposing to delete the Director's own backlog. Building `claimed_by`
against a boundary that may move is rework, so this document is the deliverable
and there is no code.

Two things are verified rather than assumed, because the design rests on them:

    fields katra knows: title date time tags hash hashes stat summary cover
      featured pinned type status spec effort horizon epic entry closes
      advances supersedes superseded
    author | assignee | claim | owner: 0 matches

    status: doing is written by `katra task start` (cmd_node.go:142)
      and by reconcile --advance (publish.go:122)

The steward's correction is also confirmed: `horizon: now|next|later` exists. It
is a coarse bucket, not a rank, which is a different complaint from the one
originally made.

## The ruling this implements

A task carries two facts with different lifetimes. **Intent** — this work should
happen, in this order, for this reason — survives every session and is katra's.
**Execution** — someone is doing it right now — expires with the session and is
stem's.

*In progress* is neither. It is the conjunction of a durable claim and a live
session, and **nothing should store it**. Both systems storing it is what makes
them half-know the same thing.

## One identity, two moments

An actor token comes from the environment, is opaque to katra, and is recorded in
two places for two different reasons:

| field | written when | answers |
|---|---|---|
| `author` | the task is created | who put this work into the world |
| `claimed_by` + `claimed_at` | the task is claimed | who is executing it |

One environment variable, `KATRA_ACTOR`, because these are the same kind of
identity observed at two moments. Two variables would invite them to disagree.

**katra never interprets the token and must never learn to.** It does not parse
it, validate its shape, map it to a tier, or resolve it to anything. It is a
string katra stores and hands back. Every question about what the token *means* —
which session, which tier, alive or dead — is a *now* question, and stem owns now.

**Absent is a value, not a default.** No `KATRA_ACTOR` means no `author` key,
never a placeholder and never a tier. A human running `katra task new` by hand is
not a manager, and an instrument that assumes so flatters whoever forgot to set a
variable. Any count katra reports must state its absent count alongside, per the
no-silent-caps rule — an instrument that hides how much it could not see is
measuring compliance, not behaviour.

## `doing` stops being stored

This is the part that carries real cost, and it is the part that makes the
pattern enforceable rather than described.

Today `doing` is a value a human writes with `katra task start`. Under the
ruling it becomes **derived at read time from the presence of a claim** and is
never written to disk again. The lifecycle on disk becomes:

    todo → specced → (claimed) → done | cut

with `claimed_by` present being what makes a task read as `doing`.

**What katra derives alone, and what it does not.** katra can say a task is
*claimed*: a durable claim exists. It cannot say the claimant is alive, because
that requires stem, and katra must never read stem. So katra's derived `doing`
means **claimed**, and the CLI says so in those words rather than implying
liveness it cannot check. stem joins claim × liveness and produces the two
signals worth having:

| katra says | stem says | meaning |
|---|---|---|
| claimed | no live session | **abandoned work** |
| no claim | session working | **undeclared work** |

Neither is an error to reconcile away. They are the product.

### Backwards compatibility, which is not optional here

`docs/format.md` makes the on-disk format the public API, and katra is released.
So:

- A stored `status: doing` on an existing task **keeps reading as doing**. It is
  legacy, not invalid, and nothing rewrites it.
- New claims do not write `status: doing`. The claim is the record.
- `katra task start` keeps working and becomes a claim. It is the same intent
  expressed in the old vocabulary, and breaking it would break every script and
  every harness that learned it.
- `reconcile --advance` (`publish.go:122`) writes `doing` today. It becomes a
  claim on the same code path.

This needs a migration note in `docs/format.md`. Adding two optional keys is
additive and degrades safely; changing what `doing` *means* does not, and saying
so is the difference between a migration and a surprise.

## Harness parity is the constraint that shapes all of it

katra must answer "what is in flight" under Codex, in CI, and on a machine with
no stem installed. That is why:

- the token is opaque — resolving it would require the resolver;
- the claim is stored but liveness is not — katra can read its own claim without
  asking anything;
- `KATRA_ACTOR` is an environment variable rather than a stem lookup — every
  harness can set one, and a harness that sets none produces honest absence.

`cmd_agent_hook.go:20` already carries a `SessionID` from the Claude Code hook
payload. **It must not become the author token.** It is a harness UUID, it is
absent under Codex, it is never written to a node, and adopting it would make
katra's durable record depend on one vendor's hook shape — the parity failure
this whole boundary exists to prevent.

## What this does not do

It does not make katra read stem, ever. It does not let katra resolve a token to
a tier. It does not persist "in progress" anywhere. And it does not attempt
metric 4 on its own: katra's author token is the *secondary* source, and the
primary one — writes to `on_deck` in stem's plan files — lives in stem and
catches the case katra structurally cannot, which is a Director who ranks work
without creating a single task.

Stating that blind spot is part of the design. An instrument that counts only the
compliant representation of a behaviour measures compliance, not behaviour.
