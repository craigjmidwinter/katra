---
title: Assemble the .mcpb in the katra-mcp build post-hook, not an after hook
date: "2026-08-25"
time: "08:34:52"
summary: Build-phase assembly is what gets the bundles inside the signed checksums.txt
type: decision
status: accepted
entry: ship-an-mcpb-bundle-before-the-v0-1-0-tag
---

GoReleaser's order is build → build post-hooks → archives → checksum → sign → publish.

An `after` hook runs past `checksum`, so a bundle assembled there ships outside `checksums.txt`. That matters here specifically: the release footer teaches `sha256sum --check` followed by `cosign verify-blob` as *the* way to verify a katra download. An asset that instruction silently does not cover is worse than having no instruction, because the reader has no way to tell which assets it applied to.

Assembling per-target in the `katra-mcp` build's `hooks.post` lands the bundles before `checksum`, so `checksum.extra_files` picks them up and cosign's signature over `checksums.txt` covers them transitively. Verified against a local `release --snapshot`: all four `.mcpb` appear in `checksums.txt` beside the four tarballs.

Rejected — a second `archives` entry with `formats: [zip]`. The obvious move, and it fails on the extension: goreleaser appends `.zip` from a fixed format enum with no custom-extension escape, producing `katra_x_y.zip` that an uploader expecting `.mcpb` will not take.

Rejected — assembling from the finished tarballs in a workflow step after goreleaser. Same defect as the after hook, one layer further out.
