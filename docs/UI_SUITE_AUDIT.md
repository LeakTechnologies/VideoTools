# VideoTools UI audit — task inventory and first visual pass

Dev95 implementation follow-up: Convert now has persistent Load Video/source identity, a metadata-first workspace and measured collapsed headers. The native Frame-only strip is replaced by labeled Player tools. This addresses C-01 and the focused Frame/split findings for the shipped native pane; the other suite findings remain open. Runtime/gate evidence is in PLAYER_DEBUG.md. The first-pass observations below retain their original baseline.

Date: 2026-10-12. Observed Windows build: v0.1.1-dev92_win. Repository baseline: dev93. Current appearance settings: large font size, VCR OSD Mono. Windows screenshots were inspected at approximately 803×661, 1175×731/825 and 1707×912 capture sizes; these are capture dimensions, not a claim about physical monitor resolution.

This report starts the requested module-by-module audit. The required-task inventory covers all modules. The first visual pass covers every currently reachable module except Upscale, whose detailed scope remains deferred by AGENTS.md. Loaded-disc scanning was exercised with the supplied ISO. The ISO was also opened in Inspect, exposing a disc/stream context mismatch. Loaded-video, encoding, running-job, error-recovery, keyboard and alternate-font coverage are incomplete. A completed full audit cannot be claimed from these initial screens.

No application implementation was changed. Real media names and paths are intentionally omitted. Supplied sources are called Disc sample A, Disc sample B and Video sample A. A generated neutral test clip is also available separately.

## Standing visual direction: SMPTE test pattern

The Human Director explicitly retains the vibrant SMPTE idle-player pattern as a major VT brand element. Its colors relate to the module colors. Keep it prominent wherever a media preview belongs; do not replace it with a generic empty card merely to reduce visual density.

Make the branded idle panel useful: context-specific Load video / Load disc / Load two videos to compare instructions, a clickable and keyboard-accessible opening action, and drag-and-drop where supported. Keep loading, unsupported-input and error states distinct from idle. If the player is collapsed, retain a visible source-opening action elsewhere. The pattern should welcome the next task without implying media is already loaded. Use consistent placement, type and behavior across modules while respecting different source requirements.

Assess redundant bars/statistics and misplaced controls separately from the branded pattern. Readability and task clarity should strengthen VT's identity, not erase it.

## Standing workflow direction: optional player and responsive layout

The Human Director wants playback optional: one user values an embedded player, another finds it unwelcome. Every working module must remain complete with its player closed. Keep the SMPTE brand central through a compact contextual source/loading affordance; expanding Preview reveals the full player and transport. This preserves identity while allowing the workspace to prioritize the current task.

Requirements:

- Remember preview-open preference per module. Do not re-open it merely because source, preset or settings changed. The dedicated Player module naturally centers playback.
- Preserve source opening, source identity, properties, output settings, readiness, queue/run and result access in both preview states. Playback track selectors and output stream decisions must have clearly distinct meaning.
- Closing preview must release its layout allocation rather than leaving a 3%-height scroll panel or an invisible minimum-size constraint. Define and verify whether decoding pauses/stops; hidden rendering must not consume active-player resources silently.
- Use a prominent SMPTE source affordance with a context-specific Load action and optional Preview action. Exact footprint is a design proposal to validate, not an implementation commitment.
- Start from useful source/task/output hierarchy, then adapt layout. Wide workspaces may use two columns; narrow workspaces should stack essential groups or use task-based sections. Advanced settings can scroll. Essential action labels and selected values cannot be clipped.
- Do not reduce or replace the user's selected font merely to make one screenshot fit. Test supported fonts and large text against actual minimum sizes and wrapping.

Planned verification matrix (not yet executed in full): Windows and Linux; player open/closed; narrow/current/wide windows; common physical display resolutions 1024×768, 1280×720, 1366×768, 1920×1080 and 2560×1440 where supported; display scaling 100%, 125%, 150% and 200%; record logical canvas dimensions separately. Test VCR OSD Mono, IBM Plex Mono and Plus Jakarta Sans, available text-size choices and long translated labels. Include multi-source names and populated data, not only empty screens. No macOS scope.

Accept a layout only when the source's next action, current settings and effective destination remain understandable at the supported combinations. Fixed-resolution screenshots alone do not prove cross-platform usability.

