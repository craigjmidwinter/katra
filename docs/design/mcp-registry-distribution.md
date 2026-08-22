---
title: MCP Registry distribution
layout: default
parent: Design notes
nav_order: 5
description: >-
  Package the existing stdio server for official registry discovery without
  turning a container into Katra's supported install path.
---

# MCP Registry distribution

Status: implementation plan for the first release.

## Problem

`katra-mcp` is already a standalone stdio MCP server, but the official MCP
Registry lists installable packages rather than bare Go binaries. Katra's
release archives contain both `katra` and `katra-mcp`; they are not an accepted
registry package type.

Katra also deliberately does not offer a container as a normal install path.
Its value comes from operating on the caller's working tree and Git history,
so a container requires mounting the very host state a native binary can use
directly.

## Decision

Publish a narrow OCI package for registry discovery only:

- The image is `ghcr.io/craigjmidwinter/katra-mcp` and contains the
  `katra-mcp` binary plus `git`, which stamping and diffstat operations need.
- The image entrypoint is `katra-mcp`; it does not include the viewer, hub,
  setup flow, hooks, or the `katra` CLI.
- `server.json` lists the image as a stdio package under
  `io.github.craigjmidwinter/katra`.
- The image manifest carries the same server name in the
  `io.modelcontextprotocol.server.name` annotation, which is how the registry
  verifies package ownership.
- A tag release builds and pushes the image first. A dependent workflow job
  then stamps the tag into `server.json`, authenticates with GitHub Actions
  OIDC, and publishes the metadata. No long-lived registry credential is
  stored.
- CI and release use the same installer for `mcp-publisher`. Its versioned
  Linux/amd64 release URL and SHA-256 digest are pinned in one script, so the
  validator cannot drift from the binary that publishes.
- Pull requests validate `server.json` and build the image without publishing
  it. The first registry publication waits for the first release tag.

The native binary remains the documented install path. The OCI wrapper is not
a new general-purpose container distribution: it exists to give the official
registry a package it can index.

## Acceptance evidence

- The SHA-256-verified `mcp-publisher` v1.8.1 accepts `server.json`.
- A snapshot builds the release archives and the local OCI image without
  publishing either.
- Starting the image against a freshly initialized, bind-mounted repository
  completes the MCP initialize handshake, lists the same tools as the native
  server, and performs one write through a tool call.
- The image's manifest annotation equals the name in `server.json`.
- README, release runbook, contributor guidance, and `AGENTS.md` describe the
  same registry-only scope.
