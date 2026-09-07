# RIP_LOAD_DISC — Load Disc Directly From an Optical Drive

Rip module follow-up (`dev64`): let the user point the rip module at a
physical DVD in an optical drive instead of an ISO or a
`VIDEO_TS`/folder source. The normal load → scan → titles → rip path is
reused untouched; this feature only supplies the resolved source path.

## Overview

- New **Load Disc** pill button in the rip Source row, to the left of the
  existing `...` Browse button and Clear ISO.
- The module stays dumb: it calls `opts.OnLoadDisc()` and feeds the
  returned `VIDEO_TS` path into the existing `loadDisc()` entry point.
- All hardware knowledge lives in the root package, reusing the burn
  module's `detectOpticalDrives()`.

## Flow

1. User clicks **Load Disc**.
2. `appState.loadDiscFromDrive()` (`rip_module.go`):
   - `detectOpticalDrives()` — reused from the burn module (Windows:
     `GetLogicalDrives` + `DRIVE_CDROM` scan; Linux: `/dev/sr*`,
     `/dev/dvd`, `/dev/cdrom` + `/dev/disk/by-path` symlink targets).
   - Zero drives → error dialog `RipErrNoDrive` ("No optical drive detected").
   - One drive → use it directly.
   - Multiple drives → radio custom-confirm dialog (`RipSelectDriveTitle`,
     Load/Cancel); cancel returns `("", nil)` and nothing loads.
3. `resolveOpticalDriveVIDEOTS(drive)` resolves the disc structure:
   - **Windows** (`burn_windows.go`): drive letter `D:` → `D:\VIDEO_TS`
     must exist; otherwise `RipErrNoDVD` ("No DVD-Video structure found
     on the disc").
   - **Linux** (`burn_linux.go`): resolves `/dev/sr0` symlinks, parses
     `/proc/mounts` for the mount point, and joins `VIDEO_TS`; an
     unmounted drive reports `RipDriveNotMounted` ("No disc in the drive").
4. The resolved `VIDEO_TS` path is handed to `loadDisc()` in the rip
   module, which scans titles, populates the DiscSummary/Content Browser,
   and enables the normal rip action bar.

## Files

- `internal/app/modules/rip/view.go` — Load Disc button + `OnLoadDisc`
  wiring (button added to the Source row HBox; path fed to `loadDisc`).
- `internal/app/modules/rip/types.go` — `Options.OnLoadDisc func() (string, error)`.
- `rip_module.go` — `loadDiscFromDrive()` (drive detection + picker dialog).
- `burn_windows.go` / `burn_linux.go` — `resolveOpticalDriveVIDEOTS()`
  platform resolvers (named for burn interop; reused detection).
- `internal/i18n/{strings,en_ca,fr_ca,iu,iu_latin}.go` — new keys
  `RipLoadDisc`, `RipErrNoDrive`, `RipErrNoDVD`, `RipDriveNotMounted`,
  `RipSelectDriveTitle`. Inuktitut entries are machine-generated and
  flagged for human review.

## Constraints

- `loadDisc()` still enforces ISO / `VIDEO_TS` sources, so a non-DVD
  data disc in the drive surfaces the usual `RipErrNotDisc` path after
  resolution (never a crash).
- No new runtime dependencies: Windows uses Win32 APIs already in use by
  the burn module; Linux reads `/proc/mounts`.
- The rip module gains no platform imports — the seam is the single
  `OnLoadDisc` callback.

## Testing Checklist

1. Windows: DVD in drive → Load Disc loads titles, DiscSummary scans, rip
   works end to end (scenes + full movie).
2. Windows: drive with a data (non-DVD) disc → user-visible error, no crash.
3. Windows: empty drive → error, no crash.
4. Multiple optical drives → picker dialog appears; Load loads the
   selected drive; Cancel returns to the module unchanged.
5. Linux: mounted DVD under `/media/<user>/<label>` → resolves via
   `/proc/mounts`; unmounted device → "No disc in the drive".
6. French + Inuktitut (syllabics and Latin) UI render the new strings.
7. Clear ISO while a drive-loaded disc is present → Source cleared, Menu
   Preview returns to the compact strip, no stale state.