<!--
See CONTRIBUTING.md for the conventions this codebase holds itself to.
-->

## What this changes

<!-- One or two sentences. Link the issue if there is one. -->

## Why

<!-- The behaviour that was wrong, or the capability that was missing. -->

## Checklist

- [ ] `gofmt -l .` is empty, `go vet ./...` is clean, `go test -race ./...` passes
- [ ] New behaviour has a test, and behaviour changes update the existing ones
- [ ] No change breaks the on-disk format contract ([docs/format.md](docs/format.md)) without a migration note — an unknown frontmatter key is still ignored, an unknown fence still degrades to a code block, an absent `type` still means `entry`, and an unknown `status` still degrades silently instead of erroring or vanishing from a count
- [ ] A new component is registered in `internal/core/render.go`, documented in `docs/components.md`, and exercised by `examples/entry.md`
- [ ] New or changed CLI surface (command, flag, MCP tool) is documented in `docs/cli.md` (and `docs/agents.md` for an MCP tool)
- [ ] Viewer asset changes (`internal/viewer/assets/`) are rebuilt into the binary before testing — assets are `go:embed`, so a stale build silently serves stale CSS/JS