## Standing priority rule: allocate space according to the module's task

Human Director clarification: Convert metadata is more important than its preview. The large player currently displaces useful metadata. Reclaiming Frame-strip space must benefit source information and conversion controls first, rather than defaulting to an even larger player.

| Module | Primary workspace | Secondary / optional workspace |
|---|---|---|
| Convert | Active source/batch identity, readable source metadata, output decisions and effective result summary | Optional preview and playback tools; deliberate enlargement for checking a frame |
| Rip | Disc scan state, title/scene/chapter selection, tracks and intended output | Optional preview of the intended title |
| Queue/History | Job state, progress, errors, output access and next action | Detailed logs and thumbnail decoration |
| Inspect | Accurate properties, chapters, metadata and analysis | Optional playback |
| Player | Video, transport and current playback/source state | Expandable metadata and capture tools |
| Merge | Clip order, compatibility, stream/chapter decisions and output | Optional clip preview |
| Compare | Clearly identified source/result and meaningful comparison; choose properties or visual comparison according to the current action | Supporting playback controls and additional properties |
| Thumbnail | Generation settings and generated images | Optional source-video playback |
| Author | Ordered content, disc capacity/settings, menus/chapters and generation readiness | Source-video playback; menu preview becomes primary while testing navigation |
| Settings | Findable preferences, readability and persistence | No persistent player allocation |

Upscale remains deferred; locked/unavailable modules remain skipped.

Convert requirements: metadata remains readable with preview open and closed; narrow windows prioritize source/metadata and output settings before preview; expanding preview must not silently reduce metadata to an unusable strip. A focused larger-view action is allowed, with a clear way back. Remember preferences without treating the shared player's minimum size or a fixed 50/50 split as the module's importance policy.

The shared SMPTE brand is retained. A compact contextual loading panel can express it when playback is closed; its visual presence does not require a large decoding/player pane in every module. Determine panel hierarchy from import → understand source → choose output → validate → run, then test actual layouts. Do not make all modules conform to the Player module's hierarchy.

## Audit order

Shared source opening, navigation, status and queue behavior are checked alongside every module. Fix shared blockers first, then follow this order:

1. Rip — current user blocker and entry to encrypted DVD acceptance.
2. Convert — core output workflow and content handling.
3. Queue and History — know exactly what will run, what ran and where the output went.
4. Inspect — establish source/output facts without guessing.
5. Player — visually verify output and state; deferred seek implementation remains deferred.
6. Merge — preserve clip order and stream compatibility.
7. Compare — assess original versus result.
8. Thumbnail — generate predictable, inspectable images.
9. Author — make disc creation understandable and verifiable.
10. Settings and shared shell — check defaults, readability, language, paths and navigation throughout, then finish their dedicated audit.
11. Upscale — inventory only until DVD → Rip → Convert → playback acceptance succeeds.

Skip Audio, Subtitles, Filters and Trim: explicitly locked in mainmenu_module.go. Their controls embedded in unlocked modules still belong in those modules' audits. Burn is also visibly locked/unavailable in the running build, though it is not in lockedModules: its module handler is nil. The standalone Files module is not exposed as a tile in the observed menu; distinguish it from the shared Files dropdown. Do not unlock modules merely to audit them.

## Audit method and completion criteria

For each module: define the job the user came to do; walk empty → import → loading → ready → options → queue/run → progress → result → recovery; inspect small/current/maximized layouts; use long names and multiple sources; test keyboard focus and supported fonts/languages; verify what the visible settings actually send to the job. A visually attractive control that lies about the output fails the audit.

Every finding needs an evidence class:

- Observed: reproduced on a rendered screen or by interaction in this pass.
- Code-confirmed: implementation proves the behavior; runtime extent may remain pending.
- Design judgment: proposed improvement tied to a specific task.
- Pending: hypothesis or untested state; do not label it a defect yet.

A module closes only when required tasks are reachable, controls match actual state, essential content decisions are explained, results are discoverable, failures permit recovery, and the representative media ladder succeeds. Existing D1–D15 layout hypotheses and historical Convert briefs are leads, not current verdicts. Convert #20/#11 are implemented changes awaiting real-media verification, not automatically unfixed findings.

## Ranked findings from the first pass

