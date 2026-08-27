---
title: Write the derived half automatically; ask only for what cannot be derived
date: "2026-08-27"
time: "08:52:42"
summary: A session about to be compacted is the worst possible thing to ask for cooperation
type: decision
status: accepted
entry: the-tool-with-no-entry-for-its-own-release
---

Compaction destroys a session's working knowledge whether or not the session cooperates. Asking it to act at that moment is asking for cooperation from something that is, by definition, out of room — so the derived half of a checkpoint is written without being asked. That half is exactly the part needing no judgement: tasks in flight and owed, changed code and whether it is declared, unresolved memory, branch and commit. katra already held all of it; nothing assembled it.

The session is then asked only for the part it alone has — why the work was being done — which is the part a fresh session most needs and the part no tool can derive.

**With a threshold, because a hook that always fires is a hook people mute**, and a muted hook is not there for the compaction that mattered. Nothing in flight means nothing written and nothing said. A `specced` task deliberately does not count: work that is designed and unstarted is already durable on disk, and losing context does not lose it — counting it would fire the hook on every compaction for the life of the task.

The threshold lives in the tool rather than in a harness's configuration. The report proposed something a harness could surface; putting it in `HasOpenLoops` means every harness gets it, and there is one answer to "is there something to lose" instead of one per integration.

`session-end` is excluded. A session that ended has usually finished, and a checkpoint on every exit is the ceremony that gets the whole thing turned off.
