# FFmpeg DLL Bootstrap — Architecture & Troubleshooting

## Overview

VideoTools uses FFmpeg in two distinct ways:

1. **Statically linked at compile time** — CGo links the FFmpeg C API (`libavcodec`, `libavformat`, etc.) as static `.a` archives into `VideoTools.exe`. This is what provides video/audio playback, frame decoding, and all media engine features.

2. **Shared DLLs at runtime** — The bundled `ffmpeg.exe` and `ffprobe.exe` CLI tools are linked against shared `.dll` FFmpeg libraries. These DLLs must be loadable at runtime whenever a module spawns an FFmpeg subprocess (Convert, Audio, Thumbnail, Rip, etc.).

The DLLs are **not** needed by `VideoTools.exe` itself (it is fully statically linked). They are only needed by the bundled `ffmpeg.exe` / `ffprobe.exe` CLI executables.

## DLL File Structure

A release package contains:

```
VideoTools/
├── VideoTools.exe          # Statically linked — no runtime FFmpeg DLL deps
├── ffmpeg.exe              # Shared-linked — needs DLLs
├── ffprobe.exe             # Shared-linked — needs DLLs
├── DLL/
│   ├── avcodec-61.dll
│   ├── avformat-61.dll
│   ├── avutil-59.dll
│   ├── swscale-8.dll
│   ├── swresample-5.dll
│   ├── avfilter-10.dll
│   ├── liblzma-5.dll       # Transitive dep of avformat
│   └── ... (other transitive deps)
```

> **DLL versioning:** The ABI version number in each DLL name (e.g., `-61`) must match the FFmpeg build that `VideoTools.exe` was compiled against. If a release bundles mismatched DLLs, `ffmpeg.exe` / `ffprobe.exe` will fail to load with a missing-entry-point or DLL-not-found error.

## DLL Search Order

At startup, `appcfg.AddFFmpegDllsToPath()` (in `internal/app/appcfg/ffmpeg_bootstrap.go`) finds the DLLs and prepends their directory to `PATH`:

1. `<exe-dir>/DLL/` — the CI/release bundled subfolder (primary)
2. `<exe-dir>/` — flat DLLs next to the exe (local dev builds, flattened extraction)
3. `%LOCALAPPDATA%\VideoTools\DLL` — legacy download path

Once `PATH` includes the DLL directory, both `ffmpeg.exe` and any FFmpeg internal `LoadLibrary` calls resolve the DLLs correctly.

## Startup Validation (`--dllcheck`)

VideoTools now runs a **live smoke test** at startup to catch DLL issues before the user tries to use a feature:

1. **Existence check** — Every expected DLL is verified on disk.
2. **ffprobe smoke test** — If `ffprobe.exe` is present next to `VideoTools.exe`, it is launched with `-version`. If this fails, the Windows loader could not resolve the DLLs. The stderr output is captured and displayed.

If validation fails:
- A warning is logged to the session log.
- A **non-blocking Fyne error dialog** is shown at startup explaining the issue and how to fix it.
- VideoTools continues to boot — only player/encoding features that depend on FFmpeg are affected.

### CLI diagnostic flag

Run `VideoTools.exe --dllcheck` from a terminal to print a full diagnostic report without launching the GUI:

```
VideoTools.exe --dllcheck

=== FFmpeg DLL Diagnostics ===
DLL directory: C:\Users\...\VideoTools\DLL
DLL files found: 8
  avcodec-61.dll (48373760 bytes)
  avformat-61.dll (14126080 bytes)
  ...
PATH entries: 15
  FFMPEG/DLL: C:\Users\...\VideoTools\DLL
ffprobe.exe: C:\Users\...\ffprobe.exe (12320768 bytes)
VALIDATION: OK
```

## Common Issues

### "FFmpeg DLLs Not Found" at startup

**Cause:** The `DLL/` folder is missing or misplaced relative to `VideoTools.exe`.

**Fix:**
1. Verify the release ZIP was extracted completely (not just `VideoTools.exe`).
2. Ensure the `DLL/` folder sits **next to** `VideoTools.exe` (not inside it).
3. Re-extract from the latest release package.

### "FFmpeg DLL Load Error" at startup

**Cause:** DLLs are present but fail to load — version mismatch or corrupted download.

