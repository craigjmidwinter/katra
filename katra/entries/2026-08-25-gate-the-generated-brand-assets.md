---
title: Gate the generated brand assets
date: "2026-08-25"
time: "08:40:27"
closes:
    - gate-branding-build-py-output-with-a-hash-based-check
---

Why hash-based and not a byte-diff of everything. The steward tried a byte-diff first: green on its laptop, red on CI for all six files, because runners' librsvg and ImageMagick differ from any developer's. The split that actually works is by determinism, not by file type. The seven SVGs are pure string output from the definitions in build.py, so they are compared byte for byte. The eight PNGs are rsvg-convert output, so they are compared by the SHA-256 of the SVG they were rendered from, recorded in branding/assets.lock.json. That catches the failure that matters — a vector regenerated and committed while its rasters were left stale — without pinning the repo to one renderer build.

One real fault the injections found in my own checker. Faulting the palette table (Muted #6f6656 → #7f7666) while leaving the contrast table alone walked straight through the first version of contrast.py: the contrast rows repeat their hexes inline, so each row still recomputed correctly against its own now-orphaned colour. Green check, two tables describing different colours. Fixed by asserting every hex quoted in the contrast table is still present in the palette table. This is exactly the failure mode the steward described from its own repo — a script existing and nothing running it is one problem; a script running and not looking at the right thing is the quieter one.

```embed
src: media/gates-fire.html
height: 480
caption: Both gates faulted, one injection per failure mode they claim to catch. All six fired; the tree was restored clean afterwards.
```

Left ungated, deliberately, and said so in BRAND.md rather than quietly. BRAND.md claims the palette is 'the same one the viewer uses (internal/viewer/assets/styles.css). If you change one, change both.' Six of the fourteen palette hexes are not in that stylesheet: #1c1a17, #26231e, #3a352d, #c9bda4, #e08a5c, #ece5d8. The light core does match — paper, surface, page, ink, muted, accent, link — but Rule is #c9bda4 in BRAND.md and #e5dcc9 in the stylesheet, and the whole dark column has no counterpart because the viewer's stylesheet is light-only. Gating that claim means either editing the stylesheet or softening the claim, and both are decisions beyond a standards gap. BRAND.md now says the sentence is ungated instead of implying it is checked.
