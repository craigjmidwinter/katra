---
title: Answer coverage per path, not per set
date: "2026-08-25"
time: "19:49:40"
summary: Committing part of declared work is still declared work; requiring the set to match is what made the gate impossible
type: decision
status: accepted
entry: the-gate-that-could-not-be-satisfied
---

A receipt declaring `{a.go, b.go}` genuinely does cover a commit of `{a.go}`. The whole-set fingerprint could not express that, and a split commit is the most ordinary thing a person does — so the gate refused a routine action with a message implying the person had a step left.

Receipts now carry `pathRecords`: each declared path's change record fingerprinted on its own. A staged path is covered when some resolving receipt carries an identical `(path, record)` pair.

The content-sensitivity property is deliberately preserved, because it is the reason the gate is worth having. The per-path key is the same `changeRecord` the set fingerprint was built from — op, mode, HEAD blob, new blob — so declaring a file and then editing it makes the record change and the gate asks again. Tested by faulting the check down to path-only matching: `TestGateReblocksOnChangedContent` fails with 'content changed after declaring, but the gate still considers it covered'.

Receipts written before this field exist without it and fall back to the whole-set comparison, so an existing ledger keeps working rather than needing a migration.

Second half: reconcile's unit now includes staged non-store code. The index is definitionally what a commit records and what the gate judges, so reconcile must see at least that much. This is what makes the empty-unit receipt impossible.
