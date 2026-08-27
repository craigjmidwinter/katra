---
title: Count commits since anything was written down
date: "2026-08-27"
time: "09:40:54"
summary: The only threshold input that no amount of missing discipline can starve
type: decision
status: accepted
entry: the-threshold-that-measured-the-record
---

`HasOpenLoops` decided whether the pre-compact hook speaks, and its three inputs — `Doing`, `InFlight`, `Memory` — were all derived from the record the feature exists to repair. The steward put it exactly right: **the threshold's inputs and the problem's cause are the same variable.**

On the first live use: one owed task and one memory against a day that produced sixteen artefacts, with the pending memory the only thing that tripped it. Without that memory the hook would have written nothing into a session at 97% of its context.

`Undocumented` is the fourth input and the only one no discipline can starve: commits on this branch touching non-store paths, since the newest commit any entry is stamped with. Git counts commits whether or not anyone made a task, wrote an entry, or left the tree dirty.

Two properties make it correct rather than merely loud:

- **Store-only commits do not count.** A commit that only touches `katra/` *is* chronicling, so the counter falls to zero precisely when someone does the thing the hook is asking for. A counter that could not be cleared by complying would be a dead gate pointing the other way.
- **The reference point is the last chronicled commit**, so the number is literally "work done since anyone wrote it down" rather than a rolling count that grows forever.

`Doing` stays. The report is right that it lies when nobody maintained a task board; the answer is to stop it being load-bearing, not to remove a real signal from the sessions that do keep one.

Rejected: making `IsWorkProduct` include the store so dirty prose counts. That exclusion is correct — a hook that fired while someone was mid-sentence in an entry would interrupt the very act it wants — and once that prose is committed it is chronicling, which should clear the counter rather than raise it.