| ID | Priority / evidence | Current problem | Proposed correction and acceptance |
|---|---|---|---|
| S-01 | P1, observed + code-confirmed | Clicking the main menu Files pill did not show a menu. internal/ui/mainmenu.go buildFilesDropdown only calls ShowAtPosition when Canvas.Focused is non-nil, then assumes *widget.Button. The actual PillButton is not focusable and is not widget.Button. | Anchor the popup to the actual Files control, show it unconditionally on activation, remove the unsafe type assumption. Mouse and keyboard must expose Open Files and recent sources at the control's location. |
| S-02 | P1, code-confirmed | Shared PillButton has mouse tapping/hover but no Fyne Focusable or keyboard activation implementation (internal/theme/pillbutton.go). Many essential actions use it. | Implement visible focus and standard keyboard activation for the shared control. Tab through import, destination, queue and run actions; Enter/Space must work consistently. Do not infer OS accessibility completeness from this change alone. |
| C-01 | P1, observed | Convert's empty screen exposes presets/configuration/output actions but no visible source-opening action. With the player folded, even the drag instruction is absent. The shared Files menu is only on the main menu and is currently broken. | Put a persistent source strip inside Convert: Open files, Add files, active source and batch count. Empty state should point to the first action. A user must import a clip without returning to the menu or discovering drag-and-drop. |
| R-01 | P1, observed in dev92; fixed in dev93 source | Supplied ISO scans to 9 titles, but cards have no text/checks/images. | Retest dev93 rendering fix with this disc, scrolling, mode selection and bulk operations. This is not a new unfixed dev93 rendering finding. |
| R-02 | P1, observed + code-confirmed | Typing the ISO path leaves No disc loaded. Leaving and re-entering Rip starts scanning. Source OnChanged changes path but does not scan or invalidate old scanned metadata. | Give path entry an explicit Open/Scan action or submission behavior, and make pending/ready source identity explicit. Old selection must never describe a new source. See docs/RIP_AUDIT_DEV93.md for the related stale-callback defect. |
| R-03 | P2, observed + design judgment | Main-feature/scenes/choose-titles mode lives far down the right settings column while the title list is on the left. At current size it requires scrolling; output destination is also below the first view. | Put content mode next to the content selection it governs. Keep destination and effective output summary easy to inspect before Rip Now. Group format, audio/subtitle/chapter options by the decision they represent. Scrolling may remain for advanced settings. |
| R-04 | P2, observed | While scanning, Disc says Reading disc information, Content Browser still says load a disc, and readiness says Load a disc to begin. Action buttons appear available. | Use one consistent loading/readiness state across the module. Loading cannot be presented as empty. Disable or explain unavailable actions; preserve the successful scan summary. |
| S-03 | P2, observed + code-confirmed | Error after an archive was opened through Inspect said only Failed to load video: exit status 1. It did not identify an unsupported input or explain recovery. Inspect's file chooser is unfiltered. | Filter expected source types while allowing an explicit all-files route, validate unsupported input early, and present a useful probe reason with a retry/browse action. Preserve detailed logs. |
| S-04 | P2, observed; cause pending | The error dialog's standard dismissal control showed no visible label in the captured frame. Tab revealed a small focused button; clicking it dismissed the modal. Enter/Escape attempts did not visibly dismiss it. | Reproduce with the supported large/VCR setting and alternate fonts. Restore readable dismissal text and reliable keyboard recovery. Do not attribute the visual fault to the player without evidence. |
| T-01 | P2, observed + code-confirmed | Thumbnail's horizontal output-mode radio overflows the narrow left column; the selected third label is off-screen. Some dropdown extents are also clipped at current size. outputModeRadio.Horizontal=true. | Stack/wrap the modes or enforce a measured responsive layout. All three labels and selected state must remain visible at supported font sizes without manually dragging the splitter. |
| A-01 | P2, observed + code-confirmed | Author Generate tab says Ready to generate with no clips; capacity simultaneously says add videos to see estimate. Global Generate DVD still looks actionable. Summary uses AuthorReadyToGenerate unconditionally. | Derive readiness from actual prerequisites; show what is missing before allowing generation. An empty project must read Add videos, not Ready. |
| A-02 | P2, observed + design judgment | Author exposes Add to Queue, Compile to DVD, Generate tab and persistent Generate DVD, with no visible explanation of the distinction or next required step. | Define one clear authoring sequence and one primary action per state. Keep expert menu/chapter/subtitle tabs, but make generation readiness and queue-versus-run semantics explicit. |
| C-02 | P2, observed + design judgment | Convert Simple starts with Show Batch Settings, Device Presets and User Presets. At current large-font width, even Output is below the first viewport. A preset description line is visibly cut off. | Lead with source, intent/preset, output and a concise effective settings summary. Treat batch scope as context, not the first task for a single empty source. Wrap explanatory text and use progressive disclosure. |
| I-01 | P1, observed + probe/code-confirmed | Inspect accepts an ISO as ordinary MPEG-PS and presents 90-second stream properties without title/disc context. Rip identifies nine titles and a main feature of about 82 minutes. | Route disc inputs through a disc/title-aware path or explicitly identify the inspected stream; do not imply the first stream describes the whole disc. |
| I-02 | P2, observed + probe/code-confirmed | Inspect labels 740,703 kbps format bitrate as Video Bitrate; video stream bitrate is 7,500 kbps. GetBitrate returns inspectFile.Bitrate populated from format metadata. | Model/label container and video-stream rates separately; use unknown when unavailable. Verify copied/exported values too. |
| I-03 | P2, observed | At current width and large/VCR font, the two-column Inspect property grid clips codec, aspect, pixel aspect and other values. | Adapt the facts layout to available width; wrap or stack values and retain copyable full data. Test with preview closed as well as open. |
| Q-01 | P2, observed + design judgment | Empty Queue still gives visual priority to Start/Pause/Resume/Clear and reserves a large Live Output area. It offers no obvious import/task route. | Empty Queue should explain how to add a job. Actions need availability tied to jobs; running-job information should emphasize source, stage, progress, ETA, result and actionable errors before raw logs. |
| S-05 | P2, observed + design judgment | Repeated colored bars, empty statistics strips and similarly weighted secondary actions consume vertical space across modules. Back navigation is labeled with the current module; icon-only clear and pipeline controls need interpretation. | Retain VT's identity/color system, reduce redundant chrome and distinguish primary actions. Label tooltips and focus names; present current module/source and a clear Back to modules route. Validate usefulness of persistent statistics when no job exists. |

