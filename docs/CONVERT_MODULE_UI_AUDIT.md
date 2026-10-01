# Convert Module — UI/UX Audit Brief

**Companion to:** `docs/CONVERT_MODULE_AUDIT.md` (the logic/architecture audit — already completed)
**Purpose:** a self-contained brief for an external auditor (GPT) to audit the **Convert module's
interface** — how it looks, how it behaves, how honest it is about its own state, and how hard it
is to use.

**Repo:** `github.com/LeakTechnologies/VideoTools` (Go + Fyne v2, CGo media engine)
**Cycle:** `v0.1.1-dev81`
**Platforms:** Windows (primary user platform) and Linux (primary dev). **No macOS.**

---

## 1. What went wrong with the first audit — read this first

A previous audit of this module produced 25 findings. Almost all were logic defects: preset mappings,
CRF values, config migration, output-path collisions. **It barely touched the interface**, because
the brief asked it to find defects in code and a Fyne widget tree is not a picture.

The user-facing complaint was never "the wrong codec is chosen." It was:

1. **The controls do not work, or lie about what they are doing.**
2. **The panels are cluttered and hard to use.**
3. **Parts of the UI look unfinished.**
4. **The layout is visually wrong.**

**You cannot settle any of those four from Go source alone.** "This looks broken" is a judgment
about a rendered frame. Your job is to read the layout code with the *specific* knowledge of how
Fyne resolves sizing, and reason about what the user actually sees — and to be explicit and honest
about the difference between what the code proves and what only a screenshot can settle.

**You are auditing the UI. Do not modify production code.** Output is the report in §7.

---

## 2. The layout engine, in one page (read this or your layout reasoning will be wrong)

Fyne does **not** work the way web CSS does. Getting this wrong produces a whole class of false
findings, so it is stated explicitly.

| Concept | Real behaviour |
|---|---|
| `container.NewVBox(...)` | Stacks children vertically. Each child gets **its minimum height only**; all remaining vertical space is left empty. This is the single most common cause of "large dead gap below the content." |
| `container.NewBorder(top, bottom, left, right, objects...)` | Top/bottom/left/right objects are placed at their **minimum** size; the centre object receives **all remaining space**. This is how you make something fill. |
| `container.NewMax(a, b, ...)` | Overlays, all sized to the **largest** child. Used to put a background rectangle behind content. |
| `container.NewCenter(a)` | Centres at the child's minimum size — the child does **not** expand. |
| `SetMinSize(...)` | Raises a widget's floor. It does **not** request that much space; it only prevents the layout from going below it. A widget with no `MinSize` can collapse to nearly nothing. |
| `widget.NewScroll(container.NewVBox(...))` | Scrolls. A VBox inside a scroll reports a small min height, so the scroll fills the space it is given — this is correct and expected. |
| `fyne.NewSize(w, h)` | Logical pixels (≈1.0 == 1 dip). `0` for a dimension means "unset/no floor," not "zero pixels." |
| `container.Split` (`NewVSplit`/`NewHSplit`) | Draggable two-pane divider. `SetOffset(t)` takes a **0.0–1.0 fraction**, not pixels. `0.97` means the second pane gets 3% of the axis. |
| Theme metrics | Padding, text size, and control heights come from the Fyne theme. Hardcoded pixel heights (34, 36, 40) only line up with themed controls if someone measured them — assume they drift. |

**Consequence for this module:** several panels are assembled as a bare `VBox` inside a sized
container, which is exactly the pattern that produces a tall empty region under the content. When
you flag "dead space below panel X", you must state which container is responsible and why.

---

## 3. File map — the interface surface

