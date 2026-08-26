#!/usr/bin/env bash
# Assemble one .mcpb bundle -- an MCP Bundle, which is a zip -- around a
# freshly built katra-mcp binary.
#
#   build-mcpb.sh <binary> <os> <arch> <version> <outdir>
#
# Called from .goreleaser.yml as a post-hook on the katra-mcp build, once per
# target. It has to run there rather than in an `after` hook: goreleaser
# computes checksums.txt before `after` hooks fire, and an asset outside that
# file is an asset the release footer's verify instructions cannot cover.
#
# See docs/design/mcpb-bundle.md for why the bundle exists at all.
set -euo pipefail

if [ "$#" -ne 5 ]; then
  echo "usage: build-mcpb.sh <binary> <os> <arch> <version> <outdir>" >&2
  exit 2
fi

binary=$1
goos=$2
goarch=$3
version=$4
outdir=$5

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
template=$root/packaging/mcpb/manifest.json.tmpl

[ -x "$binary" ] || { echo "build-mcpb: not an executable: $binary" >&2; exit 1; }
[ -f "$template" ] || { echo "build-mcpb: missing template: $template" >&2; exit 1; }

# The manifest declares only what the goreleaser matrix actually builds. A
# bundle for a platform we do not ship is a claim, not a package.
case "$goos" in
  darwin | linux) ;;
  *)
    echo "build-mcpb: refusing to bundle unshipped platform: $goos" >&2
    exit 1
    ;;
esac

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

mkdir -p "$stage/server"
cp "$binary" "$stage/server/katra-mcp"
chmod 0755 "$stage/server/katra-mcp"

# The template carries a placeholder rather than being goreleaser-templated, so
# `python3 -c ... json.load` in the test suite can read it as-is.
sed "s/__VERSION__/${version}/g" "$template" > "$stage/manifest.json"

mkdir -p "$outdir"
outdir=$(cd "$outdir" && pwd)  # the zip runs from $stage, so this has to be absolute
bundle=$outdir/katra_${version}_${goos}_${goarch}.mcpb
rm -f "$bundle"

# -X drops the extra file attributes that vary between the machine that built
# a bundle and the one that rebuilds it, so the same inputs zip to the same
# bytes.
( cd "$stage" && zip -q -X -r "$bundle" manifest.json server )

echo "build-mcpb: wrote $bundle"
