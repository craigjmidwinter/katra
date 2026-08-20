# katra brand

Everything here is generated. Do not hand-edit the SVGs under
`docs/assets/brand/` — change `branding/build.py` and re-run it:

```bash
python3 branding/build.py
```

Requires `rsvg-convert` (librsvg) for the PNG rasters and `fonttools` for
converting the wordmark to outlines.

## The mark

A page with its bottom-right corner turned up.

The reading is the tool's one idea: an entry is a page, and it keeps turning
over. The turned corner is where the accent lives, because turning the page is
the moment katra is about — the draft becoming a logged entry.

It is authored on an exact pixel grid and emitted as `<rect>` elements with
`shape-rendering="crispEdges"`, so it is sharp at every size rather than
resampled into mush.

**The silhouette stays a full rectangle.** The crease is a diagonal keyline and
everything beyond it is the accent — the underside of the folded corner.
Putting the accent on the other side of the crease was the first thing tried,
and it reads as a wedge stuck to the page rather than as a fold.

### Two grids

| File | Grid | Use |
| --- | --- | --- |
| `mark.svg` | 24 units, 5 ruled lines, 6-unit fold | 48px and above |
| `mark-small.svg` | 16 units, 3 ruled lines, 4-unit fold | 32px and below |

This is not belt-and-braces. **24 does not divide into 16**, so a 16px raster of
the full mark resamples the page keyline away completely and leaves a grey
smudge with no edges. The small grid is 1:1 at 16px and 2:1 at 32px, so both are
exact. Five ruled lines also merge into a solid block at favicon sizes, which is
why the small mark carries three.

## Voice

**Editorial, archival, measured.** The voice of a well-kept notebook: it
records, it does not sell.

What it refuses to do: no marketing superlatives, no exclamation points, no
adjective standing where a number could stand. It names its limitations in the
open, and it recommends a competitor when the competitor is the better fit —
the Alternatives table in the README is load-bearing, not a formality.

## Name treatment

**Always lowercase: `katra`.** In the wordmark, in prose (including at the
start of a sentence), in CLI output, in the viewer, in `config.yml` titles.
Never "Katra", never all-caps.

## The wordmark

Set in [Fraunces](https://github.com/undercasetype/Fraunces) (SIL Open Font
License 1.1, `branding/fonts/OFL.txt`) — an old-style soft serif with real
editorial weight, which is the identity: katra is an archival, typographic
tool, and its wordmark should read like a masthead, not a terminal.

