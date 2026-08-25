---
title: Point the colophon at midwinter.io/katra/, with no tracking parameter
date: "2026-08-25"
time: "17:44:10"
summary: A link that ships inside other people's repos has to be the address, not a forwarding note
type: decision
status: accepted
entry: a-sign-on-the-shopfront-attributing-generated-deploys
---

Asked the steward, because the fleet is standing up analytics this week and that could have changed the answer. Both halves came back with reasons rather than preferences, and both checked out on inspection.

**The URL.** Not `https://craigjmidwinter.github.io/katra/`, which is what every other piece of katra metadata used:

    $ curl -sSI -L https://craigjmidwinter.github.io/katra/
    HTTP/2 301
    location: http://midwinter.io/katra/     <- plaintext
    HTTP/1.1 200 OK

    $ curl -sSI -L https://midwinter.io/katra/
    HTTP/2 200

It is a redirect, and it goes through plaintext `http://` before landing. This link ships inside other people's committed sites, so it should be the address rather than a forwarding note. `katra.midwinter.io` does not resolve and is not a target today.

**No query parameter.** Umami records the `Referer` header natively — referrers are a first-class view there, not something a parameter opts into — and `midwinter.io` is already registered as a tracked site. A `utm_source` would be redundant. It would also be the single most expensive thing here to change later: adding a marker afterwards is one line in one constant, and removing one from every devlog already published in someone else's repo is not possible at all.

Collection is not live yet — the steward reports the Umami collector resolving to a private LAN address pending a Cloudflare Tunnel. That changes nothing about the link, which starts being measurable when the tunnel lands.

**Knock-on, caught because of this check:** `server.json`'s `websiteUrl` and the .mcpb manifest's `homepage` both carried the redirecting URL. Both now point at the canonical one, and a parity test holds them together — fixed before the v0.1.0 tag, which is the only cheap moment for a published listing.
