---
title: The Katra workflow
layout: default
nav_order: 3
description: >-
  The harness-neutral spec-to-stamp loop for tasks, epics, decisions, entries,
  visual evidence, task closure, and optional automation.
---

# The Katra workflow

This is the common workflow for a person, Codex, Claude Code, an MCP client, or
any other tool that can run the `katra` CLI. Harness integrations can remind or
automate parts of it; none of them define the workflow.

{: .warning }
The `specced` phase, `katra task spec`, and `katra task new --spec` are present
in current source but not in the installed v0.1.0 CLI. Until the next release,
build current source with `make all` before using the complete flow below. The
shorter v0.1.0 lifecycle remains `todo → doing → done | cut`.

## The loop in one screen

```bash
# Plan a larger outcome and one bounded unit of work.
katra epic new "Portable agent workflow" --horizon now
katra task new "Document the common loop" --epic portable-agent-workflow

# Write a durable design, point the task at it, then commit the planning state.
katra task spec document-the-common-loop docs/design/common-loop.md
git add docs/design/common-loop.md katra/epics/ katra/tasks/
git commit -m "docs: specify the common loop"

# Begin from the committed spec and chronicle while implementation is live.
katra task start document-the-common-loop
katra new "The workflow escaped the harness" --tags agents,workflow
katra append "The first approach depended on private harness state; the CLI won because it leaves a committed contract."
katra capture /tmp/workflow.html --caption "The plan-to-proof path"

# Commit the implementation, then publish the entry and close the task.
git add -A
git commit -m "docs: publish the common workflow"
katra stamp --closes document-the-common-loop --commit
```

That final stamp marks the task `done`, links it to the entry, and rolls up its
parent epic. With the portable post-commit hook installed, the implementation
commit stamps automatically; declare the closure first with
`katra reconcile --close document-the-common-loop` so the draft carries its
`closes:` edge when the hook publishes it.

## 1. Find or declare the work

Start by reading durable state, not reconstructing intent from a chat:

```bash
katra task list --status specced   # designed and ready to implement
katra task list --status doing     # already in flight
katra epic rollup                  # larger outcomes and computed status
```

For a new outcome, create the epic first and attach its child task:

```bash
katra epic new "Portable agent workflow" --horizon now
katra task new "Document the common loop" \
  --epic portable-agent-workflow --effort M
```

An epic begins `planned`. Starting a child moves the computed epic status to
`active`; closing its final open child rolls it to `done`.

## 2. Put the spec before the implementation

Not every task needs a design. When one does, write the artifact before code
and commit it with the task pointer so another session can recover the intent.
A spec can be:

- a decision or article node created with `katra decide` or
  `katra article new`;
- an entry already in the Katra;
- any committed Markdown file in the repository.

Attach it by node slug or repository-root-relative path:

```bash
katra task spec document-the-common-loop docs/design/common-loop.md
katra task list --status specced
```

`task spec` changes `todo` or an empty status to `specced`; it never moves
`doing`, `done`, or `cut` backwards. An unresolved reference warns but writes,
because the artifact may be added in the same change. `katra doctor` reports a
reference that is still unresolved afterwards.

Read the committed artifact, then start:

```bash
katra task start document-the-common-loop
```

## 3. Open the entry before editing code

A draft is simply an entry with no `hash:` or `hashes:` frontmatter. Create it
while the problem, rejected alternatives, and evidence are still available:

```bash
katra new "The workflow escaped the harness" --tags agents,workflow
```

One draft at a time is the simplest path. When several are open, every writing
command accepts `--entry <slug>`; otherwise the newest draft is active.

## 4. Chronicle as you go

Write a post for a later reader, not a handoff transcript:

- Open with the stakes rather than a filename.
- Name the alternative that lost and why.
- Record the decision and reasoning the diff cannot show.
- End with what is now true and what remains uncertain.

Append short reasoning directly or pipe longer Markdown:

```bash
katra append "A private hook could enforce this, but a committed CLI recipe survives every harness."
katra append --file notes.md
printf '%s\n' 'Longer Markdown from another command.' | katra append --file -
```

Promote a choice that future work will ask about into a durable decision:

```bash
katra decide "Published docs own the common workflow" \
  --entry the-workflow-escaped-the-harness \
  --body "Harness adapters may automate the loop, but cannot redefine it."
```

Avoid bodies that are only invariant lists, tool receipts, or instructions to
the next agent. Put coordination in the task and evidence in the entry, but
write the entry around the reason the result matters.

## 5. Show the thing

Treat a draft with no visual as unfinished whenever the work can be shown:

1. Capture a UI, render, screenshot, animation, or before/after.
2. Render structured output such as a graph or layout.
3. Chart measurements, sizes, timings, or pass rates.
4. Diagram architecture when there is no visible product surface.

```bash
katra capture screenshot.png --caption "after the fix"
katra compare before.png after.png --caption "before and after"
katra capture /tmp/benchmark.html --caption "p95 latency by workload"
```

For a chart or diagram, make a self-contained HTML file and capture it. Use no
remote scripts, fonts, or images; give it an explicit background, size it for
the content column, and label axes and units. Inline SVG is usually enough.
`capture` copies the artifact into `katra/media/` and emits an `embed` block.

Rich component fences available to appended Markdown are `embed`, `compare`,
`gallery`, `video`, `note`, and `warning`. Unknown fences remain ordinary code
blocks, so the Markdown degrades safely in older readers.

## 6. Reconcile, commit, stamp, and close

The portable manual path is:

```bash
git add -A
git commit -m "docs: publish the common workflow"
katra stamp --closes document-the-common-loop --commit
```

`stamp` defaults to `HEAD`, computes the diffstat, marks the entry published,
closes and links the named task, rolls up its epic, and—when `--commit` is
given—commits those Katra bookkeeping changes. Use `--hash a,b,c` for an entry
that describes a chapter of several commits.

Before declaring completion, run:

```bash
katra doctor
katra task list --status done
katra epic rollup portable-agent-workflow
katra list
```

## Optional automation

The portable automation is the Git post-commit hook:

```bash
katra init --install-hook   # new Katra plus the hook
katra hook install          # add it to an existing Katra
```

Before the implementation commit, attach the task closure to the active draft:

```bash
katra reconcile --close document-the-common-loop
git add -A
git commit -m "docs: publish the common workflow"
```

The hook stamps that commit, closes the task, rolls up the epic, and leaves the
Katra changes for review and a bookkeeping commit. `autoCommit: true` can make
that second commit automatic. The hook honours `core.hooksPath`.

`katra setup` additionally installs the Claude Code skill, session hooks,
memory adapter, and optional blocking gate. Those are Claude-specific
conveniences. Codex and other harnesses use the same CLI workflow above; they
do not need private-memory ingestion or harness hooks to produce a complete
entry.

## MCP equivalents

`katra-mcp` exposes the same core operations as fifteen tools: seven entry
tools (`katra_list`, `katra_get`, `katra_new`, `katra_append`,
`katra_capture`, `katra_compare`, `katra_stamp`) and eight node tools
(`katra_nodes`, `katra_task_new`, `katra_task_list`,
`katra_task_set_status`, `katra_task_spec`, `katra_epic_new`, `katra_decide`,
`katra_article_new`). See [Agents](agents#mcp) for the inventory.
