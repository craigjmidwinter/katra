---
title: Five sizes, and only five
date: "2026-11-22"
time: "23:52:34"
tags:
    - format
    - cli
    - mcp
    - initiative
hash: 1baf88a
stat:
    f: 10
    a: 122
    d: 6
summary: 'Effort becomes the scale XS|S|M|L|XL: writers normalize case and refuse anything else, readers stay tolerant.'
closes:
    - effort-enum-xs-s-m-l-xl-refused-otherwise-at-write
---

Effort was documented as `S|M|L` and enforced nowhere: the CLI and the MCP
tool wrote whatever they were given. The galaxy-brain log shows the cost: five
tasks use an undocumented `XS`, four carry `size:XL` as a tag because there
was no XL, and nothing stops `medium` or `m` from reaching a board column.

The scale is now `XS|S|M|L|XL`, in `core.Efforts`. `NormalizeEffort` trims and
uppercases (`m` becomes `M`) and refuses anything off the scale with an error
that lists it. Both writers call it before creating the task, so a refused
effort creates nothing.

Readers are untouched. A file with `effort: medium` still parses, lists and
renders, because the format promises that an odd value degrades; only
`ValidateNode` reports it, and it also flags a lowercase value on disk, which
no writer produces and so marks a hand edit.

Rejected: mapping `size:` tags here. That is the migration's job (K5), with a
dry run first, not a side effect of creating a task.
