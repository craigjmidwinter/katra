# Design: Windows release support

Status: accepted, 2026-09-13.
Task: `build-and-release-windows-binaries`.

## Outcome

Tagged releases ship `katra.exe` and `katra-mcp.exe` for Windows on amd64 and
arm64. Windows archives use zip, while the existing macOS and Linux archives
remain tarballs. The Windows MCP server also ships as an `.mcpb` bundle covered
by the release checksum and signature.

This is release support rather than only a build-matrix claim: both commands
must compile on Windows, the packaged executable names must be usable there,
and CI must inspect the Windows archive produced by GoReleaser.

## Portable state locking

The current build imports `syscall.Flock` directly from `reconcile.go`, which
prevents every Windows build even though the lock only protects Katra's local,
gitignored state ledgers. Keep the fail-open behavior and move the operating
system call behind a small build-tagged helper:

- Unix uses the existing advisory `flock` call.
- Windows uses `LockFileEx` and `UnlockFileEx` from `kernel32.dll`, reached
  through the standard library. This preserves cross-process exclusion without
  adding a dependency solely for two system calls.

The common code remains responsible for opening and closing `.lock`; the
platform helper acquires the lock and returns an unlock function. An acquisition
failure continues to run the protected operation unlocked, matching the
documented fail-open contract.

## Release artifacts

Add `windows` to both GoReleaser build matrices. GoReleaser supplies the `.exe`
suffix to Windows binaries. Its supported per-OS archive override produces zip
archives on Windows, the conventional format for users who do not have `tar`.
The release names remain `katra_<version>_<os>_<arch>`.

The MCPB format names Windows `win32`, not `windows`. The manifest therefore
declares `win32`, while the packaging parity test maps Go's `windows` value to
that manifest value. The bundle script stages `server/katra-mcp.exe` and renders
that path into both `entry_point` and `mcp_config.command`; Unix bundles retain
the extensionless name. A test must prove the Windows bundle shape and keep a
negative guard using a genuinely unshipped platform.

## Verification and documentation

CI runs the Go suite natively on Windows and cross-compiles arm64 there. Its
release dry run lists tarballs and zip files, checks both `.exe` binaries inside
the Windows amd64 zip, and retains the existing Linux workflow-surface smoke
test. README download instructions gain a PowerShell path, and the release
runbook names six archives and six MCP bundles rather than four.

The OCI image remains Linux-only. It is MCP Registry transport packaging, not
the native release matrix, so adding Windows there would be both impossible and
outside this outcome.
