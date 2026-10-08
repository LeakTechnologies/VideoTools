# VideoTools Agent Tools Reference

Registry of all agent/subagent tools in the VideoTools toolchain, with purpose, model, usage case, permissions, and known issues. Maintained in `docs/AGENT_TOOLS.md`.

---

## 1. VT-AUDITOR — Forensic Auditor (Cloud)

**File:** `.opencode/agent/vt-auditor.md`
**Model:** `google/gemini-2.5-flash`
**Mode:** subagent

**Purpose:** Independently confirm or disprove CLAIMED bugs or regressions in VideoTools UI and player code before any fix is attempted. Establishes what is TRUE, not what the brief asserts.

**Primary use case:** Forensic audit of the seek/timeline/player state logic and the UI layout tree — e.g. confirming or disproving "seeking ignored / erratic timeline jumps / timestamp drift / seekGen/lastSeekGen mismanagement" and "text clipping / padding overflow / UI-thread stall risks".

**Method:**
1. Read actual source before repeating any claim; locate exact functions and line numbers and quote them.
2. Trace state tracking across its full lifecycle. An `if x != y` guard that mutates neither `x` nor `y` is a bug class; confirm whether the tracked variable is actually assigned at the comparison checkpoint.
3. Read-only searches (`rg`, `git log -p`, `git blame`) to establish history and coverage.
4. Consult architecture docs (`docs/NATIVE_PLAYER.md` — the "One Rule": all modules use `ui.InlineVideoPlayer`; `docs/PLAYER_DEBUG.md` — real playback evidence).
5. Distinguish what can be proved from what cannot; never assert a defect not located in code.

**Report format:** Title, date, scope audited; one section per claim labeled CONFIRMED / DISPROVEN / UNCERTAIN with `file:line` references and quoted lines; additional observations list (BLOCKER / SHOULD-FIX / NOTE); explicitly stated unverified claims; brief chat summary plus report path.

