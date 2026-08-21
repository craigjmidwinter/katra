---
title: Agents
layout: default
nav_order: 7
description: >-
  How an agent keeps the log — the Claude Code skill, the seven hooks katra
  setup installs, the blocking commit gate, memory ingest, and the MCP tools.
---

# Agents

katra was built to be written *by* an agent while it works, so the log records
what happened rather than what someone reconstructed afterwards. There are three
ways in, and they compose.

| Surface | Use it when |
| --- | --- |
| **The skill + hooks** | You use Claude Code. This is the whole system, and the one `katra setup` installs. |
| **The CLI** | Any agent that can run a shell command. `katra new`, `append`, `capture`, `stamp`. |
| **MCP** | A client that speaks the Model Context Protocol and would rather call a tool than shell out. |

## The problem this solves

An agent that logs its work at the end writes a summary of a diff. That is the
one thing the diff already tells you. What is lost is everything that happened
before the final state: the approach that failed, the measurement that changed
the plan, the screenshot of the bug.

So katra's agent integration is built around a single rule — **the draft exists
while the work does** — and the hooks exist to make that true without the agent
having to remember.

## `katra setup`

```bash
katra setup            # with the commit gate
katra setup --no-gate  # nudges only
```

Idempotent, and safe to re-run — which you should do after upgrading katra,
because it is how a project picks up new hook wiring. It installs the skill into
`.claude/skills/katra/`, merges the hooks into `.claude/settings.json` (leaving
any hooks it does not own alone), installs the git auto-stamp hook, and
registers the project with the [hub](hub).

## The hooks

Seven, all routed through one adapter, `katra agent-hook <event>`. Every one is
**fail-open**: a hook that errors, or a directory that is not a katra, allows
the operation.

| Event | Hook | What it does |
| --- | --- | --- |
| `SessionStart` | `agent-hook session-start` | Rescans memory and prints a one-line status: unresolved memory, in-flight changes needing reconciliation, or the active draft's slug. |
| `UserPromptSubmit` | `agent-hook turn-start` | Records a turn boundary, so a unit of work can be scoped to one turn. |
| `PostToolUse` (`Edit\|Write`) | `agent-hook post-tool` | Records the touched path and incrementally rescans memory. Cheap. |
| `Stop` | `agent-hook stop` | The gate. Blocks the turn ending if authored code changed and nothing declared what it was for. |
| `PreToolUse` (`Bash`) | `agent-hook pre-commit` | The commit gate. Blocks a `git commit` whose staged code has no reconciliation receipt. |
| `PreCompact` | `agent-hook snapshot --event pre-compact` | Snapshots state before the context is compacted. |
| `SessionEnd` | `agent-hook snapshot --event session-end` | The same at teardown. Async, so it never delays exit. |

