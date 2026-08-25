#!/usr/bin/env python3
"""Recompute the contrast table in BRAND.md and check it against what is written.

BRAND.md claims its ratios are "computed rather than eyeballed". That claim was
true and ungated: the numbers were right, and nothing could re-check them after
the next palette edit. This is the checker.

    python3 branding/contrast.py            # print the recomputed table
    python3 branding/contrast.py --check    # exit non-zero on any disagreement

Two things are checked, not one. The ratios must match what BRAND.md states to
the stated precision, and the verdict column must agree with the ratio: a pair
below WCAG AA's 4.5:1 has to be written down as failing. A correct number
beside a verdict that flatters it is the same error as a wrong number.

No dependencies -- the maths is four lines of the WCAG 2 definition.
"""

import argparse
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BRAND = os.path.join(ROOT, "branding", "BRAND.md")

# WCAG 2.x AA for body-size text. Large text is 3:1, but every row in this
# table is about body text or links, so one threshold is honest here.
AA_BODY = 4.5

# Colour names the contrast table uses without repeating the hex. Each one is
# cross-checked against BRAND.md's own palette table below, so this map cannot
# quietly disagree with the palette it is naming.
NAMES = {
    "Paper": ("Paper", "light"),
    "Surface": ("Surface", "light"),
    "Ink": ("Ink / body text", "light"),
    "Muted": ("Muted", "light"),
    "Accent": ("Accent", "light"),
    "Ink dark": ("Paper", "dark"),
    "Surface dark": ("Surface", "dark"),
}


def luminance(hex_colour):
    """Relative luminance, per the WCAG 2 definition."""
    channels = []
    for i in (0, 2, 4):
        c = int(hex_colour[i + 1 : i + 3], 16) / 255
        channels.append(c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4)
    r, g, b = channels
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(fg, bg):
    a, b = luminance(fg), luminance(bg)
    lighter, darker = max(a, b), min(a, b)
    return (lighter + 0.05) / (darker + 0.05)


def read_palette(text):
    """The Role | Light | Dark table, as {(role, side): hex}."""
    palette = {}
    for role, light, dark in re.findall(
        r"^\|\s*([^|]+?)\s*\|\s*`(#[0-9a-fA-F]{6})`\s*\|\s*`(#[0-9a-fA-F]{6})`\s*\|$",
        text,
        re.M,
    ):
        palette[(role, "light")] = light.lower()
        palette[(role, "dark")] = dark.lower()
    return palette


def resolve(spec, palette):
    """Turn one side of a "X on Y" pair into a hex colour.

    An explicit hex in the row wins; it is what a reader sees. A bare name is
    looked up in the palette table, which is how rows like "Muted on Surface"
    stay short without becoming unverifiable.
    """
    spec = spec.replace("**", "").strip()

    found = re.search(r"`(#[0-9a-fA-F]{6})`", spec)
    if found:
        return found.group(1).lower()

    name = spec.strip()
    # "Accent deep" and "Warm" only ever appear with their hex, so an
    # unresolvable bare name is a real gap rather than something to guess at.
    if name not in NAMES:
        raise KeyError(f"no palette entry for {name!r}, and the row states no hex")
    role, side = NAMES[name]
    if (role, side) not in palette:
        raise KeyError(f"{name!r} maps to {role!r}/{side}, which is not in the palette table")
    return palette[(role, side)]


def read_rows(text):
    """The Pair | Ratio | Verdict table, as (pair, stated_ratio, verdict) tuples."""
    rows = []
    for pair, stated, verdict in re.findall(
        r"^\|\s*(.+?\son\s.+?)\s*\|\s*\*{0,2}([0-9.]+):1\*{0,2}\s*\|\s*(.+?)\s*\|$",
        text,
        re.M,
    ):
        rows.append((pair.strip(), float(stated), verdict.strip()))
    return rows


def check_palette_coverage(text, palette):
    """Every hex the contrast table quotes must still be in the palette table.

    The contrast rows repeat their hexes inline, which is good for a reader and
    bad for drift: change `Muted` in the palette table and the contrast row
    still recomputes correctly against its own stale hex, so the ratio check
    stays green while the two tables describe different colours. This is the
    check that noticed that -- it was added because the first version of this
    script did not have it and a fault injection walked straight through.
    """
    known = set(palette.values())
    problems = []
    for hex_colour in sorted({h.lower() for h in re.findall(r"`(#[0-9a-fA-F]{6})`", text)}):
        if hex_colour not in known:
            problems.append(
                f"{hex_colour} is used in the contrast table but is not in the "
                "palette table -- one of the two was edited without the other"
            )
    return problems


def check(text):
    palette = read_palette(text)
    rows = read_rows(text)
    problems = check_palette_coverage(text, palette)

    if not rows:
        return problems + ["found no contrast rows in BRAND.md -- has the table moved?"], []

    computed = []
    for pair, stated, verdict in rows:
        fg_spec, bg_spec = pair.split(" on ", 1)
        fg, bg = resolve(fg_spec, palette), resolve(bg_spec, palette)
        actual = ratio(fg, bg)
        computed.append((pair, stated, actual, verdict))

        # BRAND.md states two decimals, so compare at that precision rather
        # than pretending to more.
        if round(actual, 2) != round(stated, 2):
            problems.append(
                f"{pair}: BRAND.md says {stated:.2f}:1, recomputed {actual:.2f}:1"
            )
            continue

        says_fail = "fail" in verdict.lower()
        really_fails = actual < AA_BODY
        if says_fail != really_fails:
            problems.append(
                f"{pair}: {actual:.2f}:1 "
                f"{'fails' if really_fails else 'passes'} AA at {AA_BODY}:1, "
                f"but the verdict reads {verdict!r}"
            )

    return problems, computed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check",
        action="store_true",
        help="exit non-zero if BRAND.md disagrees with the recomputed values",
    )
    args = parser.parse_args()

    with open(BRAND, encoding="utf-8") as handle:
        text = handle.read()

    problems, computed = check(text)

    if not args.check:
        for pair, stated, actual, verdict in computed:
            print(f"  {actual:6.2f}:1  (stated {stated:5.2f})  {pair}  -- {verdict}")

    if problems:
        print(f"\nbranding/contrast.py: {len(problems)} disagreement(s) with BRAND.md:", file=sys.stderr)
        for problem in problems:
            print(f"  - {problem}", file=sys.stderr)
        print(
            "\nRecompute the palette's ratios and update the table, or fix the "
            "palette. Do not adjust the stated number to match a colour you "
            "did not mean to change.",
            file=sys.stderr,
        )
        return 1

    print(f"branding/contrast.py: {len(computed)} ratios in BRAND.md all check out.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
