---
title: katra holds history well and judgement poorly
date: "2026-08-27"
time: "10:42:26"
summary: 'Three migration findings: the flag is fixed, the rank and the headline are not'
type: decision
status: accepted
entry: the-one-prioritisation-field-unreachable
---

Three findings from a real migration assessment — galaxy-brain's 775-line hand-maintained readiness report, evaluated against katra before anyone was asked to move it. Recorded together because the first is now fixed, the second and third are filed, and the distinction between them matters.

**One: katra has no rank, and the flag was only half of it.**

`--horizon` missing from `task new` while `epic new` had it, against a format that documents `horizon` as a *task* field, was a straight asymmetry and is fixed. But fixing it does not give katra a rank. `now|next|later` is three buckets; galaxy-brain's report has BLOCKERS 1, 2, 3 **in order, with dispositions**. That ordering still cannot be expressed.

The steward told them not to flatten their list to fit the tool, and to record that the order is pending a katra field. That is the right call and worth naming as a rule: **a tool that cannot hold a shape should be told, not accommodated.** Every list flattened to fit becomes evidence the tool was sufficient.

**Two: no standing-judgement view.**

Their report is one addressable artefact with a headline, and the headline is *conditional*: GO as a single-operator scheduler, NO-GO as a multi-tenant orchestrator. katra scatters that across N nodes, and `epic rollup` computes status from children rather than answering "are we ready, and for what".

A chronicle plus a task list does not reconstitute a headline. Rollup answers *how much is done*; a standing judgement answers *what do we currently believe, and under which conditions*. The second is not derivable from the first, which is why no amount of better rollup produces it.

**Three: adjacency is a document affordance katra does not have.**

They keep superseded measurement tables *next to* their replacements — "a second row, not an edit" — so a figure that was green locally and red in CI within the hour stays visibly disagreeing. As two katra entries that is two files, and the adjacency is gone. This one may not be katra's to fix; an entry with two stamped tables might be what it is for, but nothing makes it natural.

**What katra holds better, recorded so the assessment is not one-sided:** struck-through-with-date becomes `status: done` plus the entry that closes it, stamped with commit and diffstat — strictly richer than a strikethrough. Per-commit measurement stamping is literally `katra stamp`. And their conditional headline is a decision with reasoning, which katra has a first-class type for, with a supersedes chain.

So the gap is narrower than "katra cannot hold this" and sharper than a missing flag: **katra holds history well and judgement poorly.**
