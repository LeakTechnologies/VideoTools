# Repository root audit — dev96

The root was checked against tracked files, reference searches, build tags and
the existing module extraction plan. Cleanup preserves active source code and
historical evidence.

## Changes

- Remove `author_dvd_functions.go`: six functions and three types implement an
  unused DVD import/rip tab. Every reference to those declarations was confined
  to that file; no initializer, embed or platform side effects exist. Current
  Author and Rip entrypoints use their separate maintained implementations.

- Stop tracking `VideoTools_test.exe` (about 61 MB). It is a generated build
  artifact with no source or workflow references. Keep the local copy and ignore
  future copies. This removes it from the current tree, not past Git history.
- Move the pre-dev88 reconciliation to `docs/history/RECONCILIATION_DEV88.md`.
  It contains provenance evidence that should remain available.
- Move the July 16 dev53 triage snapshot to
  `docs/history/TRIAGE_2026-07-16.md`; its status statements are historical.
  Future active triage belongs in `docs/TRIAGE.md`.
- Move the isolated QR demo and its separate Go module to
  `scripts/legacy/qr-demo/`. It is an old prototype, not a production build
  target; relocation does not certify that prototype's compilation.
- Correct references and the obsolete DLL-packaging ignore comment.

## Retained files

Root Go files still form the application's `package main`. Module wrappers
connect application state and callbacks to `internal/app/modules`; those
internal packages have not replaced the wrappers. Native/stub and Windows/Linux
pairs are alternative build implementations, not duplicates. Embedded asset
declarations are also active. Their removal would break compilation or assets.

Retain manifests, license, README, agent rules, TODO/DONE and version files.
Keep the `_fyne` submodule: the Go module uses the local Fyne replacement.
Ignored local executable/sidecars/DLLs do not appear in GitHub. They remain
available for the existing local development environment.

Making the root a thin entrypoint requires the separate source extraction in
`REFACTOR_DEV30_PLAN.md`, including appState decoupling and build-path changes.
Do not mistake that unfinished refactor for safely deletable source files.

## Verification

Relocation reference checks and Git diff hygiene passed. The standard native
build, vet and full test gate passed on Windows. No player, encoding or workflow behavior changes are
part of this cleanup. Historical Git objects are retained; history is not
rewritten.
