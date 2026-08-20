#!/usr/bin/env python3
"""Generate the katra brand assets.

The mark is authored on an exact pixel grid and emitted as SVG made of `<rect>`
elements with `shape-rendering="crispEdges"`, so it is sharp at every size
rather than resampled. The SVGs under `docs/assets/brand/` are the masters the
site and the README consume; this script regenerates them, plus the PNG
rasters, from the definitions below.

    python3 branding/build.py

Requires `rsvg-convert` (librsvg) for the PNGs and `fonttools` for converting
the Fraunces wordmark to outlines, so no SVG consumer needs the font file.

Two grids, not one. A 24-unit mark carries the ruled lines; a 16-unit mark
drops most of them for the favicon sizes. This is not belt-and-braces — the
24-grid does not divide into 16, so a 16px raster of it resamples the keyline
away entirely and the page loses its edges. Anything 32px or below uses the
small grid; 48px and above uses the full one.

See BRAND.md for the palette, the contrast measurements, and the usage rules.
"""

import os
import subprocess

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "docs", "assets", "brand")
FONT = os.path.join(ROOT, "branding", "fonts", "Fraunces72pt-Black.ttf")

# ------------------------------------------------------------- palette ---
# The Field Notebook palette, the same one the viewer uses
# (internal/viewer/assets/styles.css). If you change one, change both.
PAL = {
    "K": "#2c2823",  # Ink        — the page keyline and the fold crease
    "P": "#f7f2e8",  # Paper card — the page itself
    "R": "#c9bda4",  # Rule       — the ruled lines
    "A": "#b5502f",  # Accent     — the turned-up underside of the corner
}
INK_DARK = "#ece5d8"  # wordmark colour on a dark background


# ---------------------------------------------------------------- mark ---
def mark_grid(n, x0, x1, y0, y1, fold, rules):
    """A page with its bottom-right corner turned up.

    The silhouette stays a full rectangle: the crease is a diagonal keyline and
    everything beyond it is the accent, which is the underside of the folded
    corner. Putting the accent on the *other* side of the crease reads as a
    wedge stuck to the page rather than as a fold — it was the first thing
    tried, and it did not work.
    """
    fx, fy = x1 - fold, y1 - fold

    def cell(x, y):
        if not (x0 <= x <= x1 and y0 <= y <= y1):
            return None
        in_fold = x >= fx and y >= fy
        d = (x - fx) + (y - fy)
        if in_fold:
            if d == fold:
                return "K"  # the crease
            if d > fold:
                return "A"  # the turned-up underside
        if x in (x0, x1) or y in (y0, y1):
            return "K"  # page keyline
        if y in rules and x0 + 2 <= x <= x1 - 2 and not (in_fold and d >= fold - 1):
            return "R"  # a ruled line, stopping clear of the crease
        return "P"

    return n, [[cell(x, y) for x in range(n)] for y in range(n)]


def grid_svg(n, grid, scale=1):
    body = []
    for y, row in enumerate(grid):
        for x, c in enumerate(row):
            if c:
                body.append(
                    f'<rect x="{x * scale}" y="{y * scale}" '
                    f'width="{scale}" height="{scale}" fill="{PAL[c]}"/>'
                )
    side = n * scale
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {side} {side}" '
        f'width="{side}" height="{side}" shape-rendering="crispEdges" '
        f'role="img" aria-label="katra">\n  ' + "\n  ".join(body) + "\n</svg>\n"
    )


