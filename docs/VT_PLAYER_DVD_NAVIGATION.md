# VT_Player DVD playback and interactive navigation

## Requirement and current evidence

VideoTools should allow an ISO, VIDEO_TS directory or its parent to be dropped into VT_Player and load the DVD, including its interactive menu. This is a player-only requirement; other module/workflow/backend changes need their established approval.

Current code recognizes DVD sources in the player window drop route (main.go handleDrop), analyses topology in player_module.go and opens titles through InlineVideoPlayer.LoadDVD -> Engine.OpenDVD -> FFmpeg dvdvideo. This implementation is not real-disc acceptance. It offers title/chapter controls, not a working interactive DVD menu. No dvdnav button activation, mouse selection, directional navigation or menu-return interface is present in the player engine. FFmpeg menu-asset demuxing alone does not establish interactive navigation.

Static defects to verify before acceptance: chapter buttons call Seek(chapT / dur) although the player API takes seconds; DVD title-load errors are discarded; loadTitle launches asynchronous loading, so a source lifecycle check must include close and title replacement. Keep fixes confined to player glue and player internals. Encrypted-source acceptance remains open; Rip software decryption must not be assumed to apply to playback.

## Proposed order

1. Verify direct ISO and VIDEO_TS/parent drop loading on supplied local media. Correct player-only source routing, surfaced load errors and chapter units; exercise title changes and close during load.
2. Design a source-owned DVD navigation session using the existing libdvdnav/libdvdread foundation. Review available linked APIs and static build constraints first. Expose menu/domain changes, still frames, highlights, button selection/activation and menu return through the player API. Do not silently reintroduce libVLC or alter FFmpeg recipes.
3. Connect menu click coordinates through the displayed DVD aspect/letterbox transform and keyboard navigation. Respect source retirement, prohibit late navigation events from repainting a replaced source, and reconcile DVD discontinuities with audio/video clock reset.
4. Validate top/root menu, Play, scene selection, audio/subtitle selection, still/motion menus, return from title playback and authored-disc roundtrip. Confirm Windows and Linux behavior independently.

## Test checklist

- ISO, VIDEO_TS itself and its parent select the same disc topology.
- Reject non-DVD ISO or incomplete folders with a visible actionable error.
- Title and chapter selection reach the intended content and timestamp.
- Menu focus/highlight, mouse and keyboard activation agree with the authored menu.
- Still-frame wait and motion-menu loop behave correctly.
- Menu/title discontinuities do not leak stale audio/frames or drift.
- Close, replacement and rapid navigation retire workers cleanly.
- Encrypted and unencrypted sources are distinguished in evidence; no unsupported success claim.
- Native gate, real playback and runtime-log review precede landing each implementation slice.

## References

- Existing implementation: internal/ui/inline_player.go LoadDVD; internal/media/engine.go OpenDVD; player_module.go buildDVDPlayerView; main.go player drop route.
- FFmpeg dvdvideo documentation: https://www.ffmpeg.org/ffmpeg-formats.html#dvdvideo
- Related plan: VT_PLAYER_UPGRADE_PLAN.md. No implementation or menu acceptance is claimed by this document.
