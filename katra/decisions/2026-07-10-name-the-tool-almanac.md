---
title: Name the tool Almanac
date: "2026-07-10"
time: "22:56:39"
tags:
    - naming
    - product
summary: Rename devlog -> Almanac; the five node types map onto an almanac's sections
type: decision
status: superseded
entry: designing-the-wiki-shaped-evolution-tasks-epics-decisions-articles
superseded-by:
    - name-the-tool-katra-almanac-was-taken
---

## Context

"devlog" was right when the tool only chronicled the past. It's evolving into a
git-committed, markdown-native project wiki with five node types — entries,
tasks, epics, decisions, articles (see the [[almanac-node-model]] epic) — so
"log" now names one of five things it does. A name was wanted that fits the
whole, without dismissing the real migration cost of a rename (see
[[rename-devlog-to-almanac-compat-shim]]).

## Decision

Name the tool **Almanac**. A historical almanac bound together exactly the
genres this tool models: a chronological diary (entries), predictions &
calendars (tasks / epics / roadmap-by-horizon), and reference tables (articles /
decisions). The name describes the whole artifact, and each node type maps onto a
traditional almanac "section" — which doubles as the product's mental model.

## Alternatives considered

- **Stay `devlog`** (reframe "log" as a container noun). Lowest cost, zero
  migration. Rejected: "log" undersells tasks/decisions.
- **Cairn** (trail-marker metaphor). Strong brand, low collision. Rejected:
  evocative but doesn't *describe* the five genres.
- **Ledger** (holds history + future). Great fit; rejected for naming collision
  (crypto wallet, `hledger`).
- **Slate / Logbook / workshop-family (Forge, Foundry, Atelier).** Less
  expressive or heavily taken.

## Consequences

- CLI verb becomes `almanac`; ship a short `alm` alias.
- Per-repo data dir becomes `almanac/`, with a compatibility shim reading legacy
  `devlog/` when absent — tracked as [[rename-devlog-to-almanac-compat-shim]].
- Binaries: `almanac`, `almanac-mcp`, and the planned daemon `almanacd` (see the
  [[global-hub-almanacd]] epic).
- The `.gitignore` binary-vs-dir collision fixed for `devlog/` recurs under the
  new name — carry the `/almanac` + `!/almanac/` negation forward.
- Availability still to verify (GitHub org, brew, npm, domain).

> Migrated from `docs/decisions/0001-…` into the store's `decisions/` dir once
> the decision node type existed — exactly as that ADR said it would.
