---
title: The build that could not name itself
date: "2026-08-25"
time: "17:53:28"
closes:
    - report-a-real-version-from-go-install-builds
---

The bug, in the one install path that works. The README documents three ways to install katra and two of them have nothing behind them yet — Homebrew has no formula and there are zero releases to download — so go install is not merely the easiest documented path, it is currently the only one. And a go install binary reported 'dev', because the version is stamped with -ldflags and the go tool does not do that. A bug report from the only working install path could not identify its own build.

The fix is not the interesting part; the test is. runtime/debug build info gives the module version at runtime, so internal/buildinfo.Resolve prefers a link-time stamp when there is one and falls back to the module version otherwise. What a unit test cannot do is prove that, because under go test the main module's version is always '(devel)'. So the regression test shells out and runs the real go install, then asserts on what a user would type. Faulting the fallback makes it fail with exactly the user-visible symptom: katra --version = 'katra version dev'; a go install build still cannot name its own build.

Corrected a claim in the README while I was here. It said go install builds report 'dev' and asked bug reporters to name their commit instead — honest disclosure of a defect, which under the standard is a written exception rather than a lie. It is now simply false, so it had to go: go install …@v0.1.0 reports v0.1.0, and installing from a working tree reports a pseudo-version naming the commit. Verified: v0.0.0-20260825224452-8960a7e3ed71+dirty.