The reported Not Responding frame occurred during the file-opening experiment. Log review subsequently established that an archive was probed. It is a responsiveness observation, not proof that large-directory listing caused a freeze. The multi-click folder-navigation attempt could have acted on a newly recycled row after navigation; isolate that behavior before filing a separate chooser corruption finding.

## Required tasks and proposed presentation, module by module

### Rip

Tasks: load drive/folder/ISO; see scan progress/errors; identify main feature/extras/scenes; inspect title duration/chapters/languages; select exactly intended titles or chapter span; choose lossless/encoded output; select audio/subtitles; name destination; queue/run/cancel; find output and open in Convert.

Best forward path: Source → Content → Output → Review/Run. Content mode belongs with title selection; selecting a title should expose its relevant details. Audio/subtitle/chapter controls must describe the target rather than simply title 1. A review summary must state title count/ranges, effective output format, tracks retained and destination. Keep advanced full-disc/region/archivist work separate enough to explain its changed output semantics.

Observed: empty, typed path, re-entry scanning and loaded 9-title disc; right-column scroll at current/maximized widths. Pending: dev93 visible cards, accurate selection, chapter ranges, alternate discs, encryption, progress/cancel/output handoff. Existing executor/thumbnail/source-state findings stay in RIP_AUDIT_DEV93.md rather than being duplicated as new UI discoveries.

### Convert

Tasks: open single/batch media; recognize active source and scope; read source properties; choose container/codec/preset; control quality/size/speed; preserve/select audio, subtitles, chapters and metadata; correct crop/scale/aspect/interlace/frame rate; preview output; set safe names/destinations; queue current/all or run; recover and verify.

Best forward path: a persistent source/batch strip; an output-intent/preset choice; an effective output summary; task-based video/audio/subtitle/chapter/metadata details when needed; preview and queue/run at a consistent location. Source metadata should be useful context, not an enormous empty panel. Preview controls describe playback; output controls describe the encode. A preview track change must not imply it changes the output track mapping.

