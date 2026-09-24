---
title: The one prioritisation field, unreachable
date: "2026-08-27"
time: "10:42:04"
hash: a03be9a
stat:
    f: 8
    a: 131
    d: 1
---

Verified as an asymmetry rather than a missing feature, which is what makes it a bug. docs/format.md documents horizon as a TASK field. epic new has exposed --horizon since it existed. task new never did. So the one prioritisation field the format already has could not be set on the node type it is documented for, and the field exists on 14 nodes that must have been written by hand or by another path.

It was invisible because each command was correct on its own. Nothing compared them. The test that closes it asserts task new and epic new expose the same flag, so a future divergence fails rather than sitting unnoticed — the same shape as the palette drift and the two listings: two things that must agree, with nothing asserting they do.

The cost was real and named. galaxy-brain tracks the fleet's longest-lived goal in a 775-line hand-maintained report with BLOCKERS 1, 2, 3 in order. That ordering could not be expressed in katra at all, and the steward correctly told them not to flatten their list to fit the tool. Fixing the flag does not fix that: now/next/later is three buckets, not an order. What is closed is the gap between what the format promises and what the CLI allows; what remains is that katra has no rank.
