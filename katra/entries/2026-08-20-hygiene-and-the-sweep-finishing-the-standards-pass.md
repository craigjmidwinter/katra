---
title: 'Hygiene and the sweep: finishing the standards pass'
date: "2026-08-20"
time: "16:22:28"
tags:
    - standards
    - hygiene
    - security
closes:
    - hygiene-leg-lint-config-clean-checkout-green-dead-code-sweep
    - security-performance-sweep
advances:
    - security-performance-sweep
    - hygiene-leg-lint-config-clean-checkout-green-dead-code-sweep
---

Scope expanded mid-pass again: the docs/brand standard grew into the full PROJECT-STANDARDS — hygiene, a security sweep, and a katra-practice section that asks for exactly what this repo just built. So this leg dogfoods the dogfood: an epic for the pass, two tasks created *specced* from birth (`task new --spec`), both pointing at [[standards-pass-gap-list-hygiene-and-sweep]] by node slug — the first slug-ref specs in the wild, after this morning's path-ref. The hygiene and sweep legs are running as parallel agents against that spec.

The sweep came back with the right shape of nothing: eighteen checks clean — path traversal, exec injection, SSRF, trackers, yaml — and the one real finding was a reachable goldmark XSS CVE, fixed by a two-line bump (v1.7.8 → v1.7.17, govulncheck now clean). The go directive moved 1.25.1 → 1.25.13 so a contributor's toolchain download isn't the vulnerable one. Two defense-in-depth items became tasks rather than silent fixes, and the history grep's only hits were katra's own secret-detector test fixtures — escalated per protocol, because the protocol is the point. Hygiene: a .golangci.yml that matches what CONTRIBUTING always promised, twenty findings fixed, zero suppressed, and a clean checkout that builds, tests and lints green in under five seconds. Mid-verification the machine ran out of disk entirely — 231MB free on the root volume — which is its own kind of sweep finding.
