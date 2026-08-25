---
title: Ship an .mcpb bundle before the v0.1.0 tag
date: "2026-08-25"
time: "08:31:34"
hash: b096ba5
stat:
    f: 11
    a: 387
    d: 7
closes:
    - ship-an-mcpb-bundle-in-the-v0-1-0-release
---

Why a second package format. server.json declares an oci package and nothing else, which is right for the official MCP Registry and wrong for every venue that does not ingest it. Smithery — checked, not assumed — lists two routes only: a hosted streamable-HTTP endpoint, or an uploaded .mcpb bundle. A GitHub repo alone is not listable there. Katra runs no hosted endpoint and is not going to, so the bundle is the only door. The steward's evidence was a sibling project sitting un-listed for 25 days holding exactly the package set katra holds today.

Verified the spec rather than trusting the handoff. modelcontextprotocol/mcpb MANIFEST.md, read 2026-08-25: required fields are manifest_version, name, version, description, author, server; manifest_version is 0.3; server.type accepts binary; compatibility.platforms accepts darwin, linux, win32. The paste-ready manifest that came with the handoff was correct on all four points.

Rejected: a second goreleaser archives entry with formats: [zip]. It is the obvious move and it fails on the extension — goreleaser appends .zip from the format enum, and there is no custom-extension escape, so it produces katra_x_y.zip that an uploader wanting .mcpb will not take. Also rejected: assembling from the finished tarballs in a workflow step after goreleaser, which lands outside checksums.txt for the same reason as the after hook.

```embed
src: media/mcpb-proof.html
height: 480
caption: Snapshot build proof: the bundle's entry_point resolves, the bundled katra-mcp answers an MCP initialize, and all four .mcpb land inside the checksums.txt cosign signs.
```

Not verified, stated plainly: nobody has put a katra bundle through Smithery's submission flow, because no release exists to submit. release --snapshot proves the bundle builds, zips, checksums and structurally validates, and the smoke test above proves the bundled binary actually speaks MCP — which is a step past 'the manifest parses'. It does not prove an MCP client installs it. Expect to debug the install.
