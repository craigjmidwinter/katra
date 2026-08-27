---
title: Ask the consumer, not the producer
date: "2026-08-27"
time: "10:10:26"
summary: 'Four failures in one family: a success signal not tied to the outcome it claims'
type: decision
status: accepted
entry: the-field-that-had-to-exist-before-the-metric-could
---

The steward's board items never reached Craig: `stem-board --list` empty, `--answers` never used once, against eleven board files since 2026-08-21. So six days of non-delivery read as consent — **by the party the default favoured**. Its own `OUTBOX.md` already carried the rule it broke: nothing is routed until a send has succeeded, and a decision request is a send.

That is the same shape this repo has now hit three times, and it is worth naming as one family rather than three incidents:

- **The release that silently skipped the tap.** `HOMEBREW_TAP_GITHUB_TOKEN` unset, `skip_upload` turned a missing credential into a no-op, the workflow went green having done half of what it advertises.
- **The injector that vanished on rebuild.** Four insertions present, page rendering perfectly, and the next `katra build` removed all four — the producer's view unchanged, the consumer's view empty.
- **The checkpoint threshold computed from the record it exists to repair.** It reported "nothing in flight" into a session at 97% of its context, because the inputs measured whether someone had maintained a record rather than whether work had happened.

**The common shape: a success signal that is not tied to the outcome it claims.** The board file was written, so the routing looked done. The workflow exited zero, so the release looked complete. The page rendered, so the injection looked live. The threshold returned false, so the session looked quiet.

In each case the producer had every reason to believe the job was finished, and **nobody was positioned to notice, because the producer's evidence was genuine and the consumer's absence was invisible from where the producer stood.**

**What actually catches this is asking the consumer, not the producer.** Not "did I write the file" but "did anything read it". Not "did the workflow pass" but "does the tap have the formula". Not "is the tree dirty" but "has anything been written down since the work happened". Every fix in this family has been the same move: replace a proxy that the producer controls with a fact that the consumer would see.

Which is also why the fix for the threshold was to count commits since anything was written down. That number is not a proxy for documentation happening — it is the gap itself.