It was previously set in Silkscreen, shared deliberately with
[mail-muncher](https://github.com/craigjmidwinter/mail-muncher). The fleet
docs/brand standard now forbids two projects sharing a display face, and the
pixel face was always mail-muncher's retro-terminal register rather than this
tool's; Fraunces resolves both at once.

**Converted to outlines**, never `<text>`. An SVG carrying a `<text>` element
renders in whatever font the viewer happens to have installed, which for a logo
means it renders wrong everywhere the font is missing.

## Palette

The Field Notebook palette, the same one the viewer uses
(`internal/viewer/assets/styles.css`). **If you change one, change both.**

| Role | Light | Dark |
| --- | --- | --- |
| Paper | `#f4efe4` | `#1c1a17` |
| Surface | `#efe7d6` | `#26231e` |
| Page (in the mark) | `#f7f2e8` | `#f7f2e8` |
| Ink / body text | `#2c2823` | `#ece5d8` |
| Muted | `#6f6656` | `#a99a80` |
| Rule | `#c9bda4` | `#3a352d` |
| Accent | `#b5502f` | `#b5502f` |
| Link | `#8a3d24` | `#e08a5c` |

### The accent is not a text colour

Measured contrast, all ratios computed rather than eyeballed:

| Pair | Ratio | Verdict |
| --- | --- | --- |
| Ink `#2c2823` on Paper `#f4efe4` | 12.76:1 | body text |
| Ink `#2c2823` on Surface `#efe7d6` | 11.90:1 | body text |
| Muted `#6f6656` on Paper | 4.94:1 | secondary text |
| Muted `#6f6656` on Surface | 4.60:1 | secondary text |
| **Accent `#b5502f` on Paper** | **4.41:1** | **fails AA for body text** |
| **Accent `#b5502f` on Surface** | **4.11:1** | **fails AA for body text** |
| Accent deep `#8a3d24` on Paper | 6.60:1 | links, light side |
| Accent deep `#8a3d24` on Surface | 6.15:1 | links, light side |
| Paper `#ece5d8` on Ink dark `#1c1a17` | 13.86:1 | body text, dark side |
| Muted `#a99a80` on Ink dark | 6.30:1 | secondary text, dark side |
| **Accent `#b5502f` on Ink dark** | **3.43:1** | **fails; too dark on dark** |
| Warm `#e08a5c` on Ink dark | 6.58:1 | links, dark side |
| Warm `#e08a5c` on Surface dark | 5.93:1 | links, dark side |

So `#b5502f` is a **UI accent, not a text colour** — it is correct on the fold
of the mark, on a component boundary, and on large type, and it is wrong for
anything at body size on either background.

The relationship inverts between the two sides, which is the thing to remember:
on light the accent has to be *darkened* to `#8a3d24` to be readable; on dark it
has to be *lightened* to `#e08a5c`. Neither side can use the mid tone.

## Files

Everything in `docs/assets/brand/`:

| File | What |
| --- | --- |
| `mark.svg` | The full mark. |
| `mark-small.svg` | The favicon mark. Also served as the SVG favicon. |
| `wordmark.svg` / `wordmark-dark.svg` | The wordmark alone, outlined. |
| `lockup.svg` / `lockup-dark.svg` | Mark + wordmark. The README and the docs sidebar use these. |
| `social-preview.svg` / `.png` | 1280×640 Open Graph card. |
| `favicon-16.png`, `favicon-32.png`, `favicon-48.png` | Rasters. |
| `apple-touch-icon.png` | 180×180. |
| `mark-480.png`, `lockup-408.png`, `lockup-dark-408.png` | Rasters for anywhere SVG is not accepted. |

The mark has no dark variant, and that is deliberate rather than an omission:
it is a light page with a dark keyline, so its silhouette reads on both
backgrounds. Only the **wordmark** needs one, because ink on a dark background
is invisible — which is why `lockup-dark.svg` exists and `mark-dark.svg` does
not.

The social preview is light-only for the same class of reason: it renders on
someone else's surface — Slack, a timeline, a link preview — where our
`prefers-color-scheme` media query does not apply, so it has to carry its own
background.

## Fleet distinctness

Checked against the other fleet brands (getvect: sticker-art mascot,
interactive slider; mail-muncher: pixel-art terminal-retro). The mark (a page
with a turned corner), the metaphor (a field notebook) and, with Fraunces, the
typography are katra's own.

**Recorded exception — palette family.** The Field Notebook palette is in the
same warm-paper-and-ink family as mail-muncher's (which took its accent from
katra's original `config.yml` in the first place). It stays, with reasoning:
paper and ink are not decoration here, they are the product concept — the
viewer renders entries as pages in a notebook — and the values, the mark, and
the type differ. Revisit if the two ever ship a surface that sits side by
side and reads as one brand.

## What not to do

- **Don't** recolour the mark. The accent on the fold is the one warm value and
  it carries the identity.
- **Don't** set the wordmark in another typeface, or re-type it as live text.
  Use the outlined SVG.
- **Don't** put the mark on a mid-tone background. It is a light page; it needs
  either a light surface or a clearly dark one.
- **Don't** use the accent for body-size text on any background — the measured
  table above is the law.
- **Don't** capitalize the name, anywhere.

## Usage

- **Do** use the lockup at 408px wide or smaller, with the dark variant behind
  `prefers-color-scheme: dark`.
- **Do** keep clear space of at least the mark's own width around the lockup.
