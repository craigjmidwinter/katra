#!/usr/bin/env bash
#
# capture-screenshots.sh — regenerate docs/assets/screenshots/*.png
#
# Dependency: Playwright, invoked through `npx playwright` — it is NOT
# vendored into this repo (see CONTRIBUTING.md's "no new dependencies"
# rule). The first run downloads a Chromium binary if one isn't already
# cached; `npx --yes playwright install chromium` ahead of time gets that
# off the critical path.
#
# These screenshots are NOT CI-gated. Headless font rendering (hinting,
# subpixel AA, fallback-font selection) differs enough across CI runners
# that a byte-for-byte staleness check would flap on font metrics rather
# than catch real drift in the viewer. This script is the regeneration
# contract instead: rerun it whenever the viewer's visual output changes and
# commit the new PNGs by hand, in the same PR as the change that caused
# them. See CONTRIBUTING.md.
#
# What it does: `make build`, starts `katra serve` in the background against
# this repo's own katra/ (dogfooding — that's where the entry slug below
# lives), waits for the port, drives Playwright at four fixed routes, then
# kills the server. Re-runnable and safe: a trap always kills the server it
# started, even on error.
#
# Usage:
#   scripts/capture-screenshots.sh                # viewer shots + hub shot if :4200 is up
#   scripts/capture-screenshots.sh --viewer-only   # skip the hub shot entirely
#
# The hub shot (hub-projects.png) is captured against whatever `katra hub
# serve` daemon is already running on :4200 — this script does not start
# one, because the hub is a separate long-lived daemon
# (`katra hub install`), not something to spin up and tear down per run. If
# nothing is listening on :4200 the hub shot is skipped with a message
# rather than failing the run.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT_DIR="docs/assets/screenshots"
PORT=8123
BASE="http://localhost:${PORT}"
LOG="$(mktemp -t katra-serve-screenshots.XXXXXX)"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

shot() {
  local url="$1" out="$2" viewport="${3:-1440,900}"
  echo "==> ${out}  (${url}, ${viewport})"
  npx --yes playwright screenshot \
    --viewport-size="$viewport" \
    --wait-for-timeout=1500 \
    "$url" "${OUT_DIR}/${out}"
}

echo "==> make build"
make build

echo "==> starting: ./bin/katra serve --port ${PORT}"
./bin/katra serve --port "$PORT" >"$LOG" 2>&1 &
SERVER_PID=$!

up=0
for _ in $(seq 1 30); do
  if curl -fsS "$BASE" >/dev/null 2>&1; then
    up=1
    break
  fi
  sleep 0.5
done
if [[ "$up" -ne 1 ]]; then
  echo "katra serve never came up on ${BASE} — log at ${LOG}" >&2
  exit 1
fi

shot "${BASE}/#/overview" "viewer-overview.png"
shot "${BASE}/#/board" "viewer-board.png"
shot "${BASE}/#/node/the-fold-was-on-the-wrong-side-and-other-things-writing-the-docs-found" \
     "viewer-entry.png" "1440,1400"

cleanup
SERVER_PID=""

if [[ "${1:-}" == "--viewer-only" ]]; then
  echo "==> --viewer-only: skipping the hub shot"
  exit 0
fi

echo "==> checking for the hub daemon on :4200"
if curl -fsS "http://localhost:4200" >/dev/null 2>&1; then
  shot "http://localhost:4200/" "hub-projects.png"
else
  echo "==> nothing listening on :4200 — skipping hub-projects.png"
  echo "    start the daemon first if you need this one: katra hub serve"
fi

echo "==> done"
