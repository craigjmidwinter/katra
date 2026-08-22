---
title: Home
layout: default
nav_order: 1
description: >-
  katra is a committed, rich-component dev log you write as you build — markdown
  entries with embedded interactive components, stamped automatically with the
  commit and diffstat they describe, and served as a live, auto-reloading page.
permalink: /
---

# katra

**A committed dev log you write as you build — and the memory that makes
spec-driven agentic development work.**

For a developer, solo or pairing with an agent, who wants the log to hold the
*why* — the approach that failed, the screenshot of the thing working — not a
paraphrase of the diff. For an agent implementing from a committed spec, it is
where that work gets recorded as it happens, in the same repo the spec lives
in, so the next session has something durable to pick up.

Entries are markdown files in your repo. They can embed before/after sliders,
galleries, video, callouts and arbitrary self-contained HTML. When you commit,
the entry you were writing is stamped with that commit's hash and diffstat and
drops into the log. A live page serves the whole thing on your LAN and reloads
as you type.

```bash
katra init --install-hook        # store + portable Git auto-stamp
katra new "Reworked the swing"   # start a draft
katra capture shot.png           # drop a screenshot into it
katra serve                      # live page, reloads as you write
git commit -m "…"                # the draft is stamped with this commit
```

[Install](quickstart){: .btn .btn-primary .mr-2 }
[Quickstart](quickstart){: .btn .mr-2 }
[Source on GitHub](https://github.com/craigjmidwinter/katra){: .btn }

## The one idea

**A draft is an entry with no commit hash.** That is the entire state machine.

It shows up in the *In Progress* panel the moment you create it — there is no
scratch file, no separate document, and no "promote" step. Stamping it adds the
hash and the diffstat, which is what moves it into the log. Nothing can get
stuck in a buffer you forgot to publish, because there is no buffer.

The corollary is that the log writes itself in the order you actually worked,
including the parts that did not pan out — which is the half a squashed commit
history always loses.

![The Overview page: a Future → Now → Past spine, the current draft sitting at Now awaiting a stamp, and load-bearing decisions pulled into the right rail]({{ "/assets/screenshots/viewer-overview.png" | relative_url }})

`katra serve` renders that spine as a live page on your LAN, reloading as you
write — no build step in between.

## Where to start

| If you want to | Read |
| --- | --- |
| Get it running in five minutes | [Quickstart](quickstart) |
| Follow the complete spec-to-stamp loop in any coding harness | [The Katra workflow](workflow) |
| See what an entry can contain | [Components](components) |
| Look up a command or a flag | [CLI reference](cli) |
| Change the title, accent, or hook behaviour | [Configuration](configuration) |
| Write a tool that reads a katra | [On-disk format](format) |
| Have an agent implement from a committed spec, and keep the log for you | [Agents](agents) |
| Read every project's log from one page | [The hub](hub) |
| Change katra itself | [Architecture](architecture) |

{: .note }
This is [Diátaxis](https://diataxis.fr)-shaped but not Diátaxis-sized: a
project this small folds its how-to recipes into the tutorial
([Quickstart](quickstart)) and the workflow reference ([The Katra workflow](workflow), [Agents](agents),
[The hub](hub)) rather than maintaining a separate how-to section. Reference
([CLI](cli), [Configuration](configuration), [Components](components),
[On-disk format](format)) and explanation ([Architecture](architecture),
[Design notes](architecture#design-notes)) stay split out.

## What it is not

- **Not a blog engine.** There is no theme system, no plugin API, no feed. The
  static build is a single self-contained directory so you can host it anywhere,
  and that is the whole publishing story.
- **Not a changelog generator.** It does not summarize your diff. It exists for
  the part a diff cannot hold — the reasoning, the alternative you rejected, the
  screenshot of the thing working.
- **Not a service.** Nothing is stored anywhere but your repo. Uninstall it and
  every entry is still a readable markdown file.

## License

MIT. Full source at [github.com/craigjmidwinter/katra](https://github.com/craigjmidwinter/katra).
