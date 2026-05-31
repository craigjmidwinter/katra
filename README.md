# devlog

A committed, rich-component **dev log you write as you build** — markdown
entries with embedded interactive components, stamped automatically with the
commit + diffstat they describe, served as a live, auto-reloading page.

It exists to make one workflow reliable: *chronicle the work as it happens —
the why, the dead ends, the screenshots and animations — and never lose a draft
to a "promote" step that gets skipped.*

```
devlog init                       # scaffold devlog/ in your repo
devlog new "Reworked the swing"   # start a draft (a markdown file)
devlog capture shot.png           # drop a screenshot into the current draft
devlog compare before.png after.png
devlog serve                      # live page on the LAN, reloads as you write
# … you commit your code …
devlog stamp                      # draft → logged, with hash + diffstat
```

## Why it's built this way

- **One markdown file per entry** (`devlog/entries/2026-05-31-slug.md`), with
  YAML frontmatter. No single giant file to corrupt or fight merge conflicts in.
- **A draft is just an entry with no commit hash.** It shows up immediately in
  the **In Progress** panel — there's no scratch file and no separate "promote"
  step to forget. Stamping it (adding the hash + diffstat) drops it into the log.
- **Rich components live in plain markdown** as fenced blocks, so the source
  stays readable and diff-able but the page can show sliders, embeds, galleries.
- **Rendered in Go, viewed as static HTML.** The viewer needs no build step and
  works offline; markdown + components are pre-rendered into `data.json`.

## Storage layout

```
devlog/
  config.yml        # title, accent colour, hook behaviour
  entries/          # one .md per post (YAML frontmatter + markdown body)
  media/            # images, gifs, video, html embeds
```

Frontmatter:

```yaml
---
title: Reworked the swing arc
date: "2026-05-31"
tags: [physics, gameplay]
hash: 5ddc0f5          # or  hashes: [a, b]  for a chapter of commits
stat: {f: 12, a: 340, d: 50}
summary: tuned the magnus model      # optional, for the index
featured: true                       # optional → "Deep Dives" zone
cover: media/hero.png                # optional banner image
---

Markdown body. Write the *why*, not a paraphrased diff.
```

## Rich components

Fence a block with a registered language. Unknown languages render as ordinary
code blocks.

````markdown
```compare
before: media/bunker_before.png
after:  media/bunker_after.png
caption: Bunker reshape
```

```embed
src: media/clubhouse_360.html
height: 480
caption: 360° viewer (interactive)
```

```gallery
- src: media/a.png
  cap: tier one
- src: media/b.png
  cap: tier two
```

```video
src: media/horde.mp4
loop: true
```

```note
A callout. **Markdown** works inside it.
```

```warning
Same, with a warning style.
```
````

Plain `![caption](media/x.png)` images get a lightbox for free. Adding a new
component is one `ComponentFunc` in `internal/core/render.go` — that's the whole
extension surface.

## Commands

| Command | What it does |
|---|---|
| `devlog init [--title T] [--install-hook]` | Scaffold a devlog in the repo |
| `devlog new "Title" [--tags a,b] [--featured]` | Start a draft entry |
| `devlog append [text] [--entry slug] [--file -]` | Append markdown to a draft |
| `devlog capture <file> [--caption C]` | Import media into the active draft |
| `devlog compare <before> <after>` | Add a before/after slider |
| `devlog stamp [--hash H…] [--commit]` | Stamp the draft with commit + diffstat |
| `devlog list [--drafts] [--json]` | List entries |
| `devlog serve [--port N]` | Live, auto-reloading page on the LAN |
| `devlog build [--out dir]` | Build a static site (GitHub Pages, etc.) |
| `devlog hook install \| uninstall` | Manage the auto-stamp git hook |
| `devlog doctor` | Find dangling media + parse errors |

## Git integration

`devlog stamp` reads `git` to resolve the hash and compute the diffstat
(`--numstat`). For a chapter of small commits: `devlog stamp --hash a,b,c`.

The optional **post-commit hook** removes the "forgot to stamp" failure mode
entirely:

```
devlog hook install
```

After each commit it stamps the active draft with that commit. It skips its own
bookkeeping commits and commits that only touch the devlog. By default the stamp
is left as a working-tree change for you to commit; set `autoCommit: true` in
`config.yml` to have it commit the stamp itself.

## Agent integration

Two surfaces let an agent drive the log while it works:

- **CLI** — agents shell out to `devlog new/append/capture/compare/stamp`.
- **MCP server** — `devlog-mcp` exposes `devlog_list`, `devlog_get`,
  `devlog_new`, `devlog_append`, `devlog_capture`, `devlog_compare`,
  `devlog_stamp` over stdio. Add to `.mcp.json`:

  ```json
  {
    "mcpServers": {
      "devlog": { "command": "devlog-mcp" }
    }
  }
  ```

  It resolves the devlog from `$DEVLOG_DIR` or the working directory.

A **Claude Code skill** is in [`skill/`](skill/SKILL.md); symlink it into your
skills directory to invoke the workflow as `/devlog`.

## Install

```
go install github.com/craigjmidwinter/devlog/cmd/devlog@latest
go install github.com/craigjmidwinter/devlog/cmd/devlog-mcp@latest
```

Or from a clone:

```
go build -o ~/bin/devlog ./cmd/devlog
go build -o ~/bin/devlog-mcp ./cmd/devlog-mcp
```

## License

MIT