Observed: empty and folded player, Simple/Advanced entry points, current/maximized width. Pending: supplied Video sample A loaded, batch navigation/scope, advanced control reachability, stream handling, preset overrides, output collisions, queue and real preview behavior. Existing playback-state fixes require verification, not blind reimplementation.

HandBrake provides the content-handling benchmark: source opening, presets/quality, preview, queue, geometry/frame rate, audio/subtitle handling, chapters and metadata. VT must meet explicit output-behavior checks for these tasks; copying their screen arrangement is not an acceptance criterion. References: https://handbrake.fr/docs/en/latest/table-of-contents.html and https://handbrake.fr/docs/en/latest/cli/command-line-reference.html.

### Queue and History

Tasks: review pending jobs and effective settings; reorder/start; pause/resume/cancel supported jobs; distinguish pending/running/success/failure/cancelled; inspect stage/ETA; retry safely; view log; find/open output; route output to Convert/Inspect/Player/Compare; retain useful history.

Presentation: compact job cards/table with clear status and next action. Show raw Live Output on demand. Separate clearing completed records from cancelling work. Empty state should link back to a task rather than a row of irrelevant controls.

Observed: empty Queue and menu History affordance. Pending: running/completed/failed jobs, retries, mixed module jobs, handoff and history detail views. No jobs were submitted in this audit pass.

### Inspect

Tasks: load source/output; read container/video/audio/subtitle properties; inspect chapters and sync; recognize read-only versus editable fields; copy/export facts; replace/clear source; route to another module.

Presentation: clear source opening, organized facts with units and source identity; preview optional; explain Sync's actual operation. Export controls become relevant when data exists.

Observed: clear load affordance and tab grouping; chooser/error path; loaded ISO metadata at wide and current widths. The ISO is presented as a 90-second MPEG-PS stream although Rip identifies a roughly 82-minute main feature. Its Video Bitrate pill uses a 740,703 kbps container estimate instead of the 7,500 kbps video stream. The current-width two-column facts also clip values such as codec, aspect and pixel aspect. Pending: Video sample A loaded and per-tab details. Confirmed error lacked a useful reason; an archive was opened during navigation, so do not infer that the approved video itself failed.

### Player

Tasks: open/replace source; identify current file; play/pause/seek; trust time/duration; volume/speed; choose audio/subtitles; fullscreen/frame capture; leave/re-enter cleanly.

Presentation: an obvious Open file action plus drop target; standard transport controls with truthful state; separate capture tools from transport. Do not make dropping from Explorer the only discoverable import route.

Observed: empty preview with Drag to load video and Clear Video, no visible Open file button. Pending: actual playback, seek behavior, track switching and lifecycle/log review. No player implementation change is proposed until required real-media checks; deferred seek work stays deferred.

### Merge

Tasks: add clips; identify/reorder/remove; understand compatibility; name chapter boundaries; choose copy or encode and normalization; retain/select streams; set output; queue/run and verify.

Presentation: ordered clip list with duration and compatibility feedback, clear copy-versus-encode summary, destination and readiness. Explain whether frame-rate/interpolation options apply to Fast Merge; do not call it a confirmed mapping defect without testing.

Observed: useful Add Files and minimum-two-clips hint, default Fast Merge, frame-rate/interpolation and track preservation controls. Pending: multiple sources, long similar names, ordering/accessibility, compatibility and results.

### Compare

Tasks: choose original/result; identify sides; compare properties and corresponding moments; synchronized playback where supported; fullscreen; replace/clear; understand failed loading.

Presentation: names/roles rather than only File 1/File 2; aligned properties with differences emphasized; explicit sync/time alignment policy and availability of fullscreen/export actions.

Observed: empty two-column screen with independent Load File controls. Pending: loaded pair, sync, long names, differences and exports. Their presence is not evidence that synchronized playback works.

### Thumbnail

Tasks: load/batch sources; choose individual/sheet/both; understand grid/count/time positions/size; choose destination; generate; inspect/open/save images; manage errors.

Presentation: all modes readable; predictable grid/count relationship; destination and expected output count before Generate; preview is useful after configuration rather than competing with required controls.

Observed: Both radio label clipped; sheet/individual counts present; placeholder preview and queue/generate actions. Pending: actual generation, timestamps, batch sources, destination, image sizing and alternate fonts. Hardcoded Generated Thumbnails/result placeholder also require an i18n check (internal/app/modules/thumbnail/view.go).

