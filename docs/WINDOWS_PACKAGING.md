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
- `dist/windows/<channel>/<version>_windows.zip` (package + build.json)
- `dist/windows/msix/<version>_windows.msix` when Windows SDK is available on the runner.

Notes:
- Requires a Windows runner with MSYS2 UCRT64 toolchain.
- Linux dev packaging runs on the `ubuntu` runner label.
- Release upload requires `FORGEJO_TOKEN` (repo scope) and `FORGEJO_API_URL`.
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

- Dev builds use `v0.1.1-dev26` and publish platform artifacts as `v0.1.1-dev26_windows`.
- Stable builds use `v0.1.1` and publish platform artifacts as `v0.1.1_windows`.

## Scope Guardrails

- Windows-only packaging work in this repo.
- Linux packaging is tracked separately and will be handled on Linux.
