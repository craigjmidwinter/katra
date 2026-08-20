---
title: 'Hub: JSON API for native clients (/api/hub.json)'
date: "2026-07-27"
time: "16:35:41"
summary: cross-project snapshot as data, so a macOS menu bar client can read the hub without scraping its HTML
type: task
status: done
effort: S
entry: the-placeholder-that-ate-every-entry-and-why-nine-katras-had-no-pictures
---

The hub renders HTML for browsers. A native client needs the same portfolio
snapshot as data: every registered katra, everything in flight, and the recent
log — each row carrying both a viewer URL and the on-disk repo root, so a client
can either open a page or shell out to the CLI in the right directory.

Reuses the existing livereload SSE stream as the change signal rather than
adding a second notification channel.

Consumed by katra-bar (the projagent repo, converted to katra's menu bar client).
