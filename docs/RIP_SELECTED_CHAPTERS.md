# Rip selected chapter groups — dev98

The chapter control offers All chapters (default) and Selected chapters. Selected
reveals a compact page-style entry, for example `1-3, 5-6, 8`. Each comma-separated
chapter or range produces a separate output file, in disc order. Adjacent tokens
remain separate files: `1, 2` differs from `1-2`. Overlapping groups, reversed
ranges, empty tokens, zero and unavailable chapters are rejected before queueing.

One selection applies to each selected title. All target titles must have known
chapter data and support the complete selection; no title silently clamps it.
Main-feature mode validates the longest title only. Full-disc, archivist and
region conversion do not apply chapter selection. All chapters preserves the
existing whole-title workflow and filename.

Selected outputs add `_Chapters_01-03` or `_Chapter_08` before the extension.
Queue items carry their existing title identity, chapterStart/chapterEnd and
enrichment settings. The existing contiguous-range executor and semantic cell
restriction/remapping remain authoritative; no sparse concatenation or new
player backend is introduced. A selection covering 1..N retains the established
whole-title passthrough.

Implementation lives in the Rip module's parser, queue expansion and view. New
labels/help/errors use all four locale dictionaries; Inuktitut needs human
review. Validation covers grammar, bounds, ordering, output naming, per-title
preflight and independent queued configuration. Native build/vet/full tests
remain the gate. Real-media multi-output and encrypted-disc acceptance are
separate checks; the UI is not proof of content correctness.

Dev101 replaces the shared input as the primary UI: counts and numbered checks are per title, Every chapter creates separate files, and custom groups remain optional under a collapsed section. Full title is the default. See [contextual help](UI_CONTEXTUAL_HELP.md).
