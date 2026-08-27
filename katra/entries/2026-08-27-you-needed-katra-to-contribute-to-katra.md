---
title: You needed katra to contribute to katra
date: "2026-08-27"
time: "11:03:34"
hash: 63aa95d
stat:
    f: 8
    a: 232
    d: 26
---

Verified on katra's own repo before touching anything. craigjmidwinter/katra is PUBLIC, .claude/settings.json is tracked, and all seven committed hooks called a bare katra with no guard. katra is not on a bare PATH — env -i PATH=/usr/bin:/bin sh -c 'command -v katra' returns nothing. So a contributor cloning katra to work on katra got seven hooks calling a binary they do not have, put there by katra's own installer. A bootstrap trap on the tool's front door.

One correction to the report's shape, measured rather than assumed. The missing binary exits 127, and only exit 2 blocks a PreToolUse call — so it is not a wall, it is 'katra: command not found' on every Bash command, every edit and every prompt. Not a gate that stops you; a project that looks broken from the first minute. Worth stating precisely because the fix is the same but the severity claim should match what happens.

The principle the steward offered is the fix, and it generalises: the absence of an enforcer is not a violation. A gate that fires because the tool is missing enforces nothing — it only fails, on a machine where nothing is wrong. Same family as the release that skipped the tap and passed, and as the gate that could not be satisfied: a signal that has come loose from the thing it is supposed to be about.

exec is the part that could have quietly made it worse. Guarding with a plain call would leave the wrapping shell to report its own exit code, and the pre-commit gate blocks by exiting 2 — so a naive guard would have disabled the gate it was guarding while looking like a fix. Verified all three ways: bare command blocks with 2, guarded with katra present still blocks with 2, guarded with katra absent exits 0 and prints nothing. The test asserts exec is present for exactly that reason.
