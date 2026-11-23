---
title: Who answers for it, and what it waits on
date: "2026-11-22"
time: "23:49:41"
tags:
    - format
    - core
    - initiative
hash: 5aa22ed
stat:
    f: 6
    a: 111
    d: 2
summary: Tasks and epics gain optional owner and blocked_by keys, and a pure ValidateNode that writers call and readers never do.
closes:
    - add-owner-and-blocked-by-keys-with-a-pure-validatenode
---

The first of the initiative-model changes (Daisy's wave-2 note, section D).
The galaxy-brain log carries ownership in 81 `owner:` tags and blocking in 10
`blocked-by:` tags, because katra had no key for either. Now it does:

- `owner`, a string. Opaque in the same way the claim seam treats identity:
  katra stores it and never resolves it. It is not the author. The format
  still has no author field on purpose (git records who wrote a file); owner
  answers a different question, who is responsible for the node now.
- `blocked_by`, a list. An item is usually a node slug, but three of the ten
  galaxy-brain blockers are a decision Craig owes, not a node, so free text is
  allowed. Resolving slugs is `doctor`'s job, in a later change.

Both are `omitempty`, so every existing file serializes unchanged; the test
for that fails if either tag loses `omitempty`.

`ValidateNode` is deliberately pure and deliberately writer-side. It checks a
blank owner and an empty, duplicate or self-referencing blocker, and later
changes add the effort and status enums to it. Nothing on the read path calls
it: the format promises that an odd value degrades rather than erroring, and
a katra written by a newer tool must still list and render.

This needed the previous change first. Before it, a hand-added `owner:` was
erased by the next save; now it is a field.