**Permissions:**
- `edit`: deny repo files; allowed to write report to `C:\Users\User\.config\opencode\audits\`
- `bash`: git plumbing + `rg` + `dev-verify.ps1`
- `external_directory`: allowed only for the `audits/` output dir
- `task`: deny

**Invocation:**
```
@vt-auditor <task brief>
```

**Known issue:** Model is Gemini 2.5 Flash free tier — hard quota exhaustion (rate limit on `generate_content_free_tier_requests`). When quota is exhausted, dispatches fail with a retry window of ~3 hours.

---

## 2. VT-REVIEWER — Independent Reviewer (Cloud)

**File:** `.opencode/agent/vt-reviewer.md`
**Model:** `google/gemini-2.5-pro`
**Mode:** subagent

**Purpose:** Read-only review of a diff against its stated scope, the six-doc sync rule, and the repo's pinned invariants. Reports findings before merge.

**Primary use case:** Pre-merge review of a commit or PR.

**Method:**
1. Scope: does every hunk belong to the stated task?
2. Six-doc sync: a landing must update all six docs in the same commit — `docs/roadmap.html` (single source of truth), `docs/ROADMAP.md`, `docs/CHANGELOG.md`, `AGENTS.md`, `DONE.md`, `TODO.md`.
3. Pinned invariants (regression constraints, NOT cleanup candidates):
   - `content_list.go:292` — bulk Select All clears `locked`/`anchored` and raises `tc.updating` around `SetChecked`; removing the guard deadlocks the UI thread.
   - `executor.go:341` — `-f dvdvideo -title N -i <path>`: title MUST precede `-i`.
   - `executor.go:763` — all-chapter range is a whole-title passthrough.
   - `ripmode.go` `ripTargetTitles` — selection is sole authority; no count-based bypass.
   - `classifyRipFailure` — never reintroduce a `!isSomething` deny-list gate on the input-swapping retry.
4. Attribution & privacy: no AI attribution, no personal names in docs, no real media/disc titles (describe structurally).
5. Static-binary rule: no shared FFmpeg build, no DLL folder, no MinGW runtime DLL references.
6. Platform scope: Linux + Windows only.
7. Conventions: no new root-level `.go` files; `i18n.T().KeyName` only; no speculative code.
8. Verification: tests where required; player changes need a playback log and `PLAYER_DEBUG.md` update.

**Permissions:** `edit` deny; `bash` git plumbing only.

**Invocation:**
```
@vt-reviewer <review task>
```

**Known issue:** `gemini-2.5-pro` is no longer available to new users on this account — the agent is unusable and any dispatch fails.

---

## 3. VT-VERIFIER — Verification Agent (Cloud)

**File:** `.opencode/agent/vt-verifier.md`
**Model:** `google/gemini-2.5-flash`
**Mode:** subagent

**Purpose:** Run the verification gate after an implementation step — `scripts\windows\dev-verify.ps1` (native_media build + vet + test) — and report exact pass/fail diagnostics. Never fixes anything.

**Primary use case:** After a code change lands, confirm BUILD / VET / TEST pass before merge, with verbatim diagnostics.

**Method:**
1. Confirm repo root (contains `dev-verify.ps1` + `AGENTS.md`).
2. Run `.\scripts\windows\dev-verify.ps1`; capture exit code and full output. Cold build ~9 min; incremental fast.
3. On failure: re-run once exactly as written (single self-correction observation); report, no fixes.
4. Report per stage (BUILD / VET / TEST): PASS or FAIL with package paths, file:line, failing tests, assertion messages verbatim.
5. Interpret `0xc0000135` as `STATUS_DLL_NOT_FOUND` — environmental (FFmpeg runtime DLLs missing from `C:\ffmpeg\bin`), NOT a code regression. Name it as such.
6. Note anything not runnable given permissions as NOT RUN.

**Permissions:** `edit` deny; `bash` git plumbing + `dev-verify.ps1`.

**Invocation:**
```
@vt-verifier Run the verification gate after this implementation step
```

**Known issue:** Same Gemini 2.5 Flash free-tier quota exhaustion as `vt-auditor`.

---

## 4. VT-VERIFIER-LOCAL — Offline Verification Agent (Ollama)

**File:** `.opencode/agent/vt-verifier-local.md`
**Model:** `ollama/qwen2.5-coder:7b`
**Mode:** subagent

**Purpose:** Same verification gate as `vt-verifier`, but runs entirely on the local Ollama model with no network calls — for when free cloud keys are rate-limited (429) or the machine is off-grid. Conserves VRAM.

**Primary use case:** Verification gate on an offline machine, or to avoid burning cloud quota.

**Method:** Identical to `vt-verifier` (steps 1-6 above), scoped to the model's small context window — quote only decisive log lines.

**Permissions:** `edit` deny; `bash` git plumbing + `dev-verify.ps1`; `external_directory` deny.

**Invocation:**
```
@vt-verifier-local <verification task>
```

**Known issue:** Model tag mismatch. The declared model `ollama/qwen2.5-coder:7b` is NOT installed; the installed tags are `qwen2.5-coder:latest` (7B, 4.7 GB) and `qwen2.5-coder:14b` (9.0 GB). Dispatch fails with `model 'qwen2.5-coder:7b' not found` until the agent file is updated to `qwen2.5-coder:latest` (or a 14b model is pulled). This agent is dead as configured.

---

## 5. VT-AUDITOR-LOCAL — MISSING (Planned)

**Status:** DOES NOT EXIST — no file in `.opencode/agent/`, no entry in `opencode.json`.

**What the user references:** `@vt-auditor-local` is used in the workflow as a *local Ollama version of the forensic auditor* — the agent that should be used when:
- The Gemini free-tier quota is exhausted (blocking `vt-auditor`).
- A read-only forensic audit is needed but cloud inference is unavailable.

**Why it matters:** `vt-auditor` is the correct tool for the job, but the Gemini free tier is quota-bound (~20 requests/day). When quota is exhausted, `@vt-auditor-local` is the designated fallback. It does not exist, so the fallback chain is broken and auditors must either (a) wait for the quota window to reset, or (b) fall back to inline execution.

**To implement:**
1. Copy `.opencode/agent/vt-auditor.md` → `.opencode/agent/vt-auditor-local.md`.
2. Change `model: google/gemini-2.5-flash` → a local model, e.g. `ollama/qwen2.5-coder:14b` or `ollama/qwen3.5:latest`.
3. Adjust `steps` and `temperature` for the smaller local model.
4. Ensure `qwen2.5-coder:14b` (9.0 GB) or `qwen3.5:latest` (6.6 GB) is pulled into Ollama (host has 12 GB VRAM ceiling).
5. Optionally whitelist the model in `opencode.json` under the Ollama provider if not already registered.

**Recommended local model for this role:** `ollama/qwen3.5:latest` (6.6 GB, 262k context). The 262k context matters because a player/UI audit requires holding many whole files (e.g. 17k-line `main.go`) in context rather than aggressive slicing. `qwen2.5-coder:14b` is also acceptable but has a much tighter context window.

**Note on the existing report:** The audit report at `C:\Users\User\.config\opencode\audits\AUDIT_REPORT_PLAYER_UI_LOCAL.md` exists (462 lines) and was produced despite this agent's absence — either through another path or through direct editing. The audit is complete and accurate (see §2 below).

---

## 2. Audit Report State

**Path:** `C:\Users\User\.config\opencode\audits\AUDIT_REPORT_PLAYER_UI_LOCAL.md`
**Lines:** 462 (as of this review)

**Structure:**
- Title, date, scope audited, build tag note (`native_media`)
- Verdict summary table (7 claims: 6 CONFIRMED, 1 DISPROVED, 2 UNVERIFIED)
- Root cause section (corrected to Trim-only scoping)
- CONFIRMED 1–9 (seeking ignored, unserialised writers, optimistic time readout, conditional `seekGen` bump, drain-after-bump, `seekFlushBefore` lost-update, `WaitForPTS` stall, clock ratchet, Trim no-release-path)
- Risk register
- Part 2 — Static layout scan (candidates only):
  - 2.1 Translation expansion measured (FR mean 1.30x / max 3.00x; IU 1.10x rune floor caveat)
  - 2.2 Fixed-width container candidates (D1–D5) with `osdText` disproven and `frameTimingText` as the correct pattern
  - 2.3 i18n bypass (97 non-empty hardcoded literals, CONFIRMED)
  - 2.4 Padding overflow (DISPROVED in scanned scope)
  - 2.5 Scan limitations
- Revision history (2nd, 3rd, 4th passes)
- Section D — Static Layout Structural Risk Candidates (User-authored): D1–D11, including the highest-risk site `settings/tabs.go:873-878` (language dropdown `popupW = 280` px with raw `canvas.Text` language names in native scripts)

**Verification status:** All line citations verified against source. The following hold:
- `seekGen` historical bug DISPROVED (current code assigns `lastSeekGen = gen` at the checkpoint, `playback.go:607`)
- Trim has no `inlinePlayer.Seek` call site anywhere (only `ScrubTo`, which uses the separate `SmoothScrubbing` decoder)
- Two independent lossy mailboxes: `inline_player.go:322-325` (seekCh) and `scrub.go:266-271` (seekQueue cap 1)
- 3 concurrent writers to `SetFrame`, 4 concurrent writers to `SetCurrentTime`
- `isScrubbing` gate: zero matches repo-wide
- i18n literals: 97 non-empty confirmed via `widget.New(Label|Button|Check)\("[^"]+"\)` sweep

