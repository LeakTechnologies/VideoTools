# Dev30 Refactor Plan (Gradual, Build-Safe)

Goal: clean project structure without destabilizing nightly builds.

## Version Model

- `v0.1.1-devN` = nightly/dev iterations (current cycle: dev30).
- `v0.1.1` = current stable/public baseline.
- Public bump (`v0.1.2`) happens only after release gates pass, not by dev-count depth.

## Refactor Scope for dev30

### Phase 1 (Complete)
- Root hygiene cleanup (removed scratch files, moved demo entrypoint out of root).

### Phase 2 (Next)
- Introduce `internal/app/` package boundaries for shared app logic.
- Move low-risk helper files first (pure helper logic only).
- Keep runtime behavior identical.

### Phase 3
- Extract module builders from root-level `package main` files into `internal/app/modules/`.
- Keep compatibility shims where needed during transition.

### Phase 4
- Move primary executable entrypoint toward `cmd/videotools/`.
- Update scripts/workflows only after app package entry is stable.

## Definition of Done for dev30

- File structure is cleaner with clear app boundaries (no new root clutter).
- Main menu/settings modularization remains stable after moves.
- Windows and Linux package jobs are green.
- Full module smoke coverage recorded via `docs/TESTING_MODULE_CHECKLIST.md`.
- Changelog/TODO/DONE reflect final scope and deferred carry-over.

## Guardrails

- Small commits only; each move should be reviewable and reversible.
- No behavior changes mixed into structural refactors unless required.
- If entrypoints/workflows must change, do it as dedicated commits with docs updates.
