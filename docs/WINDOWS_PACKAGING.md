# Windows Packaging Roadmap

This document tracks the Windows packaging plan for VideoTools.

## Targets

- MSIX installer for clean install/uninstall.
- WinGet manifest for easy discovery and updates.

## Build Outputs (Planned)

- `dist/windows/msix/` for MSIX packages and signing artifacts.
- `dist/windows/winget/` for WinGet manifests and release metadata.

## Versioning Notes

- Current dev train: `v0.1.0-dev25`.
- First user build will be promoted to `v0.1.1`.
- Next dev line becomes `v0.1.1-dev26`.

## Scope Guardrails

- Windows-only packaging work in this repo.
- Linux packaging is tracked separately and will be handled on Linux.
