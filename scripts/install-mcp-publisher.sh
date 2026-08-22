#!/usr/bin/env bash
#
# Install the exact mcp-publisher release used by both CI and release jobs.
# Keep the version and digest here so validation cannot silently drift from
# publication. This installer intentionally supports only the Linux/amd64
# GitHub Actions runners that invoke it.

set -euo pipefail

MCP_PUBLISHER_VERSION="1.8.1"
MCP_PUBLISHER_SHA256="a06c9096dcb9727c13555b6be26c7effa707b01f06a4c561ba7a3635443cf2cc"
MCP_PUBLISHER_ARCHIVE="mcp-publisher_linux_amd64.tar.gz"
MCP_PUBLISHER_URL="https://github.com/modelcontextprotocol/registry/releases/download/v${MCP_PUBLISHER_VERSION}/${MCP_PUBLISHER_ARCHIVE}"

destination="${1:-.}"
work_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT

mkdir -p "$destination"
curl -fsSL "$MCP_PUBLISHER_URL" -o "$work_dir/$MCP_PUBLISHER_ARCHIVE"
printf '%s  %s\n' \
  "$MCP_PUBLISHER_SHA256" \
  "$work_dir/$MCP_PUBLISHER_ARCHIVE" | sha256sum --check --status
tar -xzf "$work_dir/$MCP_PUBLISHER_ARCHIVE" -C "$destination" mcp-publisher
chmod +x "$destination/mcp-publisher"
