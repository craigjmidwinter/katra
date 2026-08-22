---
title: CLI reference
layout: default
nav_order: 4
description: >-
  Every katra command and flag — creating and stamping entries, capturing
  media, serving and building, the node model, and the agent-facing commands.
---

# CLI reference

Every command resolves the katra by looking for a `katra/` directory (or the
legacy `devlog/`) at the git root, unless `$KATRA_DIR` names one explicitly.
The commands that come up most, in the order you would actually run them:

```bash
katra init                       # create katra/ in this repo
katra new "Reworked the swing"   # start a draft
katra append "Why X over Y."     # write into it
katra capture shot.png           # drop a screenshot into it
katra stamp                      # attach HEAD's hash + diffstat, publish it
katra serve                      # live page at http://localhost:8080
```

Everything below is the full reference: every command, every flag.

## Setting up

### `katra init`

Create a katra in this repository.

| Flag | Meaning |
| --- | --- |
| `--title T` | Log title. Defaults to the repository name. |
| `--here` | Create it in the current directory rather than the repo root. |
| `--install-hook` | Also install the `post-commit` auto-stamp hook. |

### `katra setup`

Install the Claude Code integration on this repo: store, skill, Claude Code
hooks, portable Git auto-stamp, and hub registration. **Idempotent** — re-run
it after upgrading katra to pick up new hook wiring. Other harnesses use
`katra init --install-hook` or `katra hook install` instead.

| Flag | Meaning |
| --- | --- |
| `--no-gate` | Install the session nudges but not the blocking commit gate. |

See [Agents](agents) for what each hook does and what the gate blocks.

## Writing

### `katra new "Title"`

Start a draft entry.

| Flag | Meaning |
| --- | --- |
| `--tags a,b` | Comma-separated tags. |
| `--summary S` | One-line summary for the index. |
| `--body M` | Initial body markdown. |
| `--date YYYY-MM-DD` | Entry date. Defaults to today. |
| `--featured` | Mark as a Deep Dive. |

The title is a headline — write it like one. "The HUD was eating the city"
beats "Implemented WorldHUD.cs changes".

### `katra append [text...]`

Append markdown to a draft. Takes text arguments, `--file`, or stdin.

| Flag | Meaning |
| --- | --- |
| `--entry SLUG` | Target entry. Defaults to the active draft. |
| `--file F` | Read markdown from a file; `-` for stdin. |

### `katra capture <file>`

Copy an image, gif, video or HTML artifact into `media/` and append the right
block to the active draft.

| Flag | Meaning |
| --- | --- |
| `--caption C` | Caption for the media. |
| `--entry SLUG` | Target entry. Defaults to the active draft. |
| `--name N` | Store under this filename. |
| `--as KIND` | Force `image`, `video` or `embed`. |
| `--no-append` | Import only; do not add it to an entry. |

### `katra compare <before> <after>`

Import two images and append a before/after slider.

| Flag | Meaning |
| --- | --- |
| `--caption C` | |
| `--entry SLUG` | |
| `--no-import` | The paths already point inside `media/`; do not copy. |

## Stamping

### `katra stamp`

Stamp the active draft with one or more commit hashes and the computed
diffstat, moving it from *In Progress* into the log. Defaults to `HEAD`.

| Flag | Meaning |
| --- | --- |
| `--hash a,b,c` | Commit hash(es). Repeat or comma-separate for a chapter. |
| `--entry SLUG` | Target entry. Defaults to the active draft. |
| `--closes SLUG` | Task slug(s) this entry completes — marks them done and links them. |
| `--commit` | `git add` + commit the stamped entry. |

### `katra hook install | uninstall`

Manage the git `post-commit` auto-stamp hook. After each commit it stamps the
active draft with that commit, skipping its own stamp commits and commits that
only touch the katra.

The hook honours `core.hooksPath`. Under husky it installs to the tracked
`.husky/post-commit`, not the generated `.husky/_/`, which husky regenerates on
every `npm install`.

### `katra check`

Exit **1** if code is staged with no *written* draft — no draft at all, or
one whose body is still `katra new`'s "Start writing here." placeholder, which
records nothing — else exit **0**. Intended for commit-gate hooks; `--quiet`
suppresses the message. It fails open: any uncertainty (no store, a git error,
nothing staged, only the katra staged) exits 0, so a katra bug can never
strand a commit.

## Reading and publishing

### `katra list`

List entries, newest first.

| Flag | Meaning |
| --- | --- |
| `--drafts` | Only unstamped drafts. |
| `--json` | Machine-readable output. |

### `katra serve`

Serve the live, auto-reloading page on the LAN.

| Flag | Meaning |
| --- | --- |
| `--port N` | Defaults to `8080`. |

### `katra build`

Build a static site — `index.html`, `data.json` and media — into a
self-contained directory with no external requests.

| Flag | Meaning |
| --- | --- |
| `--out DIR` | Output directory. Defaults to `dist`. |
| `--all` | Build one aggregate site of every registered katra. |

### `katra doctor`

Check the katra for problems: dangling media references and entries that fail
to parse. Exits **1** when it finds something, else exits **0**.

It also reports things that are not broken, only worth knowing, as warnings that
do not affect the exit code:

- published entries carrying no visual (a count, the coverage percentage, and
  the first few offenders by name)
- epics whose stored status disagrees with the status their child tasks
  compute — fix with `katra epic rollup --write`