**Status:** Complete. Rendering-dependent claims (clipping, padding overflow) remain UNVERIFIED pending a screenshot, per the report's own caveat.

---

## 3. Tool Selection Guide

| Need | Use | Model / constraint |
|---|---|---|
| Forensic audit of a claim | `vt-auditor` | Gemini 2.5 Flash; quota-bound (~3h wait on exhaust) |
| Forensic audit when quota is exhausted | `vt-auditor-local` | **MISSING** — implement per §5 |
| Pre-merge diff review | `vt-reviewer` | **UNUSABLE** — Gemini 2.5 Pro deprecated for new users |
| Build/vet/test gate (online) | `vt-verifier` | Gemini 2.5 Flash; quota-bound |
| Build/vet/test gate (offline) | `vt-verifier-local` | Ollama local; model tag mismatch (use `latest`) |
| Audit / verification without any agent | — | Run inline (read/grep/bash); write only outside the repo |

**Fallback rule:** when an agent dispatch fails, fall back to inline execution. Inline results are treated as provisional until a rendering session confirms rendering-dependent claims.

**Constraint:** no agent edits or revises another agent's work. All agent artifacts live outside the repo (`.config/opencode/audits/`, `docs/`).

---

## 4. Revision History

- **2026-10-08** — Initial registry; all four agent files reviewed; `vt-auditor-local` documented as missing.
- **2026-10-08** — Audit report developed (Sections A–D); report state documented here.