### Author

Tasks: add/order videos; chapters/subtitles; region/aspect/quality/capacity; menu structure; menu preview/navigation; project/destination/ISO; generate and validate DVD.

Presentation: Content → Disc settings → Menu/chapters/subtitles → Preview → Validate/generate. Expert tabs can remain; a visible prerequisite/readiness summary must make the next task clear. Explain compile/queue/generate distinctions.

Observed: Videos and Generate empty states, seven tabs, contradictory readiness and multiple generation actions. Pending: actual authoring, chapter/subtitle decisions, capacity calculation, menu navigation and generated-disc playback. Ecosystem acceptance remains a later product milestone.

### Settings and navigation

Tasks: choose readable font/size/language; find paths/tools/defaults; understand immediate versus saved changes; reset safely; check update status; return to the task without losing work.

Presentation: searchable/task-based groups if supported; clear persistence behavior; typography choices must be honored across dialogs as well as custom controls. Do not hide clipped controls by silently reducing a user's chosen size.

Observed: Preferences/Dependencies/Benchmark tabs, current large/VCR settings, long preference stack. Pending: alternate fonts/sizes/locales, focus behavior, persistence/reset, updates and source restoration. Settings entry initially completed asynchronously; do not mistake the first immediate capture for a dead button.

### Deferred/unavailable modules

Upscale inventory: choose source/model/scale/device, preview/compare, destination, queue/progress and verify. Detailed visual/task scope after encrypted DVD ladder, per AGENTS.md.

Burn inventory only: ISO, drive/media/capacity/speed, supported verification/eject, start/cancel/progress/result. Visibly locked in current build.

Standalone Files inventory only: folder navigation, types/sizes, routing content to modules and access-error recovery. Not exposed in the observed menu. Source code sizing risk in file_manager_module.go is pending visual proof.

Audio/Subtitles/Filters/Trim: explicitly skipped while locked. No unlocked status implied by their inclusion in the main menu.

## Reconciliation with the newly committed VT_Player audit

Read docs/VT_PLAYER_AUDIT_DEV93.md from commit ce3438ab (audited implementation abab4fca). It is a code audit and repair sequence; no current runtime acceptance is claimed. Its P01–P15 IDs are authoritative references for player implementation findings, rather than duplicate UI issue IDs in this report.

- Optional preview must have a defined resource lifetime. P04–P07 and P10 establish competing presentation consumers, stale-source publication, uncancellable waits and unsafe seek/resource lifetime. A hide/show toggle alone cannot guarantee that hidden playback is stopped, callbacks are retired or another module owns the active source. Closing preview must use the coordinated lifecycle boundary when that repair is authorized; exact pause-versus-stop/resume behavior needs task acceptance.
- Playback UI must report accepted media state. P01–P03 and P08 explain incorrect seek flags, preview scrubbing without committed playback repositioning, discarded final seek requests and incorrect backward stepping. A timeline displaying requested time is not proof that the shown frame/audio match it. Test preview and committed seek separately; do not attempt isolated visual patches to hide these defects.
- Source controls require Ready-driven updates. P09 and P12 establish inconsistent terminal state, inherited chapters/thumbnails, stale track selectors and Player Clear not clearing host state. The UI audit must check fresh source identity, selected tracks, chapters, Clear and error recovery after actual load completion and navigation.
- DVD context and module exits matter. P11/P14 cover lost title/open options, unverified direct CSS playback and incomplete DVD pane disposal. An Open in Player action must retain the intended title and must not imply that successful Rip decryption makes direct encrypted playback valid.
- P13/P15 add growing-file watcher and partial-open cleanup acceptance cases. Performance claims remain measurements to gather after correctness; do not remove the SMPTE branding or increase software decode threads based on unmeasured costs.

The player repair sequence stays lifecycle ownership → transport correctness → source/module state → measurement → real-media acceptance. The UI plan keeps the SMPTE brand and optional playback while respecting the standing encrypted-disc priority and deferred seek implementation. No backend, workflow or player implementation change is authorized by merely reading this audit.

## Focused space audit: Convert's Frame strip

