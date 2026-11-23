---
title: Tags that were standing in for keys
date: "2026-11-23"
time: "00:07:45"
tags:
    - migrate
    - format
    - initiative
hash: b3aa786
stat:
    f: 7
    a: 520
    d: 0
summary: 'katra migrate interim-tags moves owner:, blocked-by: and size: tags into their keys, as a dry run first and then a minimal edit.'
closes:
    - katra-migrate-interim-tags
---

The last of the five initiative-model changes. Before `owner`, `blocked_by`
and `XL` existed, the galaxy-brain log recorded them as tags: 81 `owner:`, 10
`blocked-by:`, 4 `size:XL`. `katra migrate interim-tags` moves each into its
key and drops the tag, so the two cannot disagree later.

Against a scratch copy of that log (galaxy-brain at 7cb7424; its own tree was
only read), the dry run reported exactly the expected 81 owners, 10 blockers
and 4 efforts, and changed no file hash. `--write` then migrated 81 nodes, and
a second run found nothing.

The first `--write` was not good enough. It saved through `Entry.Save`, which
re-serializes the whole frontmatter in katra's own style, and on hand-written
files that re-quoted titles and summaries and dropped blank lines before the
body: noise in another lane's diff, which a reviewer has to read around. The
write now edits the frontmatter as a YAML node tree, so untouched values keep
their exact quoting, and copies the body byte for byte. The diff on the copy is
now 99 lines out and 105 in, and every one of them is a migrated line.

Decisions:

- **size beats effort.** The four `size:XL` tasks all say `effort: L`,
  because L was the largest value the scale had. The tag is the real estimate.
- **Statuses are not touched.** Ten tasks gain `blocked_by`, but whether each
  is blocked today is a judgement, and some blockers are finished; doctor now
  reports those once a task is marked blocked.
- **Conflicts stay tags.** Two owner tags, or a tag that disagrees with an
  existing `owner`, are reported rather than guessed at. None occur in the
  galaxy-brain log.