| Location | Lines | Role |
|---|---|---|
| **`main.go:8806–13151`** | **~4,346** | **`buildConvertView` — the entire Convert interface, one function** |
| `main.go:12815` | 1 | `leftColumn = container.NewVSplit(videoPanelWithHeader, metaPanelScroll)` — left column: player over metadata |
| `main.go:12820–12822` | 3 | `mainSplit = container.NewHSplit(leftColumn, optionsPanel)` — left column vs settings |
| `main.go:12831` | 1 | `mainContent := container.NewPadded(mainSplit)` — 10px outer padding |
| `main.go:9256–9267` | 12 | `resolveLeftOffset` — the split-offset policy (see §4.1) |
| `main.go:9269` | fn | `playerHeader` — collapsible header over the player |
| `main.go:9247` | 1 | `videoPanel := buildVideoPane(state, fyne.NewSize(480, 270), src, updateCover)` |
| `main.go:9288` | 1 | `metaPanel, metaCoverUpdate := buildMetadataPanel(state, src, fyne.NewSize(0, 200), …)` |
| `main.go:9112–9141` | 30 | `buildDrawer` — pop-up drawer, 420px wide, full height, min 220px |
| `main.go:9144–9180` | 37 | `buildBottomDrawer` — pop-up, 220px tall, full width, positioned from hardcoded bar heights |
| `main.go:12483` | 1 | `tabs = container.NewAppTabs(...)` — the Simple/Advanced settings tabs |
| `main.go:12549–12569` | 21 | `optionsPanel` — settings panel: background rect + padded tab stack + collapsible header |
| **`main.go:13170–13613`** | **~444** | **`buildMetadataPanel` — METADATA PANEL** |
| **`main.go:13614–14042`** | **~429** | **`buildVideoPane` — PLAYER PANEL (dispatcher)** |
| `main.go:13649`, `13698` | 2 | `// outer.SetMinSize(...)` and `// img.SetMinSize(...)` — **both commented out** |
| `main.go:14043–14103` | 61 | `previewAnimator` — the non-native path's frame animator |
| **`convert_player_native.go:29–616`** | **~588** | **`buildVideoPaneNative` — the real player panel and its custom controls** |
| `convert_player_native.go:481–498` | 18 | The control bar: seek row, transport buttons, speed, volume, fullscreen |
| `convert_player_native.go:572–574` | 3 | `frameTools` — separator, frame label, subtitle select, audio select, cover/save/import buttons |
| `main.go:13614–13617` | 4 | `if HasNativeMediaPlayer() { return buildVideoPaneNative(…) }` |
| `native_media.go:125` / `native_media_stub.go:10` | — | `HasNativeMediaPlayer` — the flag that selects the panel |
| `internal/ui/collapsible.go` | 80 | `BuildCollapsibleHeader` — the fold used by player/metadata/settings |
| `internal/ui/inline_player.go` | 975 | The player widget + its control overlay |
| `internal/ui/components.go` | 2,089 | Shared components: pills, value rows, labelled panels, drawers |
| `internal/theme/palette.go` | 46 | Colour palette |
| `internal/theme/vt_theme.go` | 72 | Custom theme (padding/text metrics) |
| `internal/theme/pillbutton.go` | 108 | `PillButton` |
| `internal/theme/vtslider.go` | 192 | `VTSlider` — the custom slider used for seek + volume |

---

## 4. Findings already confirmed by the client — do not re-derive

Established by direct inspection. **Your value-add is severity, root cause, blast radius, and the
UI judgement nobody has made yet.**

### 4.1 The split-offset policy collapses one panel to 3% (`main.go:9256–9267`)

```go
switch {
case state.convert.PlayerOpen && state.convert.MetadataOpen:
    leftColumn.SetOffset(0.5)   // Both open: 50/50
case state.convert.PlayerOpen:
    leftColumn.SetOffset(0.97)  // Player only: metadata collapsed
case state.convert.MetadataOpen:
    leftColumn.SetOffset(0.03)  // Metadata only: player collapsed
default:
    leftColumn.SetOffset(0.5)
}
```

Toggling either fold header re-runs this, so the intent is that collapsing one panel gives the
other the whole column. **Judge what 0.03 actually looks like**: a `MetaPanelScroll` that is still
in the tree, still in a scroll container, squeezed to 3% of the column height. A scroll container
that cannot show its own content is a real usability failure, not a cosmetic one. State the
concrete visual result and the correct value or approach.

