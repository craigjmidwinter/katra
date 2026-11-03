---
title: The sandbox that was not one
date: "2026-11-03"
time: "17:15:54"
tags:
    - security
    - viewer
hash: 2ef6f25
stat:
    f: 23
    a: 970
    d: 75
summary: Embeds now run in an opaque origin, and nothing committed can run as the viewer.
closes:
    - isolate-embeds-and-lock-down-viewer-content
---

```embed
src: media/embed-isolation.html
height: 480
caption: The same fixture against main and this branch, in headless Chromium
```

`allow-scripts allow-same-origin` on a same-origin document is no sandbox: the frame can delete the attribute. On a hub, that origin is every registered project. Run against unmodified `main`, one hostile entry escaped ten different ways, and now it escapes none.

Three things turned up that the plan did not have.

The attribute was not enough. It protects one iframe, so opening a media file directly, or framing it from raw HTML with the attribute left off, still ran on the viewer origin. Every `media/` response now carries the `sandbox` directive itself, including `.svg` and extensionless files that `http.ServeFile` sniffs into HTML.

`script-src 'self'` had a hole. A raw `<iframe srcdoc>` inherits the viewer origin and may load a committed script from `media/`, which `'self'` cannot tell from `app.js`. With the fix switched off, the browser test reports exactly that escape. `media/` now refuses a same-origin script or worker fetch.

"No `unsafe-inline`" and "self-contained widget" collide, because a widget is one file with an inline script. Rejected `'unsafe-inline'` on embed responses. The server hashes the inline scripts in the HTML it is about to send, and a misread script simply stays blocked. The hash does not vouch for the code; the opaque origin contains it.

Also rejected: a per-embed opt-in, since an attacker could set it; `Access-Control-Allow-Origin: *` on media, which would let any website a reader visits read a LAN hub's media; and an HTML sanitizer dependency, since goldmark's safe mode strips raw HTML with no new code.
