---
title: The hub
layout: default
nav_order: 8
description: >-
  One page across every katra on your machine — the registry, the cross-project
  board and roadmap, running the hub as a login-time daemon, and the static
  aggregate build.
---

# The hub

A katra is per-repository, which is right for the log and wrong for the
question "what have I been doing?". The hub serves every registered katra from
one URL.

```bash
katra hub serve            # http://localhost:4200
```

![The hub's Projects view: a grid of in-flight tasks and draft entries pulled from 14 separate repositories, each card labelled with its own project]({{ "/assets/screenshots/hub-projects.png" | relative_url }})

That is the maintainer's own hub — 14 registered projects, 25 things in
flight, none of it re-typed anywhere. Every card is a real task or draft entry
still open in its own repository's katra.

## The registry

A machine-global list of store directories at
`$XDG_CONFIG_HOME/katra/registry.yml` (or `~/.config/katra/registry.yml`), so
the hub finds every project without scanning your disk.

```yaml
projects:
  - /Users/you/work/game/katra
  - /Users/you/work/api/katra
```

`katra init` and `katra setup` register a project automatically. Otherwise:

```bash
katra hub scan ~/work      # find and register every katra under a root
katra hub list             # list them, dropping any that no longer exist
```

`hub list` prunes as a side effect — an entry whose `config.yml` has gone is
removed and the file is rewritten. So is a temp directory some other tool
registered and then deleted. Pruning is how the registry stays honest without a
separate garbage-collection step.

`KATRA_REGISTRY` overrides the path, which is mostly useful for tests.

## What it serves

| Path | What |
| --- | --- |
| `/` | The project index — every katra, with its accent, entry count and drafts. |
| `/board` | A cross-project board: what is in flight everywhere. |
| `/roadmap` | Tasks and epics across every project. |
| `/log` | One merged, chronological log. |
| `/p/<id>/` | One project's full viewer, exactly as `katra serve` renders it. |
| `/api/hub.json` | A portfolio snapshot as data, for native clients. |

Project ids are derived from the repository name and are stable, so a bookmark
into `/p/<id>/` keeps working. A collision gets a numeric suffix.

Livereload is global: a change in any store reloads every open tab.

## Running it at login

```bash
katra hub install        # macOS: a launchd agent on port 4200
katra hub uninstall
```

`contrib/launchd/com.katra.hub.plist` in the repository is the same agent, if
you would rather install it by hand or adapt it for systemd.

The agent runs `katra hub serve` with `KeepAlive`, so it restarts if it dies and
starts at login. It resolves `katra` by **absolute path**, which means a
rebuild does not change what is running:

```bash
make install
launchctl kickstart -k "gui/$(id -u)/com.katra.hub"
```

{: .note }
The hub re-reads the registry every 10 seconds while it runs, so a katra you
create later appears on the board on its own. It did not always: before that,
the daemon snapshotted the registry at startup and a project registered after
login stayed invisible until the machine was rebooted. If you are on an older
build and a new project is missing, that is the bug, and restarting the hub is
the workaround.

A registry that is briefly unreadable — caught mid-write — leaves the previous
project set in place rather than emptying the board.

## The static aggregate

```bash
katra build --all --out ./site
```

One self-contained directory containing every registered katra, with the same
index, board, roadmap and per-project views the daemon serves. No external
requests, so it hosts anywhere: GitHub Pages, an S3 bucket, a USB stick.

This is the sharing story. There is no hosted service, no account, and no
sync — the aggregate is a build artifact you own.

## Per-project, without the hub

`katra serve` from inside a repository serves that one project on port 8080,
with the same viewer. The hub is additive; nothing depends on it, and a project
that is not registered still works completely on its own.
