---
title: Katra is a publishable chronicle, and the routing that makes it one is v0.2.0
date: "2026-08-25"
time: "19:58:37"
summary: The attribution footer already asserted this; the limitation ships written down rather than discovered
type: decision
status: accepted
---

The steward owns the katra-as-devlog question and decided it: **blog, not viewer.** The reasoning that actually settles it is that the viewer answer contradicts what has already shipped — a viewer does not need an attribution footer, because nobody arrives at it from outside. The colophon has been asserting katra is a publishing tool for a while; this makes the assertion true rather than withdrawing it.

**But the routing is v0.2.0.** Putting a medium project in front of a release that is otherwise ready would turn a decision into a delay. v0.1.0 ships with the limitation stated in the README instead: entries share one social card, and route changes emit no pageview. A limitation a user discovers is a defect; a limitation the README states is a boundary.

Two constraints on the v0.2.0 scope, both drawn from the risk analysis rather than added afterwards:

- **The link scheme comes from one source.** Go writes the routes into `data.json` and `app.js` consumes them, rather than the scheme existing in both places. Two renderers of one scheme is the palette-drift failure in a new costume, and this fleet has been caught by that shape twice in a day.
- **Every published `#/node/<slug>` link keeps working, permanently.** The redirect shim is a precondition of the change, not a follow-up — published links outrank tidiness, the same rule the address convention set.

One correction that changed the plan on both sides: the head-injection gap and the routing gap are **necessary together, not alternatives**. Routing makes pageviews possible; it does not make them present, because katra emits no analytics script and knows no deploy URL. The steward had analytics sequenced behind "publish the chronicle" on the assumption that publishing made it measurable. It does not — and a flat chart next week would have read as low readership rather than as absent instrumentation, which is a distortion that would have been taken as a fact about Craig's writing.
