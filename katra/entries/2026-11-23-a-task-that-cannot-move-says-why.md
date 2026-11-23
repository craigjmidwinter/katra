---
title: A task that cannot move says why
date: "2026-11-23"
time: "00:00:11"
tags:
    - tasks
    - board
    - cli
    - mcp
    - initiative
hash: c772f82
stat:
    f: 21
    a: 596
    d: 33
summary: Tasks gain blocked and waiting_on_owner, set by block, wait and unblock, validated at every write, and shown in a Blocked column.
closes:
    - blocked-and-waiting-on-owner-task-statuses
---

Until now a stuck task had nowhere to go. It stayed `todo` or `doing`, and
the reason it was stuck lived in a tag (`blocked-by:` ten times in the
galaxy-brain log) or in someone's head. Brainstem's board then counted it as
ordinary todo.

Two statuses, because there are two kinds of stuck:

- `blocked` waits on something: other nodes, or an outside fact such as a
  decision not yet made. It needs a non-empty `blocked_by`, so the board can
  always say what.
- `waiting_on_owner` waits on a person. It needs an `owner`, because a task
  cannot wait on nobody.

`katra task block --by` adds blockers (repeatable, and a comma inside a value
is kept, since free-text blockers have commas), `task wait --owner` waits, and
`task unblock` returns the task to where it would stand had it never stopped:
`specced` if it has a spec, else `todo`. Unblock does not restart it; that
stays a deliberate `task start`. It clears `blocked_by`, because what it named
no longer holds.

Every status write now goes through one validated setter, so `task start`,
`task done` and the MCP `set_status` refuse `blocked` with no blockers, an
epic status on a task, or a typo like `wip`, and write nothing. The check
compares against the file on disk and refuses only problems the change
introduces: a node with a legacy `effort: medium` can still be started.
Reading stays tolerant; nothing on the read path validates.

The epic rollup needed no code: it only counts `doing` as started, so both
new statuses already fall into not started. Three new cases pin that.

![The viewer board with the new Blocked column: one card blocked by a task and an outside check, one waiting on its owner.](media/k3-board.png)