**Fix:**
1. Run `VideoTools.exe --dllcheck` to see the full diagnostics.
2. If the smoke test shows a missing-entry-point error, the DLLs are from a different FFmpeg build than VideoTools expects.
3. Download the latest release package and re-extract.

### DirectX / OpenGL crash on startup (not DLL-related)

If VideoTools crashes before the GUI appears with an OpenGL or GPU-related error, this is **not** a DLL issue. See `docs/INSTALL_WINDOWS.md` for GPU troubleshooting.

## DLL Build Pipeline (CI)

All three CI pipelines (Forgejo dev-packages, GitHub release, GitHub MSIX) follow the same architecture:

```
Source build (static)              BtbN download (shared)
  ┌──────────────┐                 ┌──────────────────────────┐
  │ x264 → .a    │                 │ ffmpeg-master-latest      │
  │ x265 → .a    │                 │ -win64-lgpl-shared       │
  │ FFmpeg → .a  │                 │   → avcodec-61.dll etc.  │
  └──────┬───────┘                 │   → ffmpeg.exe            │
         │                         │   → ffprobe.exe           │
         │ CGO_LDFLAGS             └──────────┬───────────────┘
         ▼                                    ▼
  VideoTools.exe                     DLL/ + ffmpeg.exe
  (statically linked)               (shared-linked CLI tools)
```

The static `.a` libs and the shared `.dll` files are built from **different FFmpeg source trees**. This is an inherent risk: if the BtbN master branch advances its ABI between CI runs, the DLLs can become incompatible with the statically-linked binary.

**Mitigation**: The `ci-build.ps1` packaging step runs `objdump` on every DLL in the `DLL/` directory to detect transitive DLL dependencies, and copies any missing ones from `C:\msys64\ucrt64\bin\`. `ExpectedFFmpegDLLs()` also checks for `liblzma-5.dll` at runtime. The GitHub workflows now perform the same objdump scan.

**Remaining risk (BUG-013)**: The BtbN download URL uses `latest`, which is a moving tag. When BtbN bumps a major FFmpeg version (changing e.g. `-61` to `-62`), the hardcoded ABI numbers in `ExpectedFFmpegDLLs()` and the DLL filenames will break. This should be pinned to a specific release tag.

### CI Pipeline Details

| Pipeline | Static FFmpeg | Shared DLLs | FFmpeg/ffprobe | Transitive deps |
|----------|--------------|-------------|----------------|-----------------|
| Forgejo dev-packages.yml | Source-built (FFmpeg 8.1 + x264 + x265) | BtbN `latest` lgpl-shared | Bundled | objdump scan from MSYS2 |
| GitHub release.yml | Source-built (FFmpeg 8.1 + x264 + x265) | BtbN `latest` lgpl-shared | Bundled | objdump scan from MSYS2 |
| GitHub windows-msix.yml | Source-built (FFmpeg 8.1 + x264 + x265) | BtbN `latest` lgpl-shared | Bundled | objdump scan from MSYS2 |

**Important**: The `VideoTools.exe` binary does NOT need the DLLs. It is fully statically linked. The DLLs are only needed by `ffmpeg.exe` and `ffprobe.exe` which are bundled for subprocess use.

## Zero-Touch Guarantee

VideoTools is designed to work immediately after extracting a release ZIP:

1. Extract the ZIP anywhere.
2. Run `VideoTools.exe`.
3. Everything works — no PATH setup, no manual DLL installation, no VC++ redistributable required.

If this guarantee is broken (startup validation shows DLL errors), please:
1. Run `VideoTools.exe --dllcheck` and capture the output.
2. Open an issue at the project issue tracker with the diagnostics output.
3. Re-install from the latest CI build as a temporary workaround.

## Developer Setup

### Windows (local dev build)

1. Install MSYS2 with `mingw-w64-ucrt-x86_64-toolchain`, `nasm`, and `mingw-w64-ucrt-x86_64-cmake`.
2. Run `scripts/windows/build-ffmpeg-shared.ps1` to build FFmpeg and install to `C:\ffmpeg-shared\`.
3. Run `scripts/windows/build.ps1` to compile VideoTools.

The build script sets `CGO_CFLAGS` and `CGO_LDFLAGS` automatically. If building manually with `go build`, ensure FFmpeg headers are at a path that matches the `#cgo` directives or set the environment variables.

### Linux

FFmpeg is fully statically linked via pkg-config. Install `libavcodec-dev`, `libavformat-dev`, etc. from your package manager, or use the CI build script.
