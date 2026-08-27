---
title: The status nobody stores
date: "2026-08-27"
time: "10:20:56"
hash: 24096b8
stat:
    f: 15
    a: 498
    d: 18
closes:
    - derive-doing-from-a-claim
---

Craig ruled: the Manager owns the roadmap, the Director has authority and can direct an override, and Managers are expected to push back. The on_deck question falls out as a consequence — ordering moves to katra under the Manager, and a Director who wants it different asks rather than edits. plans/<session>.yml keeps only autonomy. So katra is now the roadmap store for the fleet, not only the chronicle, and the author field built yesterday stopped being instrumentation and became part of the model: a task that cannot say who ranked it cannot express what was just ruled.

doing is now derived and never written. On disk a claimed task reads status: todo with claimed_by and claimed_at beside it; katra reports doing when asked. The precedence has to be exactly this: a terminal stored status wins, because a finished task is finished whether or not someone forgot to release; then a claim reads as doing; then the stored status, which keeps a legacy doing reading as doing rather than silently reverting to todo. That last one is not politeness — the on-disk format is the public API and katra is released.

An unattributed claim is not the same as no claim, and collapsing them would lose the thing the seam exists for. No claim means nobody took the work up. An unknown claimant means somebody did and the environment could not say who. Author has no equivalent because an unwritten author is not a fact about the work, whereas an unattributed claim is. Faulting the code to collapse them fails with 'a claim with no actor token did not register as claimed'.

The fault injection found my own guard was theatre, on the only path anyone uses. I faulted the CLI to claim AND write status doing, and nothing failed — because the test asserting doing is never persisted drives core directly, and the test on the CLI asserts the derived status, which stays doing either way. Both green, seam quietly undone at the entry point. A CLI-level test now asserts the stored field is untouched by task start; re-faulting it fails with 'task start wrote status: doing; it must record a claim instead'. Two tests can cover a behaviour from both sides and still leave the middle open.
