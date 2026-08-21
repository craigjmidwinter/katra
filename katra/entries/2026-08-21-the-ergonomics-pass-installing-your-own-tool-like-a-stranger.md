---
title: 'The ergonomics pass: installing your own tool like a stranger'
date: "2026-08-21"
time: "07:52:40"
tags:
    - standards
    - ergonomics
hash: ff82859
stat:
    f: 13
    a: 382
    d: 69
closes:
    - ergonomics-pass-fresh-install-failure-paths-upgrade-uninstall-paste-ability
advances:
    - ergonomics-pass-fresh-install-failure-paths-upgrade-uninstall-paste-ability
---

The standard grew an ERGONOMICS section overnight, so today katra gets installed by someone who has never installed it — an agent with a scrubbed HOME, no katra on PATH, and orders to follow only the published docs. The parts we already suspect will hurt: the README documents a release-download flow and no release has ever been cut, and the brew tap serves a binary from before the spec phase existed. The pass exists to turn suspicion into quoted terminal output.

The stranger's verdict: ninety seconds from paste to stamped entry — but only on the one install channel of three that isn't dead. The brew tap has no katra formula and the release-download commands 404 silently, because no release has ever been cut; the README was documenting a future. The fixes in flight reorder Install around what works today and say so plainly, document upgrade and uninstall for everything setup creates, and teach two error paths that were parroting raw git. Two findings became tasks (`setup --uninstall`, `hub install --dry-run`) rather than patches, and the strongest argument yet for cutting v0.1.0 is now a findings file: two-thirds of the install section is waiting on it. Also resolved: yesterday's full-disk scare was a transient — the Data volume has 49Gi free; my df had been reading the sealed system volume.

Fixes verified: the Install section now leads with the channel that works and says out loud why the other two are waiting on v0.1.0; upgrade and uninstall are documented down to the settings.json keys and the registry's self-pruning rule (which the fix agent caught me mis-framing — the registry entry lives exactly as long as katra/config.yml does, by design); and `git` missing from PATH now says so instead of gaslighting you about not being in a repo. Suite, lint, doctor all clean. The five ergonomics boxes tick: one-command install (go install, primary, honest), failure paths triggered-and-teaching, fresh install timed at ~90s against a five-minute bar, upgrade/uninstall tested and documented, and every documented command now pastes.