### 4.2 The player pane's minimum size is deliberately disabled (`main.go:13649`, `13698`)

```go
// Don't set rigid MinSize - let the outer container be flexible
// outer.SetMinSize(fyne.NewSize(targetWidth, targetHeight))
…
// Don't set MinSize on image - it will scale to container
// img.SetMinSize(fyne.NewSize(targetWidth, targetHeight))
```

Both commented out, in the non-native pane. The native pane builds its own geometry. **Establish
whether the native pane has the same protection or the same problem**, and whether removing a
minimum size here trades letterboxing for the player collapsing when the column is narrow.

### 4.3 `speedSelect.SetSelected("1×")` contradicts a live engine speed (`convert_player_native.go:338–344`)

```go
player.SetSpeed(speedSteps[i])
…
speedSelect.SetSelected("1×")
```

The control reports 1× while the engine plays at the user's chosen rate after any rebuild. **A
displayed value that contradicts reality is the most dangerous class of UI bug** — the user trusts
it. Say how severe this is and what the correct source of truth is.

### 4.4 Track selectors reset to their first entry on rebuild (`convert_player_native.go:524`, `556`)

```go
audioTrackSelect.SetSelected(names[0])
…
names[0] = "Off"
…
subtitleTrackSelect.SetSelected(names[0])
```

Rebuild → selector shows the first option → engine keeps the user's actual selection. Same
contradiction class as §4.3.

### 4.5 `state.playerPaused = true` is assigned in three places, one of them during widget construction

- `convert_player_native.go:65` — during `buildVideoPaneNative` (i.e. during view construction)
- `convert_player_native.go:214` — a second construction-time assignment
- `convert_player_native.go:284–290`, `298`, `308`, `413–424` — the transport handlers

`buildConvertView` re-runs on every `showConvertView` (`main.go:3798` → `buildConvertView`). So
**every rebuild resets the app's idea of play-state while the singleton engine keeps playing.**
Decide: is a transport control that displays the wrong state worse than one that never resets?
What is the correct architecture (single source of truth on the player)?

### 4.6 Play/pause is decided from app state, not from the player (`convert_player_native.go:284–290`, `413–424`)

```go
if state.playerPaused {
    state.playNative()
    state.playerPaused = false
} else {
    state.pauseNative()
    state.playerPaused = true
}
```

There are **two** transport handlers doing this (two call sites above). The authoritative player
separately tracks `v.playing`. Anything that changes engine state outside these handlers — EOF,
source switch, frame step, another module using the shared player — desynchronises the two truths
and the **next button press then acts on the wrong assumption**. Trace every path that changes
engine state without going through these handlers.

### 4.7 Frame stepping pauses the engine but never updates the button (`convert_player_native.go:283–314`)

`StepFrame` sets `state.playerPaused = true` and calls `pauseNative()`, but the play/pause button
icon is not refreshed. The button still shows "pause" while the video is stopped.

### 4.8 Pop-up drawers are positioned from hardcoded bar heights (`main.go:9110–9175`)

```go
drawerWidth  := float32(420)
drawerInset  := float32(8)
…
statsBarHeight  := float32(40)
footerRowHeight := float32(32)
drawerTop := canvasSize.Height - drawerHeight - drawerInset - statsBarHeight - footerRowHeight
```

The bottom drawer's position is computed by **subtracting assumed pixel heights of other bars**.
If either bar's real height differs by a few pixels — theme change, font scale, longer label,
different platform — the drawer lands in the wrong place or off-screen. Read `statsBarHeight` and
`footerRowHeight` against what is actually constructed and report the drift. Also assess: the
drawer reads `state.window.Canvas().Size()` at build time, so what happens on window resize?

### 4.9 The player pane is dispatched to one of two implementations (`main.go:13615`)

