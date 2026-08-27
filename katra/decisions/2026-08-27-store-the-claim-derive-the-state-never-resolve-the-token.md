---
title: Store the claim, derive the state, never resolve the token
date: "2026-08-27"
time: "10:05:21"
tags:
    - seam
    - proposed
summary: katra half of the stem/katra seam — designed, awaiting Craig ruling on on_deck
type: decision
status: accepted
---

**Proposed, not accepted.** Blocked on Craig's ruling in the steward's board item: two charters both claim the roadmap, and moving ordered `on_deck` out of stem into katra is a Director proposing to delete the Director's own backlog. Building against a boundary that may move is rework, so this records the design and no code exists.

**The shape.** A task carries intent (durable, katra's) and execution (expires with the session, stem's). *In progress* is neither — it is the conjunction of a durable claim and a live session, and nothing should store it.

katra's half is one identity observed at two moments, from one environment variable `KATRA_ACTOR`:

- `author`, written at creation — who put this work into the world.
- `claimed_by` + `claimed_at`, written at claim — who is executing it.

**katra never interprets the token and must never learn to.** Not parsed, not validated, not mapped to a tier, not resolved. Every question about what it *means* is a now question, and stem owns now.

**Absent is a value, not a default.** No variable means no key — never a placeholder, never a tier. A human running `katra task new` by hand is not a manager, and an instrument that assumes so flatters whoever forgot to set a variable. Counts must report their absent count alongside.

**`doing` stops being stored.** It becomes derived from the presence of a claim. katra alone can say *claimed*; it cannot say *alive*, because that needs stem and katra must never read stem. So katra's derived answer means claimed and the CLI says so in those words. stem joins claim against liveness and produces the two signals worth having: claimed with no live session is abandoned work; no claim with a working session is undeclared work. Neither is an error to reconcile away.

**Backwards compatibility is not optional**, because the on-disk format is the public API and katra is released. A stored `status: doing` keeps reading as doing — legacy, not invalid. `katra task start` keeps working and becomes a claim, because breaking it would break every script and harness that learned it. `reconcile --advance` becomes a claim on the same path. Adding two optional keys is additive; changing what `doing` means is not, and needs a migration note in `docs/format.md`.

**Rejected: `cmd_agent_hook.go`'s SessionID as the author token.** It is a Claude Code hook UUID, absent under Codex, never written to a node. Adopting it would make katra's durable record depend on one vendor's hook shape — the exact harness-parity failure the boundary exists to prevent.

**Verified rather than assumed:** katra has zero author, assignee, claim or owner fields, so metric 4's secondary source cannot be computed today or retroactively. `horizon: now|next|later` does exist, confirming the steward's own correction — a coarse bucket rather than a rank, which is a different complaint.
