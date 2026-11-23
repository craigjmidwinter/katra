# Changelog

All notable changes to katra are documented here, in [Keep a
Changelog](https://keepachangelog.com/en/1.1.0/) format.

No version has been tagged yet — see [Status and
scope](README.md#status-and-scope) and [RELEASING.md](RELEASING.md). Until the
first release, everything lands under `[Unreleased]`.

## [Unreleased]

### Security

- **Embeds run in an opaque origin.** The sandbox is now `allow-scripts
  allow-pointer-lock`, because `allow-same-origin` let an embed remove it. Under
  `serve` and `hub serve`, every `media/` response is sandboxed by header as
  well, so opening a file directly no longer runs it on the viewer origin.
- **Content security policy on every page.** Inline event handlers and
  `javascript:` links in entries no longer run, and remote images and frames are
  blocked. This changes existing content: see
  [the migration note](docs/components.md#migrating-to-the-opaque-origin-sandbox).

### Fixed

- **Saving a node no longer erases frontmatter keys katra does not know.** A
  status change or stamp re-wrote the file from katra's own fields, so a key
  added by another tool, or by a newer katra, vanished the first time an older
  one touched the file. Unknown keys now round-trip with their values.

### Added

- **`rawHTML: allow|sanitize`** in `config.yml`. `sanitize` strips raw HTML from
  entries; the default keeps it.
- **Windows release support.** Tagged releases now include amd64 and arm64 zip
  archives containing `katra.exe` and `katra-mcp.exe`, plus matching MCPB
  bundles. Native Windows locking and path handling make those binaries usable,
  not merely cross-compiled.
- **One harness-neutral Katra workflow.** The complete epic → specced task →
  committed spec → running entry → decision/evidence → stamp/closure loop now
  lives in public docs and canonical `AGENTS.md`, rather than depending on the
  Claude Code skill. Release CI checks the packaged CLI for all three spec-phase
  surfaces that an installed v0.1.0 binary lacks.
- **Official MCP Registry release packaging.** The existing 15-tool
  `katra-mcp` stdio server now has validated `server.json` metadata, a minimal
  registry-only OCI wrapper, and tag-gated GitHub OIDC publication with no
  stored registry token. Native binaries remain the supported install path;
  no image is published before the first tag.
- **The core log workflow.** `katra new` starts a draft, `katra append` /
  `capture` / `compare` build it up while you work, and the commit you were
  already making stamps it with its hash and diffstat. A draft is an entry
  with no commit hash — that's the whole state machine, so nothing sits in a
  scratch file waiting to be promoted.
- **Six rich components** — `compare`, `gallery`, `video`, `embed`, `note`,
  `warning` — as fenced code blocks. An unregistered language renders as a
  plain code block instead of erroring, so an entry written against a newer
  katra still reads in an older one.
- **A node model beyond entries**: `task`, `epic`, `decision` and `article`
  node types, cross-linked with `[[wikilinks]]` and rolled up onto a board and
  a roadmap.
- **A `specced` task status and `spec:` reference.** `katra task spec <slug>
  <ref>` points a task at a committed design — a decision, an article, or a
  path in the repo — and moves it from `todo` to `specced`: a design exists,
  committed, and nobody has started building it yet.
- **Agent integration**: a Claude Code skill, seven lifecycle hooks
  (`SessionStart`, `PostToolUse`, `Stop`, `PreToolUse`, and the git
  post-commit stamp), an optional commit gate that blocks a commit whose
  authored changes nothing has declared a purpose for, an MCP server
  (`katra-mcp`) for clients that would rather call a tool than shell out, and
  `katra memory scan` to ingest Claude Code's own project memory into a
  local, gitignored ledger.
- **The hub.** `katra hub serve` gives every registered katra on the machine
  one cross-project board, roadmap and merged chronological log, with a
  launchd agent for macOS (`katra hub install`) and a systemd unit in
  `contrib/` for Linux.
- **`katra build`**, a self-contained static site — `index.html`, `data.json`,
  media, no external requests — and `katra build --all` for the hub's
  cross-project equivalent.
