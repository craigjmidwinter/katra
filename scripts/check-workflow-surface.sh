#!/usr/bin/env bash
# Verify that a built or packaged katra binary matches the published
# spec-to-stamp workflow. v0.1.0 predates these three task-spec surfaces, so
# release candidates must run this against the artifact, not only source tests.

set -euo pipefail

katra_bin="${1:-katra}"

"$katra_bin" task spec --help >/dev/null
task_new_help="$("$katra_bin" task new --help)"
task_list_help="$("$katra_bin" task list --help)"

grep -Fq -- '--spec string' <<<"$task_new_help"
grep -Fq -- 'todo|specced|doing|done|cut' <<<"$task_list_help"

printf 'workflow surface: task spec, task new --spec, and specced filter OK\n'
