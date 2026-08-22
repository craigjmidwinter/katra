---
title: Quickstart
layout: default
nav_order: 2
description: >-
  Install katra, scaffold a log in a repo, write and stamp your first entry,
  and serve the live page — about five minutes, no accounts, no services.
---

# Quickstart

About five minutes. No account, no service, nothing leaves your machine.

## 1. Install

With Go 1.25+ — the primary channel, and the only one that works before the
first release ships:

```bash
go install github.com/craigjmidwinter/katra/cmd/katra@latest
go install github.com/craigjmidwinter/katra/cmd/katra-mcp@latest
```

Both binaries matter. `katra-mcp` is what an MCP client talks to, and the
published workflow treats CLI and MCP as two surfaces over the same core.

{: .note }
A `go install` build reports `dev` for `katra --version`, because the version is
stamped at link time and the `go` tool does not do it. Released binaries and
`make build` report the real tag.

Once a release exists (v0.1.0 — see [RELEASING.md](https://github.com/craigjmidwinter/katra/blob/main/RELEASING.md)
in the repo), Homebrew is the other option:

```bash
brew install craigjmidwinter/tap/katra
```

[The full Install section](https://github.com/craigjmidwinter/katra#install)
in the README also covers building from source, downloading a binary directly,
upgrading, and uninstalling.

## 2. Set up a repo

From inside a git repository:

```bash
katra init --install-hook
```

That does two harness-neutral things:

1. Creates `katra/` if it is not there — `config.yml`, `entries/`, `media/`.
2. Installs the portable Git `post-commit` auto-stamp hook.

`katra init` also registers the project with the [hub](hub). For an existing
Katra, `katra hook install` adds only the portable hook.

Claude Code users may instead run `katra setup`, which adds its skill, seven
session hooks, memory adapter, and optional commit gate on top of the same
store and Git hook. Those integrations are optional; see [Agents](agents).

{: .warning }
Claude's `katra setup` installs a **blocking commit gate** — a hook that refuses a
`git commit` whose staged code has not been reconciled against a task. That is
the point of it, but it is a real change to your commit flow. Use
`katra setup --no-gate` for the nudges without the block. See [Agents](agents).

## 3. Write the welcome draft

`katra init` creates `hello-katra`, a real welcome draft so the first page has
something visible. It has no `hash:`, so it is already in the *In Progress*
panel. Replace its starter text in an editor, or append from the shell:

```bash
katra append --entry hello-katra "The first reason this project needs a chronicle."
```

After this first success, `katra new "A real headline" --tags area,kind`
starts each new entry.

## 4. Show the thing

An entry with no visual is the most common way a log stops being read. Capture
one — optional for this walkthrough, since it needs a file that exists on
*your* machine, but worth doing for real once you have something to show.
Swap in any image, gif or `.html` chart you already have (`screencapture -x
shot.png` grabs one on macOS if you don't):

```bash
katra capture ~/Desktop/swing.png --entry hello-katra --caption "first visible proof"
katra compare before.png after.png --entry hello-katra --caption "before and after"
```

`capture` copies the file into `katra/media/` and appends the right block for
its type — image, video, or, for an `.html` file, an `embed`. See
[Components](components) for everything an entry can hold, including charts
(you author a self-contained HTML file and capture it).

## 5. Watch it live

```bash
katra serve
```

Serves on `http://localhost:8080` and on your LAN address, and reloads open tabs
whenever a file under `katra/` changes. Leave it running in a split while you
write.

## 6. Commit

```bash
git add -A
git commit -m "swing arc rework"
```

The `post-commit` hook stamps the active draft with that commit's hash and
diffstat. The entry drops out of *In Progress* and into the log. The stamp is
left as a working-tree change for you to include in your next commit; set
`autoCommit: true` in `katra/config.yml` to have the hook commit it itself.

Without the hook, do it by hand:

```bash
katra stamp                    # HEAD
katra stamp --hash a1b2c3,d4e5f6   # a chapter of several commits
```

## 7. Publish, if you want to

```bash
katra build --out ./site
```

A self-contained directory — `index.html`, `data.json`, and your media. No
build step, no external requests, no server. Open it from a USB stick, or point
GitHub Pages at it.

## Where to go next

- [Components](components) — everything an entry can embed.
- [The Katra workflow](workflow) — tasks, epics, specs, decisions, entries,
  closure, and automation in any harness.
- [Agents](agents) — hand the log to an agent so it writes from a committed
  spec instead of a conversation, and records how the work actually went.
- [The hub](hub) — one page across every katra on your machine.
