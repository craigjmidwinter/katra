---
title: One Katra workflow for every coding harness
layout: default
parent: Design notes
nav_order: 6
description: >-
  Move the durable spec-to-stamp workflow out of the Claude skill and into
  Katra's public CLI documentation and canonical agent context.
---

# Design: one Katra workflow for every coding harness

- **Status:** accepted for implementation
- **Date:** 2026-08-21

## Problem

Katra's CLI already exposes entries, tasks, epics, decisions, spec pointers,
task closure, and post-commit stamping. Its most complete explanation of how
those pieces fit together lives in the public Claude Code skill, however.
`docs/agents.md` points readers back to that skill for the full writing
guidance, and the root `AGENTS.md` explains the Claude gate without teaching a
Codex or shell-only agent the complete workflow.

The skill also describes Claude Code project-memory ingest as though every
harness supplied it, and its MCP equivalence note lists only the seven entry
tools even though the server exposes fifteen tools, including task specs,
epics, decisions, and articles.

## Decision

Make a published, harness-neutral workflow page the common contract:

1. Create an epic and child task when the work belongs to a larger outcome.
2. Author a durable spec, attach it with `katra task spec`, and commit that
   planning state before implementation when the task warrants a design.
3. Start the task and open a draft before editing implementation files.
4. Append decisions and reasoning while working; promote durable choices with
   `katra decide`; capture at least one visual artifact.
5. Commit the bounded implementation, then stamp the draft with `--closes` so
   the task closes, links to the entry, and rolls up its epic.
6. Use the portable Git post-commit hook when desired. Treat Claude hooks and
   memory ingestion as optional adapters, never as workflow prerequisites.

The root `AGENTS.md`, quickstart, agent guide, CLI reference, README, and
embedded skill will point to or summarize the same flow. The skill remains a
Claude-specific delivery mechanism, not the canonical source of shared
behavior.

## Acceptance

From this Codex-hosted session, using only the built `katra` CLI and published
repository documentation:

- create one epic;
- create one child task;
- attach this file as its spec and observe `specced`;
- commit the planning artifacts before implementation;
- move the task to `doing` and create a draft;
- implement the bounded parity changes;
- append reasoning and capture one self-contained visual;
- commit, stamp with the task closure, and verify task `done`, epic `done`, a
  resolvable spec pointer, a stamped entry, and a clean working tree.

The MCP surface must remain fifteen tools. Registry OCI packaging must remain
registry-only, and both workflows must continue using the shared pinned,
SHA-256-verified `mcp-publisher` installer.

## Out of scope

- Adding Codex-specific hooks or reading Codex private state.
- Making a spec mandatory for every task.
- Changing the on-disk format, task lifecycle, MCP tool shapes, or release
  packaging.
