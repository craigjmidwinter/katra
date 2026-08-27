---
title: The tool with no entry for its own release
date: "2026-08-27"
time: "08:44:33"
hash: 47ddd02
stat:
    f: 13
    a: 677
    d: 11
closes:
    - let-new-read-its-body-from-a-file-or-stdin
    - add-a-status-shaped-checkpoint-for-the-clearing-moment
    - make-the-pre-compact-hook-speak-at-the-moment-of-need
---

The evidence indicts this repo, not just the steward's. Its chronicle had nothing for 2026-08-26 or 27 — the two heaviest days on record. katra's own log was worse: it stopped at 2026-08-25, so there is no entry for the day katra shipped v0.1.0. The tool that exists to chronicle development has no record of its own first release, written by the sessions that did it, in the repo that is supposed to dogfood it. Two maintainers routed around it in the same week. That is a product finding, not a discipline problem.

Speed was not the problem, and that matters because it is the plausible answer. Draft creation is instant. Optimising it would have been effort spent on the thing that was already fine — the steward said so explicitly, having measured rather than recalled, and it is the single most useful sentence in the report.

The sharpest part was not in the report. The right moment is already wired: .claude/settings.json runs katra agent-hook snapshot --event pre-compact, which fires immediately before context is compacted away, and its entire body was s.ScanMemory(). The steward found that the word checkpoint was already in the tool on the wrong moment — reconcile, anchored to just-finished work. The companion is worse: the correct moment was already instrumented and stood there in silence.

Caught a hang while building it, which is the most useful thing that happened. Wiring new to readChunk unguarded made katra new "Title" block forever under any non-tty caller — readChunk sniffs stdin for a pipe, and stdin is not a character device under a hook, a CI step or a script. I introduced it and hit it within a minute, on my own tool call. It would have been invisible to a test that runs on a tty. Both new and checkpoint now read stdin only when --file asks; piping still works via --file -. The regression test holds stdin open and empty and fails on timeout — faulting the guard reproduces the hang in 15 seconds.
