---
title: 'Standards pass gap list: hygiene and sweep'
date: "2026-08-20"
time: "16:22:41"
tags:
    - standards
summary: The audit gap list that specs the hygiene and sweep legs of the 2026-08 standards pass
type: article
---

## Hygiene gaps (audit, 2026-08-20)

Keyed to PROJECT-STANDARDS.md CHECKLIST → Hygiene / Sweep.

**Hygiene leg:**
- No linter config in the repo (no .golangci.yml); CONTRIBUTING references commands — verify what it promises and make it true. Add golangci-lint config (standard linters, not exotic), make it clean, wire into CI alongside build/vet/test.
- Clean-checkout verification: clone to a scratch dir, run the documented CONTRIBUTING setup + make + go test ./... — must be green first try. Any deviation is a defect to fix.
- Dead code / scratch-file sweep: leftover files from prior passes, commented-out blocks, unused exports.
- Release discipline: no tag has ever been pushed; CHANGELOG is [Unreleased]-only and README Status states pre-1.0 honestly — checklist ticks via the honest-status branch. Cutting v0.1.0 is Craig's call (triggers release workflow + homebrew tap push): deferred, not silent.

**Sweep leg (timeboxed, report-only):**
- Secrets: working tree scan + git history spot-check. A history hit ESCALATES to Craig — never a quiet fix.
- govulncheck run and recorded.
- Pattern read: exec.Command usage (katra shells out to git), path traversal in the viewer/hub file serving (media handler, /p/<id>/ routing), the MCP server surface, unvalidated refs.
- Site: no third-party trackers in the Jekyll docs site; external resources justified.
- Perf: hot-path sanity — store listing/parse at hundreds of entries, hub scan across ~15 registered projects; README makes no perf number claims (verify).
- All findings dispositioned fixed / deferred-as-task / escalated.
