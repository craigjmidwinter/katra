# Design: an `.mcpb` bundle in the release

Status: accepted, 2026-08-25.
Task: `ship-an-mcpb-bundle-in-the-v0-1-0-release`.
Companion to [mcp-registry-distribution](mcp-registry-distribution.md), which
covers the OCI leg.

## Why a second package format at all

`server.json` declares an `oci` package and nothing else. That is the right
shape for the official MCP Registry and the wrong shape for every venue that
does not ingest it. Smithery, checked rather than assumed, offers exactly two
routes: a hosted streamable-HTTP endpoint, or an uploaded `.mcpb` bundle. A
GitHub repository alone is not listable there. katra runs no hosted endpoint
and is not going to, so the bundle is the only door.

The evidence for that came from the steward's audit of a sibling project which
has sat un-listed for twenty-five days holding exactly the package set katra
holds today. This is not a hypothetical gap.

## Why it is nearly free

`.goreleaser.yml` already builds `katra-mcp` from `./cmd/katra-mcp` with
`CGO_ENABLED=0` for darwin/linux × amd64/arm64. MCPB's `server.type: "binary"`
wants precisely that: a self-contained pre-compiled executable with no runtime
declaration. There is no new compile, only a repackage.

The window is cheap *now* because there are zero tags. Adding the bundle after
`v0.1.0` exists means cutting a second release whose only content is packaging.

## Where the bundle is assembled

The bundle must be covered by `checksums.txt`, because the release footer
teaches checksum-then-cosign verification and an asset outside that chain
quietly weakens the instruction. GoReleaser's ordering fixes the choice:

    build → build post-hooks → archives → checksum → sign → publish

Assembling in an `after` hook would land the bundle *after* `checksum`, leaving
it unverifiable. Assembling in the `katra-mcp` build's `hooks.post` lands it
before, so `checksum.extra_files` can pick it up and cosign's signature over
`checksums.txt` covers it transitively.

So: `scripts/build-mcpb.sh` runs once per `katra-mcp` build target, receiving
that target's binary path, os, arch and version.

## Layout and manifest

One `.mcpb` per platform, because `entry_point` names a single executable:

    katra_0.1.0_darwin_arm64.mcpb   (a zip)
    ├── manifest.json
    └── server/
        └── katra-mcp

`manifest.json` is rendered from `packaging/mcpb/manifest.json.tmpl`. Verified
against `modelcontextprotocol/mcpb`'s `MANIFEST.md` on 2026-08-25: the required
fields are `manifest_version`, `name`, `version`, `description`, `author` and
`server`; `manifest_version` is `0.3`; `server.type` accepts `binary`; and
`compatibility.platforms` accepts `darwin`, `linux` and `win32`.

Two deliberate constraints:

- `description` is copied verbatim from `server.json`, so the registry listing
  and the bundle listing cannot drift into two different claims about what
  katra is. A test enforces the equality rather than a comment asking for it.
- `platforms` is `["darwin", "linux"]` because the goreleaser matrix is.
  Widening it to the spec's full enum would claim a Windows build that does not
  exist. The same test pins this to the actual `goos` list.

`mcp_config.command` is included even though it is optional for some server
types: the spec's binary example is silent on stdio invocation, and stating the
command is cheaper than discovering the default was wrong.

## Named caveat

MCPB began life as a Claude Desktop format and its manifest still carries a
`compatibility.claude_desktop` key. The spec has since moved to the
`modelcontextprotocol` org, which is what makes it cross-vendor. katra does not
set that key. The lineage is worth knowing; it does not change the decision,
because the alternative to a slightly Claude-shaped format is no listing.

## What is not verified

Nobody has yet put a katra bundle through Smithery's submission flow, because
no release exists to submit. `release --snapshot` proves the bundle is built,
zipped, checksummed and structurally correct. It does not prove an MCP client
installs it. Expect to debug the install, not the manifest.