`buildVideoPane` returns `buildVideoPaneNative` when `HasNativeMediaPlayer()`, else a ~360-line
preview-frame implementation. `native_media_stub.go:10` returns `false`, so the non-native pane is
what a `!native_media` build renders. **Do not recommend deleting it** (project rule: code whose
purpose you cannot establish is not dead code — there is no `.git` history in the archive). Report
the maintenance cost and what parity would cost, then state what history would settle it.

### 4.10 The Format section is half-implemented (`main.go:10268–10269`)

```go
// Format section UI (commented out - incomplete implementation)
// TODO: Implement format section with navy background and codec info display
```

Three helpers appear to be leftovers of that work: `buildFormatBadge` (`main.go:8730`),
`buildVideoCodecBadge` (`8763`), `buildAudioCodecBadge` (`8785`). A previous audit found **no
callers** for them. **Report what the user sees in that section today** and whether the section
reads as finished, unfinished, or incoherent.

### 4.11 The metadata panel is a dense, mostly untranslated label wall (`main.go:13327–13369`)

Roughly twenty rows built with `makeRow(...)`, using hardcoded English: `File`, `Format`,
`Resolution`, `Aspect Ratio`, `Duration`, `Frame Rate`, `Interlacing`, `Color Space`, `Color Range`,
`GOP Size`, `Video Codec`, `Video Bitrate`, `Pixel Format`, `Pixel AR`, `Audio Codec`,
`Audio Bitrate`, `Audio Rate`, `Channels`, `Chapters`, `Metadata`. The repo already has common keys
(`LabelResolution`, `LabelFrameRate`, `LabelDuration`, `LabelCodec`, `LabelBitrate`) that this panel
bypasses. **Beyond translation: assess whether twenty undifferentiated rows is the right
information design at all**, or whether the panel needs grouping, priority ordering, or progressive
disclosure. An audit that only says "add i18n keys" has missed the point of the complaint.

---

## 5. Audit focus areas

Answer each with evidence. Where the source cannot settle a question, **say so explicitly and state
exactly what a screenshot would settle** — do not guess.

### A. Layout and visual correctness
1. Walk the full tree from `main.go:12815` → `12820` → `12831` and describe the resulting geometry
   at **1600×900** and at **1280×720**. What does the user actually see?
2. Every `VBox` that is not inside a scroll and not inside a `Border` centre: does it leave a dead
   gap? List each by line and say what fills the space.
3. Hardcoded pixel dimensions (34, 36, 40, 92, 60, 72, 150, 640×360, 900×600, 420, 220) — for each,
   what breaks if the theme's text size or padding differs? Which are load-bearing?
4. `previewImg.SetMinSize(fyne.NewSize(640, 360))` (`13550`) and `previewDialog.Resize(900, 600)`
   (`13563`) — what happens on a smaller display?
5. `container.NewAppTabs` (`12483`) inside a `NewMax` + `NewPadded` (`12549–12553`): do the tabs
   have room for their labels at the 35% width the settings pane receives when open
   (`mainSplit.SetOffset(0.65)`)?
6. Density: count the interactive controls in the settings pane. Is the Simple/Advanced tab split
   carrying its weight, or is Advanced a wall?

### B. Honesty of state — the highest-priority area
1. Build a table of **every** value the UI displays vs. **every** place that value is authored.
   Flag every display that can disagree with the truth. Start from §4.3, §4.4, §4.5, §4.6, §4.7.
2. For each disagreement: what does the user *believe*, what is *true*, and what does it cost them?
3. Is the correct fix "make the widget follow the player" or "make the player follow the widget"?
   Argue it. This decision governs every other player fix.
4. What is the correct rebuild contract? A view rebuilt from state should be a pure function of
   that state. Enumerate every place `buildConvertView` reads state that the player owns, and every
   place it *writes* state as a side effect of construction.
5. Which of these defects are visible in a single screenshot, and which require interaction to
   reproduce? This determines what can be regression-tested by image.

### C. Clutter and information design
1. The metadata panel: twenty flat rows. Propose a concrete grouping or hierarchy, and say what
   moves to the fold, what becomes a summary line, and what disappears.
