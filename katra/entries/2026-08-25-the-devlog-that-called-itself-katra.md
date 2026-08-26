---
title: The devlog that called itself Katra
date: "2026-08-25"
time: "19:06:42"
closes:
    - stop-published-devlogs-claiming-to-be-katra
---

The referral mechanism was degrading the page it refers from. Every katra-published devlog served the embedded shell verbatim, which carries <title>Katra</title>. app.js does set the real title from data.json, but client-side — too late for any scraper, and too late for the tab of anyone still loading. So katra's product name was the browser tab, the bookmark, and the search-result title of somebody else's page. On getvect, the only published katra a reader can reach: their own pages title 'GetVect — raster to vector, on your machine' and their devlog titled 'Katra'.

A wrong title is worse than a missing preview, which is why this outranked the og: gap it was found alongside. A missing card costs you the readers who share a link. A wrong <title> shows the wrong name to every reader, in three places, and it is the one nobody thinks to check. getvect had already fixed their og: tags with a committed injector — eight tags, their own image, alt text — and the injector could not reach the title, because the title is not in the head they inject into.

Applied to serve and the hub too, not just build. Seeing 'Katra' locally and the project name in production is exactly how a title bug survives review, and the hub previously reported 'Katra' for every project it listed. One Shell function, three call sites.
