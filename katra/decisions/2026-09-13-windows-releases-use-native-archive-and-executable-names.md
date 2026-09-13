---
title: Windows releases use native archive and executable names
date: "2026-09-13"
time: "09:03:29"
tags:
    - release
    - windows
summary: Ship Windows as zip files with .exe binaries and map windows to win32 in MCPB metadata.
type: decision
status: accepted
entry: add-windows-release-artifacts
---

Windows users receive zip archives containing katra.exe and katra-mcp.exe. MCPB bundles also retain the .exe suffix and declare the specification's win32 platform value, while GoReleaser continues to use windows internally. The OCI registry image remains Linux-only because it is a transport package, not the native distribution matrix.
