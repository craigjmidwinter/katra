---
title: Publishing the chronicle, and the origin it was pointing at
date: "2026-08-25"
time: "19:00:18"
hash: 2efecfc
stat:
    f: 7
    a: 136
    d: 9
closes:
    - publish-katra-s-own-chronicle-at-devlog
---

Two defects, one config value. The docs site declared rel=canonical, og:url and og:image at https://craigjmidwinter.github.io/katra/ — an origin that 301s, and 301s to plaintext http:// before landing. So the site told every search engine its canonical home was a redirecting origin, and handed every social scraper an image URL many of them will not follow at all. jekyll-seo-tag resolves all three against url + baseurl, so the root cause was one line: url was the github.io origin rather than the one the site is actually served from. Same class of defect as server.json's websiteUrl earlier today, found the same way — by checking the link instead of reading it.

Why the chronicle needed a workflow rather than a commit. Pages was serving main//docs directly, and a branch build can only serve committed files — so publishing katra's own chronicle that way meant committing generated output and regenerating it by hand forever. Instead pages.yml renders katra/ into docs/devlog/ in CI, hands the whole tree to the same Jekyll build GitHub Pages runs, and deploys the artifact. docs/devlog/ is gitignored. The workflow asserts the chronicle survived the Jekyll pass rather than assuming it, because a silently dropped directory looks exactly like a successful build.

One switch left that is not mine to throw. The repository's Pages source is still 'Deploy from a branch' (build_type: legacy, main//docs). Until that becomes 'GitHub Actions', pages.yml's build job runs and proves the site assembles but the deploy job cannot publish. Deliberately left to Craig: it is a settings change on a public repo with a live site, and if the workflow were wrong the failure mode is the docs going down, so the build has to be seen green first.