- a task's `spec:` reference that resolves to neither a node nor a file
- more than one draft open, and unreferenced files in `media/`

## The node model

{: .warning }
The installed v0.1.0 CLI predates `task spec`, `task new --spec`, and the
`specced` task-list help value. They are available in current source and are
gated against release artifacts for the next tag.

An entry is one kind of node. Tasks, epics, decisions and articles are the
others; they share the frontmatter schema and the `[[wikilink]]` graph. See
[On-disk format](format).

### `katra task new "Title"`

Create a task (status `todo`, or `specced` if `--spec` is given).

| Flag | Meaning |
| --- | --- |
| `--spec REF` | Spec artifact ref (node slug, or a path relative to the repo root); creates the task already `specced`. |
| `--effort S\|M\|L` | Effort estimate. |
| `--epic SLUG` | Parent epic slug. |
| `--tags a,b` | Comma-separated tags. |
| `--summary S` | One-line summary. |
| `--body M` | Initial body markdown. |

### `katra task spec SLUG REF`

Point an existing task at a committed spec. `todo`/empty → `specced`;
`doing`/`done`/`cut` is left alone.

### `katra task start SLUG`

Mark it `doing`. Legal from `specced`.

### `katra task done SLUG`

Mark it `done`.

### `katra task list`

List tasks, newest first.

| Flag | Meaning |
| --- | --- |
| `--status S` | Filter by status — `todo`, `specced`, `doing`, `done`, `cut`. Comma-separate for more than one. |

### `katra epic new "Title"`

Create an epic (status `planned`).

| Flag | Meaning |
| --- | --- |
| `--horizon now\|next\|later` | Planning horizon. |
| `--tags a,b` | Comma-separated tags. |
| `--summary S` | One-line summary. |
| `--body M` | Initial body markdown. |

### `katra epic rollup [slug]`

Show each epic's status computed from its child tasks. A `specced`-only epic
counts as not started, same as `planned`.

| Flag | Meaning |
| --- | --- |
| `--write` | Apply the computed status instead of only displaying it. |

### `katra decide "Title"`

Record a decision (status `accepted`).

| Flag | Meaning |
| --- | --- |
| `--entry SLUG` | Entry slug that occasioned this decision. |
| `--supersedes a,b` | Slug(s) of decision(s) this replaces. |
| `--tags a,b` | Comma-separated tags. |
| `--summary S` | One-line summary. |
| `--body M` | Initial body markdown. |

### `katra article new "Title"`

Longer-form writing that is not tied to a commit.

| Flag | Meaning |
| --- | --- |
| `--tags a,b` | Comma-separated tags. |
| `--summary S` | One-line summary. |
| `--body M` | Initial body markdown. |

`REF`, on `task spec` and `task new --spec`, resolves first against a node
slug in the katra (a decision, article or entry — the same rule as a
`[[wikilink]]`), then against a path relative to the **repository root**. A
ref that resolves to neither still writes — the spec may be authored in the
same change — and shows up in `katra doctor` instead.

## Agent-facing commands

These exist for the [hooks](agents) to call. You can run them by hand, and
`reconcile status` in particular is useful when the commit gate has blocked you.

| Command | What it does |
| --- | --- |
| `katra reconcile status` | Print what the gate currently wants. |
| `katra reconcile --advance SLUG` | This work advances a task (`todo` → `doing`). |
| `katra reconcile --close SLUG` | This work closes a task, applied at publish. |
| `katra reconcile --no-task --reason R` | This work advances no task. |
| `katra reconcile --skip --reason R` | Resolve just this unit of work. |
| `katra memory scan` | Scan Claude Code memory and update the ledger. |
| `katra memory status` | List ledger generations. |
| `katra memory ignore <file>` | Mark a memory generation ignored. |
| `katra memory resolve <file>` | Resolve a generation into the log. |
| `katra agent-hook <event>` | The Claude Code hook adapter. Not for humans. |
| `katra guard` | Legacy PreToolUse guard, superseded by `agent-hook pre-commit`. |

## Across projects

| Command | What it does |
| --- | --- |
| `katra hub list` | List every registered katra, pruning ones that no longer exist. |
| `katra hub scan [root...]` | Find and register every katra under a root. |
| `katra hub serve [--port N]` | Serve them all from one URL. Defaults to `4200`. |
| `katra hub install` | Install a launchd agent so the hub runs at login (macOS). |
| `katra hub uninstall` | Remove it. |

See [The hub](hub).

## Environment

| Variable | Meaning |
| --- | --- |
| `KATRA_DIR` | Explicit path to the store directory, overriding discovery. |
| `DEVLOG_DIR` | The pre-rename name, still honoured. |
| `KATRA_REGISTRY` | Path to the hub registry. Defaults to `$XDG_CONFIG_HOME/katra/registry.yml`. |

## Exit codes

Contract, for anything scripting against the CLI:

| Code | Meaning |
| --- | --- |
| `0` | Success, or — for `katra check` and `katra doctor` specifically — nothing found to block. |
| `1` | Any command error (a bad flag, no store found, a git error) — and the deliberate case, `katra check`/`katra doctor` finding a problem. |
| `2` | `katra guard` and `katra agent-hook pre-commit` only, when the commit gate blocks. This is a Claude Code `PreToolUse` convention — exit 2 is what stops the tool call — not a general CLI exit code. |

Every other command follows the plain Unix convention: 0 on success, 1 on any
error, with the error printed to stderr.