`agent-hook pre-commit` blocks by exiting **2** — the Claude Code `PreToolUse`
convention for stopping a tool call — not the plain 0/1 every other katra
command uses. See [CLI reference: Exit codes](cli#exit-codes).

### What the Stop gate actually blocks

This is the part worth understanding, because a gate that fires wrongly is worse
than no gate. It blocks only when **all** of these hold:

- The turn authored code changes that are still present in the working tree.
  An edit-then-revert nets to nothing and does not block.
- Those changes are outside the katra directory. Writing an entry is not work
  that needs its own entry.
- Nothing in this turn declared what the work was for.

A purely conversational turn never blocks. Neither does a turn that only
touched files someone else had already dirtied. And a blocked turn never blocks
twice for the same unchanged work — the block records a watermark.

To satisfy it, declare the work — `<task-slug>` comes from `katra task list`
(or create one with `katra task new`):

```bash
katra reconcile --advance <task-slug>   # this moves a task forward
katra reconcile --close   <task-slug>   # this finishes one
katra reconcile --no-task --reason "…"  # this advances no task
katra reconcile --skip    --reason "…"  # bypass just this unit
```

`katra reconcile status` prints what the gate currently wants, which is the
first thing to run when you are blocked and unsure why.

{: .warning }
The commit gate changes your commit flow in every repo you install it in.
That is the intent — it is what stops work landing unlogged — but if you are
trying katra out, `katra setup --no-gate` gives you the nudges without the
block, and you can turn the gate on later by re-running `katra setup`.

## Working from a spec

A task can point at a committed spec. `katra task spec <slug> <ref>` sets
`spec:` on it and moves it from `todo` (or empty) to `specced` — a status
meaning a design exists, committed, and nobody has started building it.
`<ref>` is a node slug in the katra (a decision, an article, an entry) or a
path relative to the repository root; either resolves the same way in every
session, because it is committed rather than remembered.

That matters more for an agent than for a person. A fresh session has no
transcript from the one that wrote the spec — what it has is whatever is in
the repo. The pickup sequence for a specced task:

```bash
katra task list --status specced   # what's designed and ready to build
```

Read the task's `spec:` reference — the file it names, in `katra/decisions/`,
`katra/articles/` or wherever the path points — before writing any code.
Then:

```bash
katra task start <slug>            # -> doing
```

Implement from the spec, not from a re-guess at what the title meant. Log
entries as you go the same as any other work, and let them record where the
build diverged from the plan and why — that comparison is the thing neither
the spec nor the diff can hold on its own.

`katra task spec` writes even when `<ref>` does not yet resolve to a node or a
file — the spec may be authored in the same change — but `katra doctor`
reports a task whose reference never ends up resolving. See
[Design: the spec phase on tasks](design/task-spec-phase) for the full
reasoning behind the status and the resolution rule.

## Memory ingest

Claude Code keeps its own project memory. katra can read it, classify each
generation, and fold the admitted ones into a private ledger so the log gets the
play-by-play without anyone re-typing it.

```bash
katra memory status          # pending / imported / ignored / quarantined
katra memory scan            # pick up anything written since the last scan
katra memory resolve <file>  # fold a generation into the log
katra memory ignore <file>   # mark it not worth logging
```

Only `metadata.type: project` memories are admitted by default — not `user`
(who you are) and not `feedback` (how you like to be worked with), because
neither belongs in a committed log. Anything matching a secret detector or a
configured `sensitiveTerms` entry is quarantined rather than offered. See
[Configuration](configuration#memory-ingest).

The ledger is machine state in `katra/.state/` and is not itself the log. Ingest
is the first of three stages — ingest, author, publish — and only publishing
puts anything in an entry.

## Writing a good entry

The failure mode is not an agent that forgets to log. It is an agent that logs a
flat technical document — a wall of prose that reads like a handoff note to the
next agent — and a log of those is one nobody, including you in a month, ever
reads.

The skill (`internal/cli/embed/SKILL.md`) is the full guidance. Its core:

- **Show the thing.** Treat a draft with zero visuals as unfinished. A
  screenshot, a before/after, a chart of the numbers you just measured.
- **Write the reasoning, not the diff.** The decision and the *why*, the
  alternative you rejected, what broke first. That is what memory paraphrases
  away and what a diff cannot hold.
- **Open with the stakes, not the filename.** "The HUD was eating the city"
  beats "Implemented WorldHUD.cs changes in-place."
- **Land it.** End on what is now true and what is still shaky — a `warning`
  block for the parts that need a device, a person, or a rerun.

Anti-patterns, all observed in real katras: addressing the next agent
("Integrator notes: wire X after Y"), closing on tooling receipts ("Validated
clean: 0 diagnostics"), spec-dumps as the body, and an entry with no visual at
all.

## MCP

`katra-mcp` is a stdio MCP server over the same core operations the CLI drives.

```json
{
  "mcpServers": {
    "katra": { "command": "katra-mcp" }
  }
}
```

It resolves the katra from `$KATRA_DIR` (or the legacy `$DEVLOG_DIR`), falling
back to discovery from the working directory.

| Tool | What it does |
| --- | --- |
| `katra_list` | List entries. |
| `katra_get` | Fetch one node's frontmatter and body. |
| `katra_new` | Start a draft. |
| `katra_append` | Append markdown to a draft. |
| `katra_capture` | Import media and append the right block. |
| `katra_compare` | Import two images and append a compare slider. |
| `katra_stamp` | Stamp a draft with commit hashes and diffstat. |
| `katra_nodes` | List nodes of any type. |
| `katra_task_new` | Create a task. |
| `katra_task_list` | List tasks. |
| `katra_task_set_status` | Move a task between `todo`/`doing`/`done`/`cut`. |
| `katra_epic_new` | Create an epic. |
| `katra_decide` | Record a decision. |
| `katra_article_new` | Create an article. |

The operations live in `internal/core`, so the MCP server and the CLI cannot
drift apart in behaviour — only in surface. If a tool is missing here, the
corresponding core operation exists and wiring it up is a small change.

## Without Claude Code

Nothing above is required. Any agent that can run a shell command can keep a
katra with four of them:

```bash
katra new "What you are about to do" --tags area,kind
katra capture screenshot.png --caption "after the fix"
katra append "Why X over Y, and what broke first."
katra stamp
```

Install the git hook (`katra hook install`) and the last one happens on its own.
