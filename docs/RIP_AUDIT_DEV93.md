# Rip audit — dev93

Audit date: 2026-10-12. Scope: content browser, source loading, option controls,
scan/thumbnail lifecycle, queue construction and executor failure reporting.
Established Rip constraints in AGENTS.md remain closed facts. This is a code
audit; encrypted-disc acceptance and real playback have not been performed.

## Fixed in dev93

Blank title cards were introduced by dev91's `cardBox` container wrapper.
Fyne's canvas traversal (`_fyne/internal/driver/util.go`, `walkObjectTree`)
only descends into a concrete `*fyne.Container` or a `fyne.Widget`. The custom
wrapper satisfied CanvasObject through embedding but matched neither branch.
The List's hover surface remained visible while every child was skipped.
`cardBox` now extends BaseWidget and uses a simple renderer for its container.
The stable selection handler and owner-before-SetChecked contract are preserved.
`TestContentCardRendersAndRecyclesSelection` exercises actual canvas markup
traversal, a recycled card toggle and bulk deselection.

The runtime test additionally exposed a bulk-selection deadlock: toggleCard
locked cb.mu before reading the updating guard, while setAllSelected holds
that lock across SetChecked. The guard now returns before locking. Existing
source-reading tests checked guard presence but could not prove lock ordering.

## Open findings

Line references describe the audited baseline; use named functions after edits.

| Priority | Finding | Evidence and impact | Focused correction |
|---|---|---|---|
| P1 | Stale scan completion replaces newer source state | `view.go:1344–1407`: scan success, error and progress callbacks lack source identity checks. Loading another source or Clear cannot reject old results. | Invalidate pending scans on load/edit/Clear; reject every stale UI callback, including errors, snippets and preview loads. |
| P1 | Typed source retains old title metadata | `view.go:180–185`: source OnChanged updates path/output but leaves scan/selection. Queue construction at 443–451 and 563–586 combines the new path with old title/VTS metadata. | Invalidate source-derived state on edits; require a current successful scan before using scanned selections. |
| P1 | Ticker lifetime can cause nil dereference | `content_list.go`, Stop/startCycling: Stop sets cb.ticker nil while worker selects on cb.ticker.C. Interleaving is possible; runtime occurrence unverified. | Capture ticker locally with its completion channel; never read mutable ticker from worker. |
| P2 | Recycled cards display another title's thumbnails | `content_list.go`, updateCard/cycleThumbnails/extractThumbnails: old states retain thumbImg pointing at a recycled card and write without owner checks. | Detach old owner bindings and verify current ownership before image updates. |
| P2 | Old thumbnail work survives source replacement | Stop closes only ticker completion; extraction does not observe it or validate current source. | Capture cancellation/source identity when starting work; reject stale completion and marshal canvas changes to UI thread. |
| P2 | All audio tracks unchecked still exports all tracks | `executor.go:354`: unconditional `-map 0:a?`; AllAudioTracks only gates metadata at 855–863. | Make mapping reflect option; verify argument behavior for both settings. |
| P2 | Option controls describe first title rather than main feature | `view.go:1153–1168`, 1227: first title supplies chapter/subtitle controls; main-feature queue uses longest title at 443–451. | Derive enrichment controls from intended target; define behavior for multi-title selections explicitly. |
| P2 | Archivist project write failure reports success | `executor.go:1183`: project WriteFile error discarded. | Propagate write failure before progress 100/success. |
| P2 | Full-disc output inspection can panic | `executor.go:1562`, 1616 discard Stat errors; 1584/1620 dereference resulting FileInfo. | Handle errors and verify generated output before declaring success. |

## Risks requiring media or focused reproduction

- Archivist copies audio into `.ac3` regardless of original codec and DVD
  bitmap subtitles into `.sup` (`executor.go:1153`, 1158). Validate muxer and
  reconstruction compatibility with neutral structural fixtures.
- Full-disc reconstruction writes `VTS_01.VOB` and a single-PGC navigation
  model (`executor.go:1536`); validate DVD layout/navigation independently.
- H.264 MKV region conversion combines audio copy with atempo. Current UI
  forces full-disc mode for region conversion, so this is an executor/config
  risk rather than a confirmed normal UI path failure.

## Next sequence and acceptance

Ship the isolated blank-card correction, then address source consistency and
browser lifetime in focused fixes. Resolve reachable option/failure-reporting
defects without broad architectural work. Walk an encrypted DVD through title
selection, Rip, Convert and playback; review the runtime log. Preserve the
deferred player seek work and do not claim acceptance from unit tests alone.

Tester checks for dev93: labels/checks visible; scroll and toggle without
selection drift; Select All/Deselect All after scrolling; mode locks still
behave correctly; thumbnail extraction failures leave usable text and checks.
