---
title: The insertions that vanish on rebuild
date: "2026-08-25"
time: "20:03:35"
hash: de6b2ab
stat:
    f: 3
    a: 31
    d: 0
---

Verified rather than accepted. A host customises the generated page, then publishes a new entry and rebuilds: three injected head tags present before, zero after. Files katra writes — index.html, app.js, styles.css, data.json — are replaced wholesale; a separate file the host added alongside them survives untouched. So the hazard is precise: it is not that the output directory is wiped, it is that the four generated files are, and the page still renders perfectly afterwards while whatever the insertions powered has stopped.

The interval is the lesson, and it is not mine — the steward's. Four hours between this being hypothetical and being real for getvect: their injector was written in the afternoon and the first rebuild since came that night, carrying three new entries onto the page a launch thread was about to point at. Nothing about the rendered page would have shown it. A hypothetical failure in generated output is usually one that has not been rebuilt yet, and the rebuild's timing has nothing to do with when the risk was introduced.

Documented rather than fixed, deliberately. The extension point that would remove the hazard — a supported way for a host to inject head content that survives a build — is a product decision Craig holds, and he has already ruled once on the narrower version of it. What does not need a decision is telling people the truth about what build overwrites, which is the thing that would have saved getvect. That is now in the README limitations and in the CLI reference beside katra build.

The severity is inverted from how it was first filed, and the steward named it better than I had. This was reported as a wiping hazard, which is a thing people are already cautious about. What the reproduction actually shows is worse: partial respect for user edits is more dangerous than none. A generator that clobbered the whole directory would train correct caution. This one respects the file the host added — so they run a successful experiment, draw a correct conclusion from it, and generalise it exactly one step too far, onto the edit that had to live inside a generated file. The docs now say that rather than only stating which files get rewritten.
