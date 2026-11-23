---
title: Ignored had come to mean erased
date: "2026-11-22"
time: "23:47:14"
tags:
    - format
    - core
hash: 373bab3
stat:
    f: 4
    a: 100
    d: 0
summary: Saving a node re-marshalled katra's own fields and dropped every key it did not know; unknown keys now round-trip.
closes:
    - keep-unknown-frontmatter-keys-when-a-node-is-saved
---

The format promises that an unknown frontmatter key is ignored, never an
error. Reading kept that promise. Writing broke it: `Serialize` marshals the
`Frontmatter` struct, so a status change, a stamp or a spec link rewrote the
file with only the keys katra has fields for. A key another tool added (or a
newer katra) survived until the first time an older one touched the file.

This came up while specifying the initiative model (owner, blocked_by): the
galaxy-brain log already carries ownership and blocking in tags because there
was no key for it, and a hand-added `owner:` key would have been deleted by
the next `katra task start`. Adding the keys first, without this, would only
move the problem to whichever key comes next.

The fix is yaml.v3's inline map. `Frontmatter.Extra` collects every key with
no field and writes it back after the known ones, sorted, with its value; it
is excluded from JSON so nothing new leaks into `data.json`. A node with no
foreign keys serializes byte for byte as before, so no existing file changes.
Comments inside the frontmatter are still lost on a rewrite; that is now
stated in `docs/format.md` rather than discovered.

The test writes a task carrying `owner`, `blocked_by` and a nested map,
changes its status through `Save`, and checks every key is still there. With
the inline tag swapped for `yaml:"-"`, it fails.
