#!/usr/bin/env python3
"""Generate the katra brand assets.

The mark is authored on an exact pixel grid and emitted as SVG made of `<rect>`
elements with `shape-rendering="crispEdges"`, so it is sharp at every size
rather than resampled. The SVGs under `docs/assets/brand/` are the masters the
site and the README consume; this script regenerates them, plus the PNG
rasters, from the definitions below.

    python3 branding/build.py            # regenerate every asset
    python3 branding/build.py --check    # verify the committed assets are current

Requires `rsvg-convert` (librsvg) for the PNGs and `fonttools` for converting
the Fraunces wordmark to outlines, so no SVG consumer needs the font file.

`--check` is deliberately not a byte-diff of everything. The SVGs are pure
string output from the definitions below, so those are compared byte for byte.
The PNGs are not: `rsvg-convert` produces different bytes on different librsvg
and cairo versions, so byte-comparing them fails on any machine that is not the
one that last ran the build -- which is every CI runner. Instead `assets.lock.json`
records the SHA-256 of each PNG's *source SVG*, and `--check` verifies that hash
against the freshly generated source. That catches the failure that matters --
an SVG regenerated and committed while its rasters were left stale -- without
pinning the repository to one renderer build.

Two grids, not one. A 24-unit mark carries the ruled lines; a 16-unit mark
drops most of them for the favicon sizes. This is not belt-and-braces — the
24-grid does not divide into 16, so a 16px raster of it resamples the keyline
away entirely and the page loses its edges. Anything 32px or below uses the
small grid; 48px and above uses the full one.

See BRAND.md for the palette, the contrast measurements, and the usage rules.
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "docs", "assets", "brand")
FONT = os.path.join(ROOT, "branding", "fonts", "Fraunces72pt-Black.ttf")
LOCK = os.path.join(ROOT, "branding", "assets.lock.json")

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


# Favicons come off the small grid; anything larger off the full mark. Each
# entry is (source svg, width, output png) -- the whole raster set, in one
# place, so the build and the check cannot disagree about what should exist.
RASTERS = [
    ("mark-small.svg", 16, "favicon-16.png"),
    ("mark-small.svg", 32, "favicon-32.png"),
    ("mark.svg", 48, "favicon-48.png"),
    ("mark.svg", 180, "apple-touch-icon.png"),
    ("mark.svg", 480, "mark-480.png"),
    ("lockup.svg", 408, "lockup-408.png"),
    ("lockup-dark.svg", 408, "lockup-dark-408.png"),
    ("social-preview.svg", 1280, "social-preview.png"),
]


def vectors():
    """Every SVG this script owns, as {filename: content}.

    Generating into a dict rather than straight to disk is what lets --check
    compare without writing anything.
    """
    _, wordmark_light = wordmark_svg(64, PAL["K"])
    _, wordmark_dark = wordmark_svg(64, INK_DARK)
    return {
        "mark.svg": grid_svg(*MARK, scale=20),
        "mark-small.svg": grid_svg(*MARK_SMALL, scale=20),
        "lockup.svg": lockup_svg(PAL["K"]),
        "lockup-dark.svg": lockup_svg(INK_DARK),
        "wordmark.svg": wordmark_light,
        "wordmark-dark.svg": wordmark_dark,
        "social-preview.svg": social_svg(),
    }


def digest(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def fonttools_version():
    import fontTools

    return fontTools.version


def lock_for(svgs):
    """The provenance record: what each PNG was rendered from, and at what width."""
    return {
        "comment": (
            "Written by branding/build.py. Each PNG records the SHA-256 of the "
            "SVG it was rendered from, because PNG bytes vary by librsvg "
            "version and the source SVG does not."
        ),
        # The wordmark SVGs are glyph outlines emitted by fontTools' SVGPathPen,
        # whose exact output has changed between fontTools releases. Recording
        # the version turns "the SVG differs and nobody knows why" into a
        # specific, actionable message. CI pins the same version.
        "fonttools": fonttools_version(),
        "vectors": {name: digest(body) for name, body in sorted(svgs.items())},
        "rasters": {
            png_name: {"source": svg_name, "width": width, "source_sha256": digest(svgs[svg_name])}
            for svg_name, width, png_name in RASTERS
        },
    }


def png(svg_name, width, png_name):
    subprocess.run(
        ["rsvg-convert", "-w", str(width), "-o",
         os.path.join(OUT, png_name), os.path.join(OUT, svg_name)],
        check=True,
    )


def build():
    os.makedirs(OUT, exist_ok=True)
    svgs = vectors()

    for name, body in svgs.items():
        with open(os.path.join(OUT, name), "w", encoding="utf-8") as handle:
            handle.write(body)

    for svg_name, width, png_name in RASTERS:
        png(svg_name, width, png_name)

    with open(LOCK, "w", encoding="utf-8") as handle:
        json.dump(lock_for(svgs), handle, indent=2, sort_keys=True)
        handle.write("\n")

    for f in sorted(os.listdir(OUT)):
        print("  ", f)
    return 0


def check():
    """Report every way the committed assets are out of date with this script."""
    svgs = vectors()
    problems = []
    stale_vectors = []

    for name, body in sorted(svgs.items()):
        path = os.path.join(OUT, name)
        if not os.path.exists(path):
            problems.append(f"{name}: missing from docs/assets/brand/")
            continue
        with open(path, encoding="utf-8") as handle:
            if handle.read() != body:
                problems.append(f"{name}: committed file differs from what build.py generates")
                stale_vectors.append(name)

    if not os.path.exists(LOCK):
        problems.append("branding/assets.lock.json: missing -- run build.py to create it")
    else:
        with open(LOCK, encoding="utf-8") as handle:
            locked = json.load(handle)
        expected = lock_for(svgs)

        # A version mismatch is the likely explanation for a wordmark diff, and
        # saying so is the difference between a two-minute fix and an afternoon.
        locked_ft = locked.get("fonttools")
        if stale_vectors and locked_ft and locked_ft != expected["fonttools"]:
            problems.append(
                f"fontTools {expected['fonttools']} is installed but these assets "
                f"were generated with {locked_ft} -- pin fontTools=={locked_ft} "
                "before concluding the assets are stale"
            )

        for png_name, want in sorted(expected["rasters"].items()):
            if not os.path.exists(os.path.join(OUT, png_name)):
                problems.append(f"{png_name}: missing from docs/assets/brand/")
                continue
            got = locked.get("rasters", {}).get(png_name)
            if got is None:
                problems.append(f"{png_name}: not recorded in assets.lock.json")
            elif got.get("source_sha256") != want["source_sha256"]:
                problems.append(
                    f"{png_name}: rendered from a stale {want['source']} "
                    "-- the vector changed and the raster was not re-rendered"
                )
            elif got.get("width") != want["width"] or got.get("source") != want["source"]:
                problems.append(
                    f"{png_name}: lock records {got.get('source')} at {got.get('width')}px, "
                    f"build.py renders {want['source']} at {want['width']}px"
                )

        for png_name in sorted(set(locked.get("rasters", {})) - set(expected["rasters"])):
            problems.append(f"{png_name}: recorded in assets.lock.json but build.py no longer renders it")

    if problems:
        print(f"branding/build.py --check: {len(problems)} stale asset(s):", file=sys.stderr)
        for problem in problems:
            print(f"  - {problem}", file=sys.stderr)
        print(
            "\nRun `python3 branding/build.py` and commit the result. "
            "The generated assets are outputs, not sources -- do not hand-edit them.",
            file=sys.stderr,
        )
        return 1

    print(
        f"branding/build.py --check: {len(svgs)} vectors and "
        f"{len(RASTERS)} rasters are current."
    )
    return 0


def main():
    parser = argparse.ArgumentParser(description="Generate the katra brand assets.")
    parser.add_argument(
        "--check",
        action="store_true",
        help="verify the committed assets match this script instead of rewriting them",
    )
    args = parser.parse_args()
    return check() if args.check else build()


if __name__ == "__main__":
    sys.exit(main())