2. The settings pane: enumerate sections and controls. Identify what is duplicated, what is
   conditional-on-mode complexity the user cannot see, and what could be removed without loss.
3. `frameTools` (`convert_player_native.go:572–574`) packs a separator, a frame label, a subtitle
   select, an audio select, and three buttons into one row. Does that row fit at typical widths?
   What is its minimum viable content?
4. Are the drawers (`main.go:9112`, `9144`) discoverable? What affordance exists?
5. The interlacing analysis UI embeds an inline result card (`main.go:13418–13590`) *and* a
   comparison preview. Is that the right place for it, or does it crowd the metadata panel?

### D. Completeness and finish
1. The `// TODO` Format section (§4.10) — what is missing, and does the gap read as broken to a
   user?
2. Search the convert range for commented-out UI, unreachable branches, and `SetText("")` /
   `SetText("  ")` initialisations that only ever hold a placeholder. List them.
3. Are there any states with no feedback — an action with no spinner, no progress, no result? The
   convert view runs long operations (analysis, preview generation, encoding); find every one
   without visible feedback.
4. Empty and error states: empty source, failed probe, unsupported codec, missing ffmpeg. For each,
   what does the user see?

### E. Interaction and responsiveness
1. `buildConvertView` runs on every `showConvertView`. Enumerate the expensive work inside it —
   probing, preview generation, analysis wiring. Does any of it run on the UI goroutine?
2. What happens to in-flight async work (interlace analysis, preview generation) when the view is
   rebuilt or the source changes? A previous audit found the interlacing result is written to
   shared state without checking the source is still current (`main.go:9888`, `13443`) — a stale
   result can be displayed against the wrong video. Confirm and assess the UI impact.
3. Is there any debouncing on the many `OnChanged` handlers that write to `state.convert` and
   persist? Repeated `savePersistedConvertConfig` calls are visible as UI stutter.

---

## 6. Hard constraints — findings that violate these will be rejected

Settled project decisions. **Do not propose changing them.** If you disagree, use §7.9.

1. **The One Rule — the player API layer.** Every module reaches the player through
   `ui.InlineVideoPlayer`. No `media.NewEngine()` in a module, no per-module playback goroutines,
   no direct `media.NewVideoPlayer()`. **A Convert-specific or module-local player is a
   non-starter.** The backend is selected by the `vlc` build tag behind `media.PlaybackEngine`.
   *A Convert-side state adapter around the existing player is fine; a second player is not.*
2. **Sanctioned dual-stream exceptions, unrelated to Convert:** Compare
   (`compare/fullscreen_native.go`) and Upscale (`upscale_player_native.go`).
3. **i18n is mandatory.** All user-facing strings via `i18n.T().KeyName`; keys in
   `internal/i18n/strings.go`; English `en_ca.go` is the source of truth; also `fr_ca.go`, `iu.go`,
   `iu_latin.go`. Naming: `ModuleXxx` / `ActionXxx` / `LabelXxx` / `StatusXxx` / `DialogXxx`.
4. **No new root-level `.go` files — hard rule.** New code goes in the appropriate `internal/`
   package. `main.go` must shrink.
5. **No `darwin` paths.** Linux and Windows only.
6. **Windows: zero new runtime dependencies.**
7. **Player changes require a log review from a real playback session.** "No errors logged" is
   **not** evidence of correctness. `docs/PLAYER_DEBUG.md` is updated in the **same commit** as the
   fix or it does not get updated.
8. **State-tracking variables must be assigned at the point of comparison.** An `if x != y` guard
   that mutates neither is a bug — this class shipped before (a counter read but never assigned, so
   a log line fired 60×/sec forever). Audit for it.
9. **Chesterton's Fence.** Code whose purpose you cannot establish from the code, its comments, or
   history is **not** dead code. The archive has no `.git`. Say "purpose unclear — needs history."
10. **No personal names in docs** — refer to `user report` / `dev report`.

---

## 7. Required output format

