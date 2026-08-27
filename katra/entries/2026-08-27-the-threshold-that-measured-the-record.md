---
title: The threshold that measured the record
date: "2026-08-27"
time: "09:36:01"
closes:
    - make-the-checkpoint-threshold-see-undocumented-work
    - stop-checkpoints-landing-in-an-unrelated-draft
    - let-append-address-any-node-not-only-entries
---

The reported cause was prose repos; the real one is broader and worse. The steward attributed the blindness to InFlight being structurally near-zero where the work product is prose. That is real and it is the smaller half. Reproduced in a repo of ordinary files: six artefacts written, each committed as it went, eight commits on the branch, clean tree — and the checkpoint printed 'nothing in flight, a hook would stay silent here'. InFlight is the dirty working tree, so it measures uncommitted work rather than undocumented work. A session that commits as it goes empties the one honest signal it had. Every repo, and worst for the most disciplined committers.

The fix has to fall to zero when someone does the thing being asked. Undocumented counts commits touching non-store paths since the newest commit any entry is stamped with. Excluding store-only commits is what makes it correct rather than merely loud: a commit that only touches katra/ IS chronicling, so the counter clears exactly when someone writes something down. Faulting that exclusion fails with 'Undocumented = 1 for a store-only commit', which is the shape of a counter nobody could ever satisfy.

One bug found by its own comment. The first version ran rev-list through s.git, whose cwd is the store directory — so the '.' pathspec meant katra/ and the exclusion inverted the answer, counting chronicle commits as undocumented work and real work as nothing. The helper that fixes it, gitRoot, already existed with a comment saying the store often lives in a subdirectory where pathspecs otherwise resolve wrong. The warning was written before the mistake and did not prevent it; the tests caught it in the same minute.
