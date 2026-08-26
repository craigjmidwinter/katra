---
title: Pin fontTools in CI and record the version in assets.lock.json
date: "2026-08-25"
time: "08:41:13"
summary: The wordmark SVGs are SVGPathPen output, so the toolchain version is part of the asset's provenance
type: decision
status: accepted
entry: gate-the-generated-brand-assets
---

The wordmark and lockup SVGs are Fraunces glyph outlines emitted by `fontTools.pens.svgPathPen`. That pen's exact output has changed between fontTools releases, so a byte-comparison of those SVGs is really a comparison of two fontTools versions — and an unpinned `pip install fonttools` in CI would eventually turn a routine push red with 'mark.svg differs' and no clue why.

So the lock file records the fontTools version alongside the asset hashes, CI pins that exact version, and a CI step asserts the pin and the lock agree. When the SVGs differ *and* the versions differ, `--check` says so explicitly rather than reporting a bare byte-diff:

    fontTools 4.64.0 is installed but these assets were generated with
    4.63.0 -- pin fontTools==4.63.0 before concluding the assets are stale

Bumping fontTools is therefore a deliberate three-part commit: regenerate, update the lock, update the CI pin.

This is the same reasoning as the PNG decision one level down. PNG bytes depend on librsvg, so they are not compared at all — the raster is checked against the SHA-256 of the vector it came from. Compare the thing that is deterministic; record the thing that is not.
