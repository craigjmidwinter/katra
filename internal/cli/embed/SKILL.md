---
name: katra
description: Chronicle development work in a committed, rich-component dev log as you build. Use when starting a piece of work that should be logged, when capturing a screenshot/gif/render into the log, when adding a before/after comparison, or at commit time to stamp the active draft with its hash + diffstat. Keywords - katra, dev log, changelog, capture screenshot, log this work, stamp the entry.
---

# katra

Keep a living dev log while you work, using the `katra` CLI (or the
`katra-mcp` MCP tools, if connected). The log is markdown entries with embedded
rich components, served as a live page and committed alongside the code.

## The model (read this first)

- A katra lives in `katra/` at the repo root: `entries/` (one markdown file per
  post) + `media/` + `config.yml`. Run `katra list` to see if one exists; if not,
  `katra init`.
- **A draft is an entry with no commit hash.** It renders in the "In Progress"
  panel immediately — there is no scratch file and no promote step. You write the
  draft as you work, then *stamp* it at commit time (hash + diffstat) and it drops
  into the log. Nothing can get stuck.

## Workflow

1. **Starting a chunk of work** → create a draft:
   ```
   katra new "What you're about to do" --tags area,kind
   ```
   Write for a reader: the *why*, the alternative you rejected, the evidence —
   not a paraphrased diff. Be candid about dead ends; that's the point.

2. **As you work** → keep extending the draft:
   ```
   katra append "A paragraph of markdown about what just happened."
   katra capture path/to/screenshot.png --caption "after the fix"
   katra capture path/to/animation.gif --caption "the horde, dispatched"
   katra compare before.png after.png --caption "bunker reshape"
   ```
   `append` also reads stdin (`--file -`) for longer markdown. Capture handles
   images, gifs, video, and interactive `.html` artifacts automatically.

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
   for a static site. `katra doctor` flags dangling media and parse errors.

## Notes

- Always keep the draft current *while* working — lining the work up for a post
  is itself QA, and a stale log is the failure mode this tool exists to prevent.
- One draft at a time is the happy path; `stamp` targets the newest draft. Use
  `--entry <slug>` to target a specific one.
- If the MCP server is connected, the equivalents are `katra_new`,
  `katra_append`, `katra_capture`, `katra_compare`, `katra_stamp`,
  `katra_list`, `katra_get`.