The Human Director explicitly authorizes reconsidering established layout conventions when a more efficient presentation is justified. Preserve correctness, optional playback and SMPTE identity; the existing chrome, split proportions and component arrangement are not design requirements.

Observed: the always-present Frame strip occupies about 55–60 capture pixels beneath the video in the inspected layouts, even when no source exists. Its only readable content is Frame: 0. This is a separate bordered bar; it is not unavoidable aspect-ratio letterboxing.

Code confirms:

- convert_player_native.go builds frameTools from frameLabel plus subtitle/audio selectors and coverBtn/saveFrameBtn/importBtn; advancedBar is always the bottom edge of the player Border.
- All three frame/cover buttons call utils.MakeIconButton with an empty symbol. internal/utils/utils.go constructs widget.NewButton(symbol, ...) and ignores tooltip. These actions therefore have neither a visible label/icon nor their supplied description. This explains why the row appears to contain only Frame.
- The same native pane is supplied to the standalone Player module. Other modules using their own InlineVideoPlayer views need separate inspection; do not assert this exact bar exists everywhere. The fallback pane in main.go contains a similar advancedBar.
- Frame display is derived from requested/current time multiplied by nominal source frame rate. It is not a reliable exact frame index for VFR or failed/uncommitted seek states.
- Convert's resolveLeftOffset assigns 50/50 when player and metadata are both open, including empty metadata. It assigns 0.97 when metadata is collapsed. The live collapse test enlarges the SMPTE image but clips the metadata header to a thin strip at the bottom. A fraction is not a safe collapsed-header size.

Recommended presentation:

1. Remove the dedicated Frame box. Keep time/transport in one compact control area; provide optional frame/timecode detail there when accurate and relevant.
2. Move capture/save/cover actions into a labeled Tools menu or compact tools section. Playback audio/subtitle controls must be explicitly labeled and distinct from output track settings.
3. Idle preview shows the branded SMPTE source action without transport/frame-tool chrome. With media loaded, controls should fit the width and leave maximum useful room for the image; stable controls and touch/keyboard access take precedence over auto-hiding everything.
4. Give collapsed sections the measured height of their headers. In Convert, preserve metadata and output workspace first; preview uses a bounded optional allocation. Metadata should have a compact source summary with readable expandable details, and a remembered user-adjustable split when both panels are wanted. Avoid unconditional 50/50 allocation.
5. Preview closed: compact branded source panel and all task controls remain available. Preview open: a useful image with a compact control area that does not displace higher-priority metadata and conversion controls. Fullscreen remains a separate deliberate viewing action.
6. Reconsider the blank stats strip and disabled snippet/config/action rows independently; do not reduce the SMPTE pattern to compensate for redundant chrome elsewhere.

Acceptance: no Frame-only bordered row in idle or loaded states; no unnamed tools; preview is optional; closed panels remain reopenable at small/large text sizes; controls fit at narrow/current/wide widths; media aspect ratio is preserved; accurate state/PTS comes from the player lifecycle authority. A layout change alone does not repair the VT_Player transport findings.

Evidence from the initial pass: the external artifact convert-metadata-collapsed.png recorded the expanded preview and clipped collapsed metadata header; this initial screenshot is not versioned in the repository. No production code was changed by this investigation.

## Fix sequence and remaining audit coverage

1. Repair shared source-opening and button keyboard interaction. These unblock meaningful end-to-end UI checks across modules.
2. Verify dev93 Rip cards, then source-state/readiness and content-mode/output placement. Finish the encrypted-disc ladder rather than expanding player work.
3. Audit loaded Convert in depth with Video sample A plus neutral multiple-audio/subtitle/chapter fixtures. Check preservation and output promises against effective job configuration; then recommend its task-based layout.
4. Exercise Queue/History success/failure/cancel/result handoff; check Inspect and Player against produced output.
5. Finish loaded-state passes for Merge, Compare, Thumbnail and Author individually. Tackle each module's findings in a small versioned fix, not one suite-wide redesign.
6. Finish font/language/small-window checks and dedicated Settings/navigation review. Then scope Upscale when acceptance permits.

Each correction needs a before/after visual check at the same appearance setting plus a task outcome check. Unit tests can protect state/argument contracts; screenshots and actual use decide clipping, hierarchy and task clarity. The current report is a full task inventory and first visual pass, not a claim that all required tasks have passed.