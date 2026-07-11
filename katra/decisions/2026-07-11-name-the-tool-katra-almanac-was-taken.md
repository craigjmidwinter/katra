---
title: Name the tool Katra (Almanac was taken)
date: "2026-07-11"
time: "07:05:53"
tags:
    - naming
    - product
summary: Almanac collided with an existing codebase-wiki tool; chose the Vulcan katra instead
type: decision
status: accepted
entry: designing-the-wiki-shaped-evolution-tasks-epics-decisions-articles
supersedes:
    - name-the-tool-almanac
---

## Context

[[name-the-tool-almanac]] chose "Almanac". An availability check before executing
the rename found **AlmanacCode/codealmanac** on GitHub — *"a codebase wiki for AI
coding agents; captures decisions, flows, invariants, gotchas."* That is a
near-exact collision: the same space and nearly the same concept. Using
"Almanac" would be confusing and look derivative.

## Decision

Name the tool **Katra**. In Vulcan lore a *katra* is the preserved living record
of a being's memories and knowledge — carried in a katric ark and passed on. That
is precisely what this tool is: a committed, durable record of everything a
project knew — its work, decisions, and reference. It also honours the standing
instruction to fall back to a Vulcan/Klingon name if Almanac was unavailable.

The only namespace hit for "katra" is a dormant alpha BASIC-interpreter toy on
npm — unrelated space, not a real collision for a Go CLI.

## Consequences

- CLI verb, binaries (`katra`, `katra-mcp`, planned `katrad`), the per-repo data
  dir (`katra/`), MCP tools (`katra_*`), the env var (`KATRA_DIR`), config commit
  prefix (`katra:`), skill, README, and design docs all move to Katra.
- **Compat shim** ships with the rename: `FindStore` falls back to a legacy
  `devlog/` dir, and `KATRA_DIR` falls back to `DEVLOG_DIR`, so the ~40 existing
  repos keep working with no migration.
- The Go **module path** (`github.com/craigjmidwinter/devlog`) is deliberately
  left unchanged for now — invisible to CLI users; it moves together with
  renaming the GitHub repo, as a follow-up.
- Still to verify: GitHub/brew/domain for `katra` (npm is the dormant toy).

## Not superseding the record

[[name-the-tool-almanac]] is marked `superseded`, not deleted — the reasoning for
Almanac (and why it lost) stays legible. This is the decision chain working as
designed.
