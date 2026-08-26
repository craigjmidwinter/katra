---
title: Emit title and og:title from data.json, and nothing else
date: "2026-08-25"
time: "19:06:34"
summary: A renderer that improves one field while degrading two is not an improvement
type: decision
status: accepted
entry: stop-published-devlogs-claiming-to-be-katra
---

The tempting version of this fix emits the whole social block from the store's config. It would make things worse.

getvect's data.json still carries katra's **default** description — `A living, committed chronicle of development.` — because they never changed it, while their hand-injected og:description is specific and good. A renderer emitting og:description from config would silently overwrite a tailored value with a placeholder. A field nobody has filled in is not evidence of intent.

So the line is drawn at what the renderer can actually guarantee:

- **title** and **og:title** — the store's own title, which is real data. Emitted.
- **og:description** — a default wearing the shape of data. Not emitted.
- **og:image** — katra knows no asset. Not emitted.
- **og:url** — katra does not know where it will be deployed. Not emitted.

Hosts keep those, and any injector they already built keeps working rather than being fought by the renderer.

The substitution is one exact string replacement against the embedded index.html, which is a quiet way to break: an innocuous edit to the asset removes the placeholder and every deploy goes back to being titled Katra with nothing failing. So Shell errors rather than returning its input unchanged, and a test asserts the placeholder still exists. Faulted by editing the title out of the asset; it fails with the actionable message rather than silently doing nothing.
