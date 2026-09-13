---
title: Add Windows release artifacts
date: "2026-09-13"
time: "08:56:52"
tags:
    - release
    - windows
hash: 8c447c5
stat:
    f: 24
    a: 319
    d: 53
summary: Make Windows a compiled, packaged, and documented release target.
closes:
    - build-and-release-windows-binaries
---

Windows support required more than adding GOOS: syscall.Flock prevented compilation, path-derived slugs retained Windows separators, and Claude memory paths retained the volume colon. The release now uses build-tagged native locks and portable path handling.

Rejected shipping Windows tarballs or binaries without MCPB parity. Windows gets conventional zip archives, and MCPB maps Go's windows target to win32 while staging katra-mcp.exe.

```embed
src: media/windows-release-matrix.html
height: 480
caption: The release matrix after adding native Windows archives and MCP bundles.
```

Verified on Windows: go test ./... and go vet ./... pass; GoReleaser v2.18.1 validates the configuration; katra and katra-mcp cross-compile for windows/amd64 and windows/arm64; and the amd64 CLI executes natively. The local race run is unavailable because CGO is disabled, so the existing Linux CI race gate remains authoritative.
