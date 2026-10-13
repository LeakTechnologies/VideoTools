# Contextual help and chapter choices — VideoTools dev101

## Presentation contract

Required labels, values, available choices, errors, output previews and feature availability stay visible. Optional explanations occupy a compact information button beside the relevant control. Hover opens help after a short delay; leaving dismisses it. Clicking or keyboard activation opens the same explanation even with hover disabled. Popups wrap within the viewport and long content scrolls. Navigation retires pending and visible tooltips.

The existing tooltip helper discarded its hover state and never installed the dependency's required window layer. Durable controls now own their hover events and use Fyne popups. This changes presentation only; encoding/player/workflow behavior is unchanged except removing Rip's unused hidden disc player.

## Unlocked module audit

| Module | Treatment |
|---|---|
| Rip | Per-title chapter count and numbered choices; whole title, every chapter separately, chosen chapters. Optional custom groups collapse; syntax is hover help. Output naming explanation beside Title. No hidden player reads on scan/card focus. |
| Convert | Optional crop, transformations, aspect handling, presets, hardware, cache/log paths and snippet explanations beside controls. Keep output path, judder/chapter-loss warnings, remux restrictions and detection notes visible. |
| Settings | Explanations beside affected preference; keep availability/status visible. |
| Upscale | Sampling, quality, bitrate, colour and motion explanations beside controls; keep unavailable backends visible. No new Upscale backend scope. |
| Author | Menu theme, motion background, logo safe area and extras help beside controls; menus-disabled warning visible. |
| Inspect / Compare / Thumbnails | Introductory prose becomes optional help beside existing actions/source controls. |
| VT_Player / Queue / Merge | Source instructions, task labels and operation status retained. Shared existing tile/select tooltips repaired; no playback engine change. |
| Burn | Existing unavailable/dependency messages retained. |
| Audio / Subtitles / Filters / Trim | Locked; skipped. |

## Chapter semantics

Full title produces one file (including full main feature when Main feature only is selected). Every chapter produces one file per chapter using each title's own count. Chosen chapters are selected independently for each title. Tick boxes create individual files. Optional custom groups preserve one file per comma-separated item (`1-3, 5` makes two files). Validate every selected title before queueing; unavailable chapter numbers do not clamp. Chapter execution and whole-title passthrough constraints remain unchanged.

## Preview workload

Automatic optical previews must not decrypt/copy an entire disc or title. Read one bounded sample of exact mapped title cells, recover CSS key from bounded title/menu bytes, decode a static still, publish it immediately. Process sequentially with main feature first; stop on cancellation. Never substitute another title's VOB content. ISO resolution still extracts through the existing UDF path; this remaining cost is not described as bounded.

## Implementation and verification

Shared controls: internal/ui/help.go and components.go. Module integration: existing views and adapters. Chapter state/preflight: Rip chapter_controls.go, chapter_jobs.go and view.go. Preview sampling: content_thumbnails.go and internal/dvd/css/sample.go.

Checklist: native build/vet/full tests; actual hover/exit/preference/click/typing tests; per-title chapter counts and output ranges; bounded samples across DVD VOB file boundaries; encrypted sample decoding; narrow/large-font visual review. Encrypted DVD -> Rip -> Convert -> playback acceptance remains separate and outstanding.

Optical verification exposed a preview address error: DVD cells use sectors across the concatenated VTS title stream; logical VOBID does not select a physical filename. Preview sampling now crosses physical segments by global sector address. The existing executor cell fallback still maps by logical ID and requires a separate correctness repair/acceptance before treating chapter fallback content as proven.

## dev101 evidence (2026-10-13)

A bounded read from the user-authorized inserted encrypted optical disc produced an 80px preview in 3.322 seconds. The PNG was inspected and contains recognizable video content. This verifies the extractor for one title; full browser publication across every title and Rip -> Convert -> playback remain unaccepted. The prior opening-only fallback decoded a black frame and was rejected; opening data now recovers a key used to decrypt the original representative sample.

Maximum title-preview reads are 12 MiB for the first uncached VTS key (4 MiB representative + optional 4 MiB menu + optional 4 MiB opening); later titles in that VTS read 4 MiB. Per-scan key reuse does not cross sources or VTS sets. ISO preparation still uses full extraction.

A neutral 22-chapter picker was rendered at 400px width with 24px text: count, four-column checks, selected outputs and scroll remain readable. Shared help interaction tests exercise actual delayed hover, exit, navigation, disabled preference, keyboard/click and Entry typing. No real media images or identities are committed.

Final local gate: scripts/windows/dev-verify.ps1 passed native_media build, vet and the full test suite. Non-native build also passed. Linux packages and full live-browser/font coverage still depend on CI/user acceptance.
