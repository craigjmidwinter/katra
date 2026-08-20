# Security

katra is pre-1.0 and written and maintained by one person. This policy is
scoped accordingly — it says what's actually true, not what a larger project
would say.

## Supported versions

The latest release only. There is no backport branch and no LTS; a fix ships
as the next tag, and you get it by upgrading (`brew upgrade katra`, or
re-running the `go install`/download steps in [README.md](README.md#install)).

## Reporting a vulnerability

Use GitHub's private vulnerability reporting rather than a public issue:

1. Go to the [Security tab](https://github.com/craigjmidwinter/katra/security)
   on this repository.
2. Click **Report a vulnerability**.
3. Describe the issue, the version (`katra --version`) and platform, and how
   to reproduce it.

That's the only channel — there is no dedicated security email, and a report
filed as a private advisory reaches the maintainer directly without going
through a public issue thread.

**Response is best effort.** This is one person maintaining a tool for their
own projects, not a product with a support contract, so there's no SLA to
promise. A real vulnerability report will get attention faster than a feature
request, but "faster" isn't "fast."

## Scope

A few things are documented, intentional behavior rather than
vulnerabilities — worth knowing before filing:

- **`katra serve` binds all interfaces and has no authentication**, by
  design. See [Known limitations](README.md#known-limitations) in the
  README: don't run it on a network you don't trust. A report that it's
  reachable from your LAN restates that limitation; a report that it lets a
  remote client do something beyond reading/serving the log (arbitrary file
  access outside `katra/`, code execution, and the like) is a real finding.
- **The commit gate and the git/Claude Code hooks execute locally**, as you,
  with your permissions — the same trust boundary as any git hook or shell
  script you've installed. They're not a sandbox and aren't meant to be.
- **`katra-mcp` speaks only stdio.** It has no network listener and is
  invoked by an MCP client's local process, not exposed to anything remote by
  itself.

Reports about any of these *behaving as documented* aren't actionable, but a
way to break out of those boundaries is exactly what this policy is for.
