---
title: The gate that could not be satisfied
date: "2026-08-25"
time: "19:44:12"
hash: 61b9bee
stat:
    f: 10
    a: 551
    d: 19
closes:
    - fix-the-unsatisfiable-pre-commit-gate
---

Reported from chesscast: the gate demanded a receipt while reconcile reported 'no changed code' the moment work was staged. Seven legitimate commits blocked in one session before that session worked around it. Induced rather than reasoned about, and the induction changed the diagnosis: the report's read was that reconcile samples after staging has moved the work. Staging turns out to be incidental. The trigger is attribution.

Two units of work, not one. reconcile's unit is the paths the session authored through Edit/Write intersected with the dirty working tree. The gate's unit is the git index. A receipt's identity is a fingerprint over the exact set, so the two agree only when the attributed working-tree set is byte-identical to the staged set. Any divergence and the id the gate looks up is one no receipt will ever carry — not after re-declaring, not ever.

Worse than reported: the remedies report success. In the induced sequence, katra reconcile --no-task prints '✓ recorded' and katra reconcile --skip prints '✓ skipped this unit of work', and the gate stays at exit 2 through both. Both wrote a receipt keyed e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 — the SHA-256 of the empty string, because the unit was empty. A receipt covering nothing, written cheerfully, forever.

A second sequence, more common and more misleading than the reported one. Edit two files through Edit, declare both, stage only one to commit that part first: reconcile status says '✓ reconciled — the gate will allow a stop' while the gate blocks. Not merely unhelpful — actively contradicting itself. That is an ordinary split commit, so this was reachable by anyone, not only by a session doing something unusual.

```embed
src: media/unsatisfiable-gate.html
height: 480
caption: Both induced sequences, before and after. The remedies reported success and changed nothing: the receipt they wrote was keyed on the hash of the empty set.
```

A third route, found by the new message on the day it was written. Probing for one, staging only a .claude/settings.json change — which katra setup produces routinely — reached the unsatisfiable state, and the message reported it correctly and by name before anyone had diagnosed it: 'invisible to reconcile: .claude/settings.json'. The cause: the gate filtered staged paths with IsStorePath while reconcile filters with IsWorkProduct, and the two differ exactly on .claude/. Both sides now ask IsWorkProduct. A safety net that catches something on the day it is installed is not a hypothetical.

Verified by faulting, not by passing. Each half of the fix reverted in turn, and each time the right test failed with the right sentence: reverting per-path coverage fails 'a receipt covering {a.go, b.go} must cover a commit of {a.go}'; reverting the staged-code union fails 'reconcile sees no work while y.go is staged; declaring cannot help'; weakening coverage to path-only fails 'content changed after declaring, but the gate still considers it covered'. The last of those is the one that mattered most — it is the property the whole gate exists for, and the one this change was most likely to break.

Separately, routed not decided: katra is a hash-routed SPA — app.js has only hashchange plus location.hash, no pushState anywhere, and zero analytics instrumentation in the shipped assets. One root cause, two consequences no head-injection hook reaches: per-entry social cards are impossible because the server never sees the URL, and route changes emit no pageview, so reading twenty entries counts as one. The remedy is per-entry pages, which is a product decision about what katra is rather than a defect. Sent to the steward with the cost attached.
