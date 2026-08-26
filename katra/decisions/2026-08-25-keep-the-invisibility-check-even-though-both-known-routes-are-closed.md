---
title: Keep the invisibility check even though both known routes are closed
date: "2026-08-25"
time: "19:49:52"
summary: The message is the part that outlives this bug, and it earned its place by finding a third route
type: decision
status: accepted
entry: the-gate-that-could-not-be-satisfied
---

The two fixes close the two sequences that were induced. Neither proves there is no third, and the failure mode of a third is silent and identical: a person staring at a gate, doing exactly what it says, watching nothing happen.

So when the gate is about to block, it asks what `reconcile` would compute. If reconcile cannot see the staged paths, the message changes:

    katra: staged code isn't covered by a reconciliation receipt, and
    `katra reconcile` cannot see it either — declaring will not help.
      invisible to reconcile: y.go
      This is a katra bug, not something you did wrong.
      Commit with:  git commit --no-verify
      Please report: https://github.com/craigjmidwinter/katra/issues

'Receipt required' and 'receipt required and reconcile cannot see the work' are different situations, and only the second lets a person stop trying. It costs one evaluation on a path that is already blocking and about to fail.

**It justified itself immediately.** Probing for a third route, staging only a `.claude/settings.json` change — which `katra setup` produces routinely — reached the unsatisfiable state, and the new message reported it correctly and by name before anyone had diagnosed it. The cause was the gate filtering staged paths with `IsStorePath` while reconcile filters with `IsWorkProduct`; the two differ exactly on `.claude/`. Both sides now ask `IsWorkProduct`, which closes it.

A safety net that catches something on the day it is installed is not a hypothetical.
