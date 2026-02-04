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
- Requires a Windows runner with MSYS2 UCRT64 toolchain.
- Linux dev packaging runs on the `ubuntu` runner label.
- Release upload requires `FORGEJO_TOKEN` (repo scope) and optional `FORGEJO_API_URL` secrets.
- Optional signing uses secrets: `VT_SIGN_EXE=1`, `VT_SIGN_PFX_B64` (preferred), `VT_SIGN_PASSWORD`, `VT_SIGN_TIMESTAMP`.

## Release Flow

- Tag a release like `v0.1.1` in the public GitHub repo.
- GitHub Actions builds `VideoTools.msix`, generates the WinGet manifest, and uploads both to the release.

## Local Signing (Dev)

Create a dev signing cert and sign the MSIX:

```
.\scripts\windows\support\new-dev-cert.ps1
.\packaging\windows\msix\sign.ps1 -PfxPath packaging\windows\msix\VideoToolsDev.pfx -PfxPassword <password>
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