### 7.1 Interface verdict
Max 20 lines. In plain language: what is wrong with this interface, ranked. The single change that
would most improve it. State plainly which of the four complaints — lying controls, clutter,
unfinished sections, wrong layout — is the biggest problem.

### 7.2 State-honesty table (the core deliverable)

| # | Displayed value | Authoritative source | Where it diverges | User believes | Actually true | Severity |
|---|---|---|---|---|---|---|

Every row must cite `file:line` for both the display and the author. This table is the most
important artifact you produce.

### 7.3 Layout findings
Per finding: location, the Fyne container responsible, the concrete visual result at 1600×900,
severity, and the specific fix. Include a short ASCII layout diagram of the current tree and the
proposed tree.

### 7.4 Clutter findings
Per finding: location, what the panel shows now, the proposed information architecture, and what
the user loses. Be concrete — "group codec/quality/bitrate under an Encoding heading, collapse
Advanced-only fields" beats "improve organization."

### 7.5 Completeness findings
Per finding: location, what is missing or unreachable, what the user sees today, severity.

### 7.6 Per-finding detail — for **every** finding
```
### U-N: <title>
- Severity: S1..S4   Confidence: Proven|Strong|Hypothesis   Area: layout|state|clutter|completeness|interaction
- Location: file.go:LINE (every relevant site)
- Failure scenario: <user action> → <user sees> → <why that is wrong>
- Evidence: the code you read (short excerpts, not paraphrase)
- Settles: <the specific complaint this explains: lying control / clutter / unfinished / layout>
- Proposed fix: smallest change that resolves the root cause
- Risk: what could regress; what must be retested visually
- Screenshot that would prove it: <what to capture, or "not reproducible in a still">
```

### 7.7 Severity scale

| Severity | Definition |
|---|---|
| **S1 Critical** | The UI misleads the user into destroying work, or a core action is unreachable. |
| **S2 High** | A control displays state that contradicts reality; a panel is unusable at the target resolution. |
| **S3 Medium** | Clutter, redundant information, or a dead region that measurably wastes space. |
| **S4 Low** | Cosmetic drift, stale comments, minor inconsistencies. |

Note the difference from the logic audit: here, **"the control lies about what the system is doing"
is S2 even when nothing crashes.**

### 7.8 Screenshot protocol
A numbered list of the screenshots you want captured, each with: what to do, what to capture, and
which finding it would confirm or refute. This is how the next pass closes the gaps you could not
settle from source.

### 7.9 Challenges (optional, separate)
Anything you believe violates a §6 constraint, stated as a challenge. Never as a fix.

---

## 8. Definition of a good audit

- Every claim anchored to a `file:line` you actually read. **If you did not read it, do not assert
  it.** Mark inference `Hypothesis`.
- **Separate "the code proves this" from "this is probably how it looks."** The user is
  specifically asking about appearance, and the honest answer to a visual question with no
  screenshot is *"I need to see it"* — not a confident guess. Guessing wrong here wastes a
  developer day.
- You reason about Fyne layout correctly (§2). An audit that says "wrap it in a VBox" where a
  `Border` centre is needed is worse than no audit.
- You address **all four** complaints. An audit that only finds untranslated strings has not done
  the job.
- The §7.2 state-honesty table is complete — every displayed value that has an authoritative
  source.
- Fixes are the smallest thing that works. No speculative rewrite of the 4,346-line function
  presented as one change.
- You respect §6. Challenges go in §7.9 only.

## 9. After the audit

Findings are triaged into issues, and the structural work is sliced into small reversible commits.
Verification is `scripts/windows/dev-verify.ps1` (Windows) or
`CGO_ENABLED=1 go build -tags=native_media ./...` + `go vet` (Linux). **A build is not verified
merely because `gofmt` passes.** Player-panel fixes additionally require a real playback session
with a log review, and `docs/PLAYER_DEBUG.md` updated in the same commit. Any landed change updates
`AGENTS.md`, `docs/CHANGELOG.md`, `docs/roadmap.html`, `docs/ROADMAP.md`, `DONE.md`, and `TODO.md`
in the same commit.
