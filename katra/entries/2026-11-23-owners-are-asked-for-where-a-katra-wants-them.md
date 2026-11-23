---
title: Owners are asked for where a katra wants them
date: "2026-11-23"
time: "00:03:33"
tags:
    - owner
    - doctor
    - config
    - initiative
hash: "8641421"
stat:
    f: 13
    a: 329
    d: 4
summary: Creation takes --owner, a katra can require it, and doctor reports owner gaps and blockers that name nothing or are all finished.
closes:
    - owner-on-creation-requireowner-and-doctor-checks
---

`owner` existed as a key after K1 but nothing wrote it. Now `task new` and
`epic new` take `--owner`, and the MCP creation tools take `owner`.

`requireOwner` in `config.yml` refuses a task or epic created without one. It
defaults to off. The format says making an optional key required is a
breaking change, and the right unit for that choice is the katra, not katra:
the galaxy-brain log can opt in; a solo project never hears of it.

`doctor` grew three warnings, all warnings and never problems, so a katra
written by another tool still passes:

- any value `ValidateNode` rejects, which is where K2's promise that odd
  efforts get reported lands;
- a blocked task whose blocker is slug-shaped and names no node (a typo, or a
  renamed node), or whose blockers are all nodes that are now done or cut,
  so it is blocked on nothing. Free text (anything with a space) is an outside
  fact and is left alone;
- open tasks and epics with no owner, listed only once the katra sets
  `requireOwner` or any node has an owner. Rejected: always listing them. On
  this repo that would be every open task, on every run, about a field the
  project has never used.

The viewer's board shows `@owner` on a card.
