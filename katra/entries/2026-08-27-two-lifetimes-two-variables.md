---
title: Two lifetimes, two variables
date: "2026-08-27"
time: "10:38:45"
hash: 7c60520
stat:
    f: 8
    a: 199
    d: 67
---

The steward caught a conflation I had already built. My first pass read one KATRA_ACTOR and wrote it to both author and claimed_by. The runtime is about to export a pane nonce for claims — a token that stops resolving when the pane dies, which is the whole elegance: a claim that no longer resolves IS the abandoned-work signal, with no expiry logic and nothing to garbage-collect. But if authorship rode that same variable, every author field would have become unreadable hex the moment a pane closed, and it would still have looked recorded. katra is the across-time half; a field that turns to garbage on pane close fails the guarantee this half exists for.

The seam ruling applied one level down. Two facts, two lifetimes, two fields — and now two environment variables that a test asserts can never collapse into one. KATRA_AUTHOR and KATRA_AUTHOR_ROLE are durable and must be legible years later; KATRA_CLAIM_TOKEN is ephemeral and expected to expire. Both names are declared beside each other in one file so the separation is visible rather than remembered, because the failure mode is not writing the wrong code, it is helpfully reusing the variable that is already there.

author_role is captured at creation, never resolved later. Resolving an identity to a role afterwards answers what that role is today, not what it was at authorship, and roles change — the stale-record failure this fleet keeps paying for. Who ranked this, in what capacity, at the time, is the durable fact and it has to be written when it is true. Absent when unset, and never inferred from the identity: an unset role is absent, not unknown-therefore-probably-a-manager.

Renaming cost nothing, and it is worth recording why. KATRA_ACTOR appeared only in an open PR — v0.1.0 predates the field entirely — so there was no released contract to keep and no compatibility shim to carry. Catching this after a release would have meant either a permanently confusing variable name or a migration for every store. The window was open because the steward read the design rather than the diff.
