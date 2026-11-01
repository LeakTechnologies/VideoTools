# Eternal and VideoTools: complementary media workflows

Owner decision: **2026-11-01**, explicit chat direction recorded on [#33](https://github.com/LeakTechnologies/VideoTools/issues/33#issuecomment-6553307119). VideoTools retains Go/Fyne and adopts useful concepts from the Rust applications. Eternal may expand into video and other media; it is not confined to music. This document defines the direction, not a shipped integration or a newly qualified backend.

## Product relationship

| Responsibility | Eternal | VideoTools |
|---|---|---|
| Organize a lasting media collection | Primary library, discovery, metadata presentation and listening/watching workflows | Source selection and processing projects; no duplicate full library required |
| Play music, video and future media | Primary consumption experience as support is developed | Deliberate VT_Player playback to inspect and verify processing inputs/results |
| Inspect media | Everyday properties and honest playback-path information | In-depth analysis, streams, timebases, chapters/subtitles, interlace and processing decisions |
| Transform media | Deliberate handoff when a source needs preparation | Conversion, rip/author, extraction, repair, filters and upscale jobs |
| Compare originals and processed results | Convenient review of library items | Controlled original/result comparison, synchronized transport and short test ranges |

These are product emphasis, not prohibitions on overlap or claims that Eternal already supports video. Each app remains useful on its own. Installing or running one must not become a requirement for the other. Preserve VideoTools' Windows static-binary and Linux deployment requirements.

## Concepts to adopt in Go

Port demonstrated invariants and behavior, not Rust syntax, a foreign application's UI or its full library database. Candidate source remains subject to qualification in #33.

1. **Typed media facts.** Separate source/container/stream facts from UI state, playback state and job settings. Retain unknown values, their origin and uncertainty. Audio/video/subtitle streams, chapters, timebases, color/rotation, duration and container metadata need explicit representation. Keep units and stream indices unambiguous. Avoid FFmpeg/device pointers in shared domain values.
2. **Stable source identity.** A selected processing source, a library item and a playback session are different identities. Track revision/replacement so late probe/load/analysis results cannot attach to a newer source. Check file changes before processing. Do not make full-file hashing a mandatory import cost; qualify an appropriate bounded identity strategy.
3. **Probe first; playback optional.** Read bounded facts for import and batching. Start preview/decoders/devices only when requested by policy. The approved #37 setting suppresses automatic previews while preserving manual VT_Player and independent Inspect. Explicit deep analysis may decode media and must show progress/cancellation.
4. **One owner per active resource.** Transport commands/events flow to an owner of source/decoder/output lifecycle. Cancel and join retirement before replacement. Bound queues and make stale publication impossible. Adapt this to Go's existing InlineVideoPlayer API; do not transplant an unqualified audio-only loop over video/DVD behavior.
5. **Independent job execution.** Processing owns immutable source/options/output plans; a preview is an optional consumer. Closing/disabling playback cannot remove job input or stop an unrelated conversion. Report preparation, running, verification, failure and cancellation honestly.
6. **Honest playback quality.** Report actual device/output behavior rather than requested format. Qualify sample fidelity and presented timing separately from format negotiation. Speed changes, normalization and resampling intentionally transform audio; do not call that path bit-perfect. Preserve unknown/unmeasured device latency.
7. **Incremental work.** Cache facts by validated source identity, bound background scans/thumbnails, prioritize visible/explicit requests, and retire obsolete work. No mandatory library scan or whole-source preparation on every processing import.

## Direct Eternal component reuse

Use Go implementations for concepts where that is simpler. A shared Rust library remains a possible bounded component, not an application rewrite or an automatic backend selection. Choose one after comparing duplicated maintenance against FFI, dependency, linking and packaging costs.

Before code reuse, pin exact committed or content-addressed source bytes, dirty state, dependencies, licences/notices and Windows/Linux results. Preserve VT's video/DVD/chapter/subtitle facts and rollback boundary. Define versioned request/result/error contracts, ownership/allocation/free, input encoding, cancellation and limits. No sibling workspace path may become a shipped dependency. VT's existing string acknowledgement bridge is not evidence of a working shared media engine.

## Deliberate interoperability

Begin with files and explicit user actions; do not start with a shared mutable database, an always-running service or speculative IPC framework.

- Eternal can hand a selected source to VT for inspection, conversion or repair. VT can hand verified outputs back for deliberate playback or library import. Dedicated handoff controls are future implementation, not current functionality.
- A handoff must identify the source revision and selected streams/range, requested action, destination and expected output behavior. Validate at the receiving boundary; reject changed/unavailable sources. Never treat paths or an imported manifest as executable command text.
- Keep original media intact unless the user explicitly requests a supported in-place change. Do not write Eternal's database directly. Report output identity, verification, warnings and cancellation separately from library-import success.
- The first proof should be simple: select one Eternal item, inspect/convert it in VT without automatic playback, verify the result, then open/import the output in Eternal. Preserve source/output distinction and handle an unavailable app gracefully.
- Future video handoffs must preserve chapters/subtitle/audio selection and defined timestamp/range semantics. Shared vocabulary can grow as proven workflows require it. Do not impose music-specific assumptions on video or disc sources.

## Delivery units and acceptance

The ordered correctness queue in [#29](https://github.com/LeakTechnologies/VideoTools/issues/29) remains in force. This sequence describes dependencies for ecosystem work, not a silent reordering of those tasks.

| Unit | Observable acceptance | State |
|---|---|---|
| Optional previews (#37) | Batch/metadata/Inspect work without automatic player resources; manual VT_Player works; cancellation/rebuild correctness; measured on/off import costs | Product behavior approved; application implementation outstanding |
| Shared media facts/identity design (#33) | Map current VT/Eternal facts, unknowns, units and revision semantics; no loss of video/DVD facts; exact source qualification | Concept direction approved; concrete port/ABI plan outstanding |
| Bounded metadata/component experiment | Fixture parity, malformed/unsupported input, bounded I/O/memory/cancellation; Windows/Linux evidence and deployment comparison | Not implemented; qualify source and agree concrete integration first |
| Explicit single-item handoff | Verified source -> job -> output -> deliberate Eternal open/import, with failure/cancellation/replacement checks | Proposed; no handoff code or schema implemented |
| Video comparison/deeper output integration | Presentation-based synchronization, transport/device failure tests and media-specific content acceptance | Later scope, not qualified by audio tests |

No full Rust/Slint rewrite is planned. No application/backend/dependency/build changes were made for this document. Documentation checks establish only record consistency; they do not establish playback, bit-perfect output, performance or interoperability acceptance.
