---
title: Isolating playable embeds
layout: default
parent: Design notes
nav_order: 12
description: >-
  P1 of the playable-log work. Embeds run in an opaque origin, every page and
  media response carries a CSP, and raw HTML can be stripped per store.
---

# Design: isolating playable embeds

- **Status:** accepted
- **Date:** 2026-11-03

## Problem

The `embed` sandbox was `allow-scripts allow-same-origin`, which on a
same-origin document is no sandbox at all: the frame can remove its own
attribute. Under `katra hub serve` that origin is shared by every registered
project, so one contributor's chart could read or rewrite every other
project's pages. Playable widgets make author JavaScript routine, so this has
to be fixed before the log asks for more of it.

The threat is committed content: a malicious or compromised contributor, and
anyone viewing what they published. `serve` and `hub serve` keep their
documented no-auth, all-interfaces design; there are no session cookies to
protect, so the goal here is that entry and embed content cannot run as the
viewer origin.

## Decisions

**Opaque origin, no opt-in.** The sandbox is exactly
`allow-scripts allow-pointer-lock`. An earlier sketch offered a per-embed key
restoring `allow-same-origin` for trusted cases. It is rejected: the key would
sit in the same committed markdown the sandbox defends against, so the attacker
could set it. Nothing katra ships needs same-origin.

**The sandbox travels with the response, not just the attribute.** An iframe
attribute only protects that iframe. Opening `media/x.html` directly, or
framing it from raw HTML without the attribute, would run it on the viewer
origin. So every `media/` response carries a `sandbox` CSP directive, whatever
its type: `.svg` runs script too, and `http.ServeFile` sniffs extensionless
files into `text/html`.

**Two policies.** Pages get a strict policy, as a header from `serve`/`hub
serve` and a `<meta>` in every generated page. Media gets its own header
policy. Both are written once in `internal/viewer/security.go`.

**Inline widget scripts are allowed by hash, not `'unsafe-inline'`.** A widget
is usually one self-contained HTML file, so its inline `<script>` has to run,
and the embed policy may not use `'unsafe-inline'`. The server hashes each
inline script in the HTML it is about to send. A file it cannot parse fails
closed: the script is blocked, not trusted. The hash does not make author code
trustworthy; the opaque origin is what contains it. What the hash adds is that
markup injected at runtime and inline event handlers stay inert.

**`'self'` needs a guard.** `script-src 'self'` on the page would admit a
committed `media/x.js`, reachable from a raw `<iframe srcdoc>` that inherits
the viewer origin. So a `media/` request whose `Sec-Fetch-Site` is
`same-origin` and whose destination is a script or worker is refused. Sandboxed
embeds are unaffected: their requests come from an opaque origin.

**CORS for `null` only.** Sandboxed multi-file builds fetch their siblings from
an opaque origin. `media/` answers `Access-Control-Allow-Origin: null` to
`Origin: null`, not `*`, so ordinary websites a reader visits cannot read a LAN
hub's media.

**`rawHTML: allow|sanitize`.** `sanitize` turns on goldmark's safe mode: raw
HTML is dropped and `javascript:` links are not emitted. Components still
render, because they are katra's own output. Stripping rather than allow-listing
avoids a new dependency and a hand-written sanitizer. `allow` stays the default
because the format is the public API, and the CSP already neutralises what
would run. An unrecognised value means `sanitize`: a typo in a security switch
must not turn it off.

## What each surface can enforce

| Control | `serve` / `hub serve` | `katra build` (static) |
| --- | --- | --- |
| Sandbox attribute | yes | yes |
| CSP | header and `<meta>` | `<meta>` only |
| `frame-ancestors`, `sandbox` directive | header | not possible; set them on the host |
| `nosniff` | header | set it on the host |
| `no-referrer` | header and `<meta>` | `<meta>` |
| Inline-script hashes on media | yes | not applicable, no media headers |
| Media opened directly stays opaque | yes | no; it runs on the site's origin |
| Multi-file embed `fetch` | CORS for `null` | depends on the host |

A static site has no katra sessions or server to protect, but a host that shares
an origin with something sensitive should serve `media/` with the same headers.

The session-cookie requirement does not apply: `serve` and `hub serve` set no
cookies, and a test keeps it that way.

## Migration

Existing content that stops working is listed in
[Components](../components#migrating-to-the-opaque-origin-sandbox).

## Rejected

- **A separate embed origin.** Stronger, but `serve` has one port and a static
  build has one host. The opaque origin gives the isolation without either.
- **`'unsafe-inline'` on embed responses.** Simple, and arguably harmless inside
  the sandbox, but it fails the policy review and hashes cost very little.
- **`bluemonday` or `x/net/html`.** A dependency, for an allow-list nobody has
  asked for yet.
