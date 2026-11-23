---
title: One odd file, one empty key
date: "2026-11-23"
time: "13:34:16"
tags:
    - review
    - format
    - migrate
    - initiative
hash: 11c9423
stat:
    f: 9
    a: 263
    d: 25
summary: 'Daisy''s review found two must-fixes: a wrong-shaped value made the store unreadable, and migrating onto an empty key wrote a file that did not parse.'
closes:
    - fix-daisy-s-review-of-the-initiative-model-h1-m4-low-notes
---

Daisy reviewed K0–K5 at 72b3f6c and reproduced two must-fix bugs. Both came
from the same blind spot: I tested the shapes katra writes, not the shapes
people write.

**H1.** `owner` became a string field and `blocked_by` a list, so
`blocked_by: other-task` (one blocker, written the natural way) and
`owner: [a, b]` were type errors. A type error failed the parse, and
`ListNodes` stops at the first failed file, so one hand edit made `list`,
`stamp`, MCP and the viewer fail for the whole store. Before K1 the same file
parsed fine, with the value in `Extra`.

The fix has two layers. `blocked_by` takes a scalar as a one-item list, which
is what anyone writing it means. For a shape katra cannot hold at all, a type
error is no longer a parse failure: yaml.v3 decodes every other field and
reports the mismatches, so the node lists with that field empty and the
mismatch recorded on `Entry.Unreadable`. Doctor reports it, and `Serialize`
refuses to write the node, because a rewrite would erase the value it could
not read. Malformed YAML is still an error.

**M4.** The migration set a replaced value by editing the YAML node's
`Value`, keeping its tag. On `owner:` (empty, so null) plus an `owner:craig`
tag that produced `owner: !!null craig`, which does not parse, after the tag
had already been dropped. It now replaces the node. Apply also re-parses
every file it is about to write and refuses unless the values read back as
planned, then writes through a temp file and a rename. Either of those alone
would have stopped this bug reaching a disk.

Daisy's low notes, taken where they were cheap: two `size:` tags are a
conflict, not last-one-wins; duplicate blockers are compared trimmed; a
`blocked-by:` key (hyphen, like `superseded-by`) is named by doctor instead of
vanishing into `Extra`; an ask id in `blocked_by` is a pending decision, not a
missing node; the migration keeps a file's own list indent; the MCP
`task_list` description names the new statuses; and the format doc now says
plainly that kept unknown values are re-encoded, not copied.

Left, with reasons: `status: open` → `todo` (a spec item, not this review's);
`CloseTasks` and `ReconcileAdvance` validating (they only ever write `done`
and `doing`); a browser test for the Blocked column.

Each new test fails with its fix reverted.
