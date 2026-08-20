---
title: 'First push: green CI and a live site'
date: "2026-08-20"
time: "16:50:31"
tags:
    - release
    - ci
hash: eec9f28
stat:
    f: 1
    a: 7
    d: 2
summary: 'The pass goes public: 9 commits pushed, first-ever CI run green, Pages enabled and serving'
---

Craig authorized the push and stepped away. Nine commits went up — the spec phase, the whole standards pass — and the push itself found the last bug: GitHub's first real parse of ci.yml rejected `runner.temp` in job-level env (a context that only exists at step level), something actionlint had flagged and we'd misfiled as cosmetic. One commit later, the repo's first CI run in history came back green across all four jobs in 67 seconds. Pages wasn't enabled at all — every docs link in the shipped README pointed at a 404 — so it is now: legacy Jekyll from /docs, built, live, redirecting through midwinter.io. The one loose end: the redirect lands on http, not https; domain enforcement is Craig's dial.
