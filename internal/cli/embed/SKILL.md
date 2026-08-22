---
name: katra
description: Chronicle development work in a committed, rich-component dev log as you build. Use when starting a piece of work that should be logged, when capturing a screenshot/gif/render into the log, when adding a before/after comparison, or at commit time to stamp the active draft with its hash + diffstat. Keywords - katra, dev log, changelog, capture screenshot, log this work, stamp the entry.
---

# katra

Keep a living dev log while you work, using the `katra` CLI (or the
`katra-mcp` MCP tools, if connected). The log is markdown entries with embedded
rich components, served as a live page and committed alongside the code.

The harness-neutral workflow is the public contract:
<https://craigjmidwinter.github.io/katra/workflow>. This skill is a Claude Code
delivery mechanism for the same task/epic/spec/entry/decision/stamp loop; it
does not define behavior unavailable to a plain CLI or another coding harness.

## The model (read this first)

- A katra lives in `katra/` at the repo root: `entries/` (one markdown file per
  post) + `media/` + `config.yml`. Run `katra list` to see if one exists; if not,
  `katra init`.
- **A draft is an entry with no commit hash.** It renders in the "In Progress"
  panel immediately — there is no scratch file and no promote step. The draft is
  the running post; you *stamp* it at commit time (hash + diffstat) and it drops
  into the log. Nothing can get stuck.

## Specs come before drafts

Before starting a task, check whether it is `specced`
(`katra task list --status specced`). That means a design is already
committed — its `spec:` frontmatter is a node slug in the katra, or a path
relative to the repo root. Read that file before writing any code; it exists
so you do not have to re-derive intent from a conversation that will not be
there next session.

If you are the one writing the design, not just building from one already
committed, and the task deserves it: write it (a decision, an article, or any
other markdown), then attach it —

```
katra task spec <task-slug> <ref>
```

— and commit the artifact plus task pointer as one planning change. This moves
the task `todo`/empty → `specced` and leaves a further-along status alone. Then
`katra task start <task-slug>` and implement from the committed spec.
Keep logging entries as you go, the same as any other work — and let them say
where the build diverged from the plan and why. That comparison is what makes
the entry worth more than the spec alone.

## You are writing a post, not a report

This is the part that goes wrong. Entries drift into flat technical documents —
a wall of prose that reads like a handoff note to the next agent — and a log of
those is one nobody, including you in a month, ever reads.

Claude Code project-memory may already capture the play-by-play, and Katra's
optional Claude adapter can offer that memory for deliberate authoring. It
never copies memory into the public log automatically. In Claude sessions,
don't re-type the transcript; in every harness, explicitly append the reasoning
worth publishing. Spend your effort on the two things a transcript can't hold:

- **Show the thing.** Screenshots, renders, before/after sliders, diagrams,
  charts. A single capture is worth paragraphs.
- **The reasoning.** The decision and the *why*, the alternative you rejected,
  what broke first. That's the signal memory paraphrases away.

Write it the way you'd write it up for someone who wasn't there:

- **Open with the stakes, not the filename.** "The HUD was eating the city" beats
  "Implemented WorldHUD.cs changes in-place."
- **Name the thing you rejected**, and why it lost. A post with no discarded
  alternative reads as though the answer was obvious, which it wasn't.
- **Land it.** End on what's now true and what's still shaky — a `warning` block
  for the parts that need a device, a person, or a rerun to confirm.

Anti-patterns, all observed in real katras:

- Addressing the next agent instead of a reader ("Integrator notes: wire X after
  Y", "Do NOT trigger a full compile"). Put coordination in the task/memory; the
  entry is for the reader.
- Closing on tooling receipts ("Validated clean via ValidateScript: 0
  diagnostics"). Evidence is good — bury it mid-post, don't end on it.
- Invariant checklists and spec-dumps as the *body*. Summarize the guarantee in
  a sentence; link or fold the exhaustive list into a collapsible tail if you
  must keep it.
- An entry with no visual at all (see below).

## Every entry ships at least one visual

Treat a draft with zero visuals as unfinished. If you're about to stamp one, stop
and ask what could be *shown* instead of described. In order of preference:

1. **It has a UI, a render, or a scene** → `katra capture` a screenshot or gif.
   Changed something visible? `katra compare before.png after.png`.
2. **It generates or transforms structured output** — a map, a graph, a layout, a
   schedule, a parse tree → render one and capture it. A map generator's post
   should contain a picture of a map.
3. **It has numbers** — frame times, token counts, latencies, file sizes, pass
   rates, before/after benchmarks → chart them (recipe below).
4. **It's pure architecture** → a diagram. A small hand-authored SVG or an HTML
   figure showing the pieces and the arrows beats three paragraphs describing the
   same boxes.

Only when none of those apply is a text-only entry the right call — and that's
rarer than it feels in the moment.

### Charts and diagrams: author HTML, then capture it

There's no chart component; you build the figure yourself and embed it, which
means any chart you can draw is available, not a fixed set of types.

Write a **self-contained** `.html` file to a scratch path, then capture it —
`katra capture` recognizes `.html` and emits an `embed` block automatically:

```
katra capture /tmp/frame-times.html --caption "Frame time vs district count, Quest 3 vs Editor"
```

Rules for the HTML, because it renders inside a sandboxed iframe:

- **No external requests.** No CDN scripts, no web fonts, no remote images.
  Inline everything; embed images as `data:` URIs. A `<script>` tag with inline
  JS is fine, but plain inline SVG is usually less work and always renders.
- **Size it for the column.** Set an explicit `viewBox`, let it scale to 100%
  width, and pass `height:` in the embed block if 480px is wrong.
- **Give the figure its own background.** `prefers-color-scheme` follows the
  *OS*, not the viewer's theme, and the iframe can't see its host — so a
  transparent figure will eventually render dark ink on a light page. Set an
  explicit `background` in both branches of your
  `@media (prefers-color-scheme: dark)` so ink and surface always agree.
- **Label the axes and the units.** An unlabeled chart is decoration.

A bar chart is ~20 lines of inline SVG — `<rect>` per bar, `<text>` per label,
one `<line>` for the baseline. Don't reach for a library.

## Workflow

1. **Starting a chunk of work** → create a draft:
   ```
   katra new "What you're about to do" --tags area,kind
   ```
   The title is a headline; write it like one.

2. **As you work** → capture assets and reasoning as they happen:
   ```
   katra capture path/to/screenshot.png --caption "after the fix"
   katra capture path/to/animation.gif --caption "the horde, dispatched"
   katra capture /tmp/bench.html --caption "p95 latency, before vs after"
   katra compare before.png after.png --caption "bunker reshape"
   katra append "Why we went with X over Y, and what broke first."
   ```
   `append` also reads stdin (`--file -`) for longer markdown. Capture handles
   images, gifs, video, and interactive `.html` artifacts automatically. Reach
   for capture/compare the moment there's something visual — the screenshot you
   don't take now is gone by the time you're writing it up.

3. **Rich components** — embed them in `append`ed markdown as fenced blocks:
   `embed` (sandboxed iframe artifact), `compare` (before/after slider),
   `gallery` (image grid), `video`, `note`, `warning`. Plain markdown images get
   a lightbox. Example:
   ````
   katra append $'```note\nThis needs a rebuild to show in the headset.\n```'
   ````

4. **At commit time** → stamp the active draft:
   ```
   katra stamp                 # uses HEAD + computes the diffstat
   katra stamp --hash a,b,c    # a chapter of several small commits
   ```
   If the post-commit hook is installed (`katra hook install`), this happens
   automatically after each commit — you only need to commit the stamped file.

5. **Review** → `katra serve` (live LAN page, auto-reloads) or `katra build`
   for a static site. `katra doctor` flags dangling media, parse errors, entries
   that shipped with no visual, and epics whose status has gone stale.

## Notes

- Claude memory ingest is optional and private. Resolve admitted memory only
  after deliberately authoring what matters into the draft. Codex, MCP, and
  plain-shell sessions use `katra append` directly and lose no Katra behavior.
- One draft at a time is the happy path; `stamp` targets the newest draft. Use
  `--entry <slug>` to target a specific one.
- If the MCP server is connected, it exposes all 15 operations: entry tools
  `katra_list`, `katra_get`, `katra_new`, `katra_append`, `katra_capture`,
  `katra_compare`, `katra_stamp`; and node tools `katra_nodes`,
  `katra_task_new`, `katra_task_list`, `katra_task_set_status`,
  `katra_task_spec`, `katra_epic_new`, `katra_decide`, `katra_article_new`.
