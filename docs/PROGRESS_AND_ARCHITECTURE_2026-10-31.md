# VideoTools progress and architecture assessment — 2026-10-31

Application baseline: **VideoTools v0.1.1-dev101**, `57704aad`; documentation baseline inspected: `7d03f029`. This is a source/issue/history assessment, not a fresh application, hardware or performance test. Ordered work remains [#29](https://github.com/LeakTechnologies/VideoTools/issues/29). Unmerged candidates are not released improvements.

## Overall assessment

VT is a substantial implemented toolset, but not yet a dependable, fully accepted suite. Core conversion/queue functionality is much further along than the full product vision. A completion percentage would mix working features, untested paths, locked modules and future scope; there is no evidence-based percentage. Treat the core workflow as stabilization work and the full suite as still in development. Correctness, bounded import work and real output acceptance are more valuable next steps than adding features or changing languages.

## Module readiness

Qualitative judgements below separate source presence from runtime acceptance. Locked modules remain locked.

| Module | Current assessment / main remaining work | Refactor need |
|---|---|---|
| Convert | Substantial encode/remux/preset/batch implementation. Two-pass no-op #23, preview/executor divergence #24 and source/playback acceptance #11/#20 remain. | High: separate import, job planning and optional preview |
| Queue | Shared job/progress foundation exists. Cancellation, immutable inputs, output checks and historical tester criteria #2 still need current acceptance. | Medium: lifecycle boundaries |
| Rip | Extensive title/chapter/CSS/naming implementation. Global cell addressing #31, viewport #27, ISO work #34 and encrypted-disc E2E remain. | High correctness, focused repair first |
| Author | Large menu/DVD/mux/UDF implementation; author-to-rip round-trip remains unaccepted. Source volume is not compatibility proof. | High after round-trip evidence |
| Burn | OS-backed burn/verify paths exist; physical-device acceptance and multi-drive/IMAPI #8 remain. | Medium, device-driven |
| VT_Player | Historical linear playback improved. dev101 lacks transport candidate 6d3cbae1; that candidate has repeated-seek/race evidence, but latest visible/audible acceptance remains pending. DVD menus absent; PR #28 separate/unmerged. | High lifecycle/presentation correctness |
| Inspect | Metadata/streams/chapters/analysis hooks exist. Should work independently of playback, with explicit deeper analysis, progress and cancellation. | High value: probe/analysis/preview split |
| Compare | Side-by-side/direct-engine infrastructure exists; comparison-grade synchronized seeking/stepping and fullscreen are not established. | High transport coordination; preserve dual-stream exception |
| Upscale | Model/job/preview infrastructure exists. Short test ranges, synchronized before/after and end-to-end output acceptance still needed. | Medium/high preview vs job split |
| Thumbnail | Output modes/image inspection implemented; tester #2 and current bounds/cancellation/large-input acceptance remain. | Medium, explicit extraction jobs |
| Trim | Locked; code exists, but frame/VFR/keyframe vs reencode semantics and output correctness remain unqualified. | Assess only after unlock approval |
| Filters | Locked; preview/output parity and cancellation need acceptance before exposure. | Assess later; preserve shared API |
| Audio | Locked; extraction/track UI is not proof of device/music playback fidelity. | Bounded reuse design, not wholesale port |
| Subtitles | Locked; timing, encoding, bitmap/pass-through/burn-in need format-specific acceptance. | Assess later |
| Files | Supporting file/source workflow, not an accepted media-library product. | Low/medium: source identity and safe handoff |
| Settings | Configuration/UI foundation exists; new persistent preview policy needs legacy defaults and clear availability. | Low/medium: policy ownership |

## Approved preview behavior; implementation outstanding

Design and checks: [#37](https://github.com/LeakTechnologies/VideoTools/issues/37). Owner explicitly chose **keep manual VT_Player available**, and Inspect should be useful as an in-depth tool without playback. Label the setting **Automatically load media previews**, default on for existing configurations.

Off skips automatic embedded player preparation in processing modules. Metadata, source identity, queueing and conversion/rip/author jobs remain functional. VT_Player retains deliberate manual file/disc playback. Inspect keeps metadata and on-demand analysis; deeper frame/sample analysis may intentionally decode media and needs progress/cancellation. This is not a zero-media-I/O switch. Existing locked modules stay locked; Compare/Upscale availability must respect their approved dual-stream architecture.

Disabling must retire/cancel/join automatic decoding, scrub, thumbnails and presentation, reject late callbacks and release automatic resources. Hiding a pane is insufficient. Preserve processing sources/jobs and manual-player ownership; a manual player must not reopen automatically through a processing rebuild. Re-enabling prepares at most the intended visible preview, never the whole queue. Audit cold-start device/hardware initialization too.

Source evidence: native_media.go load helpers; main.go Inspect imports; internal/ui/player_session.go opens engines, initializes frame cache, grabs initial/resume frames and starts scrub/thumbnail work. No current benchmark or whole-movie-RAM claim is established. Measure cold/warm import latency, first usable UI, peak working set and background activity with previews on/off for a long input and a batch. Preserve deliberate conversion/analysis decoding. Issue #37 inventories paths, exclusions, integration dependencies and runtime checks before code.

## Refactoring and Rust

Current source counts: main.go **17,868** lines; author_module.go **4,938**; settings_module.go **2,236**; subtitles_module.go **1,508**. Historical extraction-plan counts are stale. Internal view packages exist, but root adapters still contain substantial orchestration and mutable app state. Moving files alone will not fix ownership.

Prioritize demonstrated boundaries: source identity/bounded metadata probe; optional preview ownership; immutable job plan/executor/output verification; explicit analysis jobs; presentation/device ownership. Extract while fixing concrete coupling. Keep #31/#32, PR #28 assessment and transport integration separate; do not silently reorder #29 or bundle root cleanup, UI redesign and engine migration.

Eternal inspected read-only: HEAD `1f7f7b117c7ad60f32199d24b8ac9fd32ecc30e4` plus dirty app/audio files and untracked crates/eternal-audio/src/player.rs. No source was imported, built, tested or edited. HEAD alone does not pin those player bytes. [#33](https://github.com/LeakTechnologies/VideoTools/issues/33) requires source/licence/dependency/platform qualification.

Useful patterns: typed inspection facts, stable source identity, incremental scanning, one transport/device owner, command/event separation, negotiated output-format reporting and seek flushing. Avoid importing the music-library schema or assuming audio code supplies video/DVD/chapter/subtitle behavior. Linux output and submitted-vs-presented position remain qualification gaps.

Matching negotiated rate/depth/channels is useful reporting, but does not by itself prove sample-identical output or iTunes equivalence across devices. This assessment does not independently certify bit-perfect playback. Qualify exact bytes with sample/output tests before transferring fidelity claims to VT. Preserve third-party copyright/licence notices in dedicated documentation when reuse lands.

**Recommendation: retain Go/Fyne and evaluate shared Rust components incrementally.** A versioned metadata-inspection boundary is a sensible first candidate. A Rust core via C ABI adds allocation/free, encoding, cancellation, linking and dual-toolchain obligations; VT's existing Rust bridge is a string acknowledgement, not a media engine. A full rewrite would also requalify DVD/CSS, subtitles/chapters, FFmpeg, UI, static packaging and both platforms. Being the one Go project is not a defect. Reconsider a larger migration only after a measured shared component and bounded workflow demonstrate benefit. No rewrite, backend or dependency change is authorized here.

## Human credit and verification

Owner confirms Stu Leak and Jake P as the only human contributors. Live GitHub contributor data, including anonymous entries, lists both and no retired-tool identity. No tool author/co-author identity was found in local all-ref history. Historical commit prose/audit branch provenance still names a tool; no history/tags were rewritten. README explicitly credits both humans and obsolete active-agent role assignments were removed under [#36](https://github.com/LeakTechnologies/VideoTools/issues/36).

Performed: checkout/remote/HEAD/dirty/worktree inspection, live issues/contributor checks, relevant VT/Eternal source reads and scoped documentation checks. Not run: application builds, playback, conversions/rips, benchmarks, bit-perfect or hardware tests. The preview setting is designed/tracked, not implemented or released in this documentation work.
