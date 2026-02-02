# Windows Packaging Roadmap

This document tracks the Windows packaging plan for VideoTools.

## Targets

- MSIX installer for clean install/uninstall.
- WinGet manifest for easy discovery and updates.

## GitHub Actions

Workflow: `.github/workflows/windows-msix.yml`

Output artifacts:
- `VideoTools.msix`
- `dist/windows/winget/VideoTools.yaml`

## Release Hosting

- Dev builds: `git.leaktechnologies.dev` (internal).
- Public builds: GitHub releases at `https://github.com/LeakTechnologies/VideoTools`.

## Forgejo Actions (Dev Builds)

Workflow: `.forgejo/workflows/dev-packages.yml`

Outputs:
- `dist/windows/dev/*.zip` (dev package + build.json)
- `dist/windows/msix/VideoTools.msix` when Windows SDK is available on the runner.

Notes:
- Requires a Windows runner with MSYS2 (mingw-w64-x86_64-gcc).
- Linux dev packaging runs on Ubuntu runners and installs GStreamer dev packages.

## Release Flow

- Tag a release like `v0.1.1` in the public GitHub repo.
- GitHub Actions builds `VideoTools.msix`, generates the WinGet manifest, and uploads both to the release.

## Local Signing (Dev)

Create a dev signing cert and sign the MSIX:

```
.\packaging\windows\msix\sign.ps1 -CreateDevCert -InstallCert
.\packaging\windows\msix\sign.ps1
```

If SignTool fails, use Authenticode signing:

```
.\packaging\windows\msix\sign.ps1 -UseAuthenticode
```

Then install:

```
Add-AppxPackage -Path dist/windows/msix/VideoTools.msix
```

## Build Outputs (Planned)

- `packaging/windows/msix/` for MSIX packages and signing artifacts.
- `packaging/windows/winget/` for WinGet manifests and release metadata.

## Versioning Notes

- Current dev train: `v0.1.0-dev25` with platform suffix (`_win`).
- First user build will be promoted to `v0.1.1` and tagged as `v0.1.1-<hash>_win`.
- Next dev line becomes `v0.1.1-dev26`.

## Scope Guardrails

- Windows-only packaging work in this repo.
- Linux packaging is tracked separately and will be handled on Linux.