# ------------------------------------------------------------ wordmark ---
def wordmark_paths(text):
    """Fraunces glyph outlines as SVG path data, plus the total advance and
    the tight vertical bounds (max ascender to lowest descender) of `text`.

    Converted to outlines on purpose: an SVG carrying `<text>` renders in
    whatever the viewer happens to have installed, which for a logo means it
    renders wrong everywhere the font is missing.
    """
    from fontTools.pens.boundsPen import BoundsPen
    from fontTools.pens.svgPathPen import SVGPathPen
    from fontTools.ttLib import TTFont

    font = TTFont(FONT)
    glyphs = font.getGlyphSet()
    cmap = font.getBestCmap()

    out, x = [], 0
    min_y, max_y, ink_x0 = None, None, None
    for ch in text:
        name = cmap[ord(ch)]
        pen = SVGPathPen(glyphs)
        glyphs[name].draw(pen)
        d = pen.getCommands()
        if d:
            out.append((d, x))
        bounds_pen = BoundsPen(glyphs)
        glyphs[name].draw(bounds_pen)
        if bounds_pen.bounds:
            gx0, gy0, _, gy1 = bounds_pen.bounds
            min_y = gy0 if min_y is None else min(min_y, gy0)
            max_y = gy1 if max_y is None else max(max_y, gy1)
            if ink_x0 is None:
                ink_x0 = x + gx0  # left side bearing of the first glyph's ink
        x += glyphs[name].width
    return out, x, min_y, max_y, ink_x0


def wordmark_svg(height, colour):
    """The wordmark alone, scaled so its full glyph band (ascender of "k" to
    the slight descenders on "a"/"t") sits exactly on `height` — no clipping,
    no guessed cap height."""
    paths, advance, min_y, max_y, _ = wordmark_paths("katra")
    s = height / (max_y - min_y)
    w = round(advance * s)
    baseline = max_y * s  # distance from the top of the glyph band to the baseline
    body = []
    for d, x in paths:
        body.append(
            f'<path transform="translate({x * s:.3f} 0) scale({s:.5f} -{s:.5f})" '
            f'fill="{colour}" d="{d}"/>'
        )
    return w, (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {height}" '
        f'width="{w}" height="{height}" role="img" aria-label="katra">\n'
        f'  <g transform="translate(0 {baseline:.3f})">\n    '
        + "\n    ".join(body)
        + "\n  </g>\n</svg>\n"
    )


def lockup_svg(colour):
    """Mark + wordmark on one baseline, sized for a README header."""
    n, grid = MARK
    mark_px = 64  # the mark renders at 64, so 24 units -> 8/3 px each
    scale = mark_px / n
    word_h = 62  # wordmark glyph-band height, optically balanced against the mark
    gap = 16

    paths, advance, min_y, max_y, _ = wordmark_paths("katra")
    s = word_h / (max_y - min_y)
    word_w = advance * s
    total_w = round(mark_px + gap + word_w)
    height = mark_px

    body = []
    for y, row in enumerate(grid):
        for x, c in enumerate(row):
            if c:
                body.append(
                    f'<rect x="{x * scale:.4f}" y="{y * scale:.4f}" '
                    f'width="{scale:.4f}" height="{scale:.4f}" fill="{PAL[c]}"/>'
                )
    # Centre the wordmark's glyph band (not its em box) against the mark.
    top = (height - word_h) / 2
    baseline = top + max_y * s
    word = []
    for d, x in paths:
        word.append(
            f'<path transform="translate({x * s:.3f} 0) scale({s:.5f} -{s:.5f})" '
            f'fill="{colour}" d="{d}"/>'
        )

    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {total_w} {height}" '
        f'width="{total_w}" height="{height}" role="img" aria-label="katra">\n'
        f'  <g shape-rendering="crispEdges">\n    ' + "\n    ".join(body) + "\n  </g>\n"
        f'  <g transform="translate({mark_px + gap} {baseline:.3f})">\n    '
        + "\n    ".join(word)
        + "\n  </g>\n</svg>\n"
    )


