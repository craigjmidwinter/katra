---
title: The absence of an enforcer is not a violation
date: "2026-08-27"
time: "11:03:56"
summary: Guard every hook, warn about tracked files, and leave the commit choice to the repo
type: decision
status: accepted
entry: you-needed-katra-to-contribute-to-katra
---

The steward left three questions open and called them mine. Answering all three, because leaving one open would have made the other two half-fixes.

**Should `katra setup` write a guarded command? Yes, and for every hook, not only the gate.**

`command -v katra >/dev/null 2>&1 && exec katra agent-hook <event> || exit 0`

The principle, which the steward supplied and which generalises past this bug: **the absence of an enforcer is not a violation.** A hook that cannot find its binary enforces nothing. It only fails, on a machine where nothing is wrong, during someone's first five minutes with the project.

`exec` is load-bearing and a plain call would have been a worse bug than the one being fixed. The pre-commit gate blocks by exiting 2; without `exec` the wrapping shell reports its own status and the gate is silently disabled — a fix that looks like a fix and removes the enforcement. Verified three ways: bare blocks with 2, guarded-with-katra still blocks with 2, guarded-without-katra exits 0 and prints nothing.

**Should it refuse to write into a tracked settings file? No — it should say so.**

Refusing would break every repo that deliberately commits its hooks, which is a legitimate choice this fleet makes. But writing into a tracked file is writing for everyone who clones, not for this machine, and that difference should not be discovered by whoever gets the hooks. `setup` now says it. Now that the hooks no-op without katra, it is a fact worth knowing rather than a hazard.

**Should the file be committed at all? Not katra's call to make for someone else.**

Committing it is how a team shares a workflow, and that is the feature. What made it a defect was not the committing — it was committing something that failed loudly for anyone without the binary. Fix that and the choice becomes free again, which is why the guard is the right layer and a policy about committing would not have been.

**Named because it deserves it:** minds-eye and getvect declined the commit gate on workflow grounds — the bypass being cheaper than compliance — and that choice removed this distribution hazard in the two public-facing repos where it would have mattered most. The gate ruling had a second justification nobody argued for. Worth remembering that a decision made well on one axis can be protective on an axis nobody was looking at.
