---
title: The workflow escaped the skill
date: "2026-08-21"
time: "21:27:38"
tags:
    - agents
    - workflow
    - parity
summary: Making Katra’s complete spec-to-stamp loop portable across Claude Code, Codex, MCP, and a plain shell
closes:
    - close-the-codex-workflow-parity-gap
---

The workflow had all the machinery but one wrong owner. Tasks, epics, specs, decisions, entries, and stamps were plain CLI operations; the only complete explanation of how to combine them lived inside a Claude Code skill. A Codex session could discover the commands, but it had to reconstruct the method.

The tempting fix was a Codex-specific adapter mirroring the Claude hooks. That lost because it would create a second private contract and another drift surface. The published workflow and root `AGENTS.md` now own the common loop instead; harness adapters can remind, ingest private state, or gate a commit, but they cannot redefine what completing Katra work means.

The acceptance exposed a second boundary: `/opt/homebrew/bin/katra` reports v0.1.0 and has none of `task spec`, `task new --spec`, or `specced` in task-list help. An isolated current-source binary passed all three. The docs now call the phase unreleased, and CI tests the extracted release archive so source tests cannot make an older installed surface look current.

The MCP distribution work stayed inside its envelope: fifteen tools, registry-only OCI positioning, and one v1.8.1 publisher installer whose archive is checked before extraction. No new harness behavior entered the image and no release action ran.

```embed
src: media/harness-neutral-workflow.html
height: 480
caption: The same plan-to-proof path through CLI, Codex, Claude Code, and MCP
```

The common path is now durable and inspectable: a cold session can recover the outcome from the epic, the intent from the spec pointer, the choice from the decision, and the result from this entry. The remaining boundary is honest and mechanical—the spec phase becomes an installed promise only when a future release artifact passes the new surface check.
