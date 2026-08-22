---
title: The container that is not an install path
date: "2026-08-21"
time: "21:07:21"
tags:
    - registry
    - mcp
    - release
hash: e4158d7
stat:
    f: 19
    a: 469
    d: 20
summary: Packaging katra-mcp for official discovery without reversing Katra's native-first position
closes:
    - publish-katra-mcp-through-the-official-mcp-registry
---

Katra already had the server the official MCP Registry should list: a standalone stdio binary over the same core as the CLI. What it did not have was a package type the registry accepts. The narrow answer is an OCI envelope around `katra-mcp` and `git`, not a containerized Katra.

That distinction now lives in every place it can drift: the README, agent reference, release runbook, contributor map, AGENTS invariant, GoReleaser manifest annotation, and `server.json`. Tagging will build the image first, then publish metadata through GitHub OIDC; there is no registry token to store.

The standards review caught one supply-chain gap before push: both workflows fetched the publisher through an unbounded `latest` URL and trusted the archive before extraction. They now share one installer pinned to the official v1.8.1 Linux/amd64 release and its SHA-256 digest. The same checked binary therefore validates pull requests and performs a future tag-gated publication.

```embed
src: media/mcp-registry-envelope.html
height: 430
caption: The host binaries remain the supported install; the 11.9 MB compressed OCI save is only the registry package.
```


The first useful correction was not packaging at all. The wire exposes 15 tools, while the agent reference listed 14: `katra_task_spec` had shipped in code and tests but never made the table. A protocol-level inventory test now locks the live `tools/list` response to the documentation.

The bounded Linux/amd64 smoke run used the exact 2,266,481-byte build context on spaceseed. The image was 26,303,354 bytes unpacked and 11,928,639 bytes as a gzip-compressed Docker save. It initialized as `katra` v0.1.0-smoke, listed all 15 tools, and created a task in a bind-mounted repo. The payload, build directory, repo, and image were removed after the run. Native fresh install independently reached the same 15-tool response and created a task in five seconds.

```warning
No tag, GHCR image, GitHub release, or registry entry was published. The local Docker Desktop engine was unresponsive, so the bounded image test ran only for linux/amd64 on the existing remote Docker host; CI remains responsible for the linux/arm64 build through QEMU.
```