def social_svg():
    """1280x640 Open Graph card. Light only — it renders on someone else's
    surface (Slack, a timeline), where our dark-mode media query does not
    apply, so it has to carry its own background."""
    n, grid = MARK
    mark_px = 200
    scale = mark_px / n
    # The content block (mark, wordmark, tagline) spans roughly 200 units and is
    # centred on the 640 height. Cards get cropped to several ratios by whoever
    # is unfurling them, and anything hugging an edge is what gets cut.
    mx, my = 150, 215
    body = []
    for y, row in enumerate(grid):
        for x, c in enumerate(row):
            if c:
                body.append(
                    f'<rect x="{mx + x * scale:.4f}" y="{my + y * scale:.4f}" '
                    f'width="{scale:.4f}" height="{scale:.4f}" fill="{PAL[c]}"/>'
                )
    paths, advance, min_y, max_y, ink_x0 = wordmark_paths("katra")
    word_h = 96
    top = 249  # anchors the glyph band's top; band runs 249..345, clear of the tagline at 405
    s = word_h / (max_y - min_y)
    baseline = top + max_y * s
    word = []
    for d, x in paths:
        word.append(
            f'<path transform="translate({x * s:.3f} 0) scale({s:.5f} -{s:.5f})" '
            f'fill="{PAL["K"]}" d="{d}"/>'
        )
    # The first glyph ("k") carries its own left side bearing, so its ink
    # starts a little in from the wordmark's nominal x origin (400). Read
    # that bearing back out of the glyph's own bbox rather than hard-coding
    # it, so the tagline lines up with the ink, not the box.
    ink_x0 = 400 + ink_x0 * s if ink_x0 is not None else 400
    tag = (
        "A committed, rich-component dev log you write as you build."
    )
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 640" '
        'width="1280" height="640" role="img" '
        'aria-label="katra — a committed, rich-component dev log you write as you build">\n'
        f'  <rect width="1280" height="640" fill="#f4efe4"/>\n'
        f'  <rect x="0" y="0" width="1280" height="10" fill="{PAL["A"]}"/>\n'
        f'  <g shape-rendering="crispEdges">\n    ' + "\n    ".join(body) + "\n  </g>\n"
        f'  <g transform="translate(400 {baseline:.3f})">\n    ' + "\n    ".join(word) + "\n  </g>\n"
        f'  <text x="{ink_x0:.1f}" y="405" font-family="IBM Plex Sans, system-ui, sans-serif" '
        f'font-size="30" fill="#6f6656">{tag}</text>\n'
        "</svg>\n"
    )


# ------------------------------------------------------------------ io ---
# 24 units: the full mark, five ruled lines, a six-unit fold.
MARK = mark_grid(24, 4, 19, 1, 20, 6, (4, 7, 10, 13, 16))
# 16 units: the favicon mark. Three rules and a four-unit fold — at 16px the
# ruled lines are one pixel each and five of them turn into a grey block.
MARK_SMALL = mark_grid(16, 2, 13, 1, 14, 4, (4, 7, 10))


def png(svg_name, width, png_name):
    subprocess.run(
        ["rsvg-convert", "-w", str(width), "-o",
         os.path.join(OUT, png_name), os.path.join(OUT, svg_name)],
        check=True,
    )


def main():
    os.makedirs(OUT, exist_ok=True)

    write = lambda name, s: open(os.path.join(OUT, name), "w").write(s)

    write("mark.svg", grid_svg(*MARK, scale=20))
    write("mark-small.svg", grid_svg(*MARK_SMALL, scale=20))
    write("lockup.svg", lockup_svg(PAL["K"]))
    write("lockup-dark.svg", lockup_svg(INK_DARK))
    _, w = wordmark_svg(64, PAL["K"])
    write("wordmark.svg", w)
    _, w = wordmark_svg(64, INK_DARK)
    write("wordmark-dark.svg", w)
    write("social-preview.svg", social_svg())

    # Favicons come off the small grid; anything larger off the full mark.
    png("mark-small.svg", 16, "favicon-16.png")
    png("mark-small.svg", 32, "favicon-32.png")
    png("mark.svg", 48, "favicon-48.png")
    png("mark.svg", 180, "apple-touch-icon.png")
    png("mark.svg", 480, "mark-480.png")
    png("lockup.svg", 408, "lockup-408.png")
    png("lockup-dark.svg", 408, "lockup-dark-408.png")
    png("social-preview.svg", 1280, "social-preview.png")

    for f in sorted(os.listdir(OUT)):
        print("  ", f)


if __name__ == "__main__":
    main()
