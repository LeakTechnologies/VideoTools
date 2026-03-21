# Benchmark Module — Design Specification

## Purpose

The benchmark exists to answer one question:

> **Which hardware encoding path should this machine use?**

It does not tell the user how fast to encode, how much to compress, or
what codec to pick. Those are user decisions. The benchmark drives one
setting in the Convert module: **Hardware Acceleration** (`nvenc`, `amf`,
`qsv`, or `none`).

---

## What the benchmark tests

### One test per encoder, at a neutral quality preset

Each encoder is run once, at a sensible mid-quality preset. The goal is
**verification** (does it actually work end-to-end?) and **throughput
measurement** (roughly how fast is it?), not preset optimisation.

| Encoder | Preset used | Notes |
|---|---|---|
| `libx264` | `medium` | Baseline software reference |
| `libx265` | `medium` | Software HEVC reference |
| `h264_nvenc` | `p4` | NVIDIA balanced |
| `hevc_nvenc` | `p4` | NVIDIA balanced |
| `av1_nvenc` | `p4` | NVIDIA AV1, if driver supports |
| `h264_qsv` | `medium` | Intel QuickSync balanced |
| `hevc_qsv` | `medium` | Intel QuickSync HEVC |
| `h264_amf` | `balanced` | AMD AMF balanced |
| `hevc_amf` | `balanced` | AMD AMF HEVC |
| `av1_amf` | `balanced` | AMD AV1, if driver supports |

Hardware encoders that are not compiled into FFmpeg or not detected as
available are skipped entirely.

### What we do NOT test

- Multiple speed presets for the same encoder. The user controls
  quality-vs-speed tradeoff in the Convert module. Testing
  `ultrafast / superfast / veryfast / faster / fast / medium` for
  `libx264` teaches us nothing that helps recommend a hardware path —
  it just adds noise and run time.

- Software codec selection. The benchmark does not recommend `libx265`
  over `libx264` or vice versa. Codec choice is a user decision.

---

## What the benchmark recommends

The output is a single hardware acceleration mode:

| Recommended mode | Meaning |
|---|---|
| `nvenc` | NVIDIA GPU encoding via NVENC |
| `amf` | AMD GPU encoding via AMF |
| `qsv` | Intel Quick Sync encoding |
| `none` | Software encoding (no working GPU encoder found) |

This maps directly to the **Hardware Acceleration** dropdown in Settings →
Preferences and in the Convert module.

The benchmark does NOT recommend:
- A specific encoder (h264 vs hevc vs av1) — that is the codec choice.
- A specific preset/speed — that is the quality tradeoff.
- A specific bitrate or CRF — that is the quality setting.

---

## Test video

- Synthetic 10-second 1080p test pattern (FFmpeg `testsrc`), generated
  at benchmark start.
- 10 seconds is enough to measure encoding throughput with low variance.
  30 seconds adds run time with no benefit.
- Encoded to `/dev/null` (FFmpeg `-f null -`) — no output file needed.

---

## Scoring and recommendation logic

1. For each hardware vendor (NVIDIA, AMD, Intel), check if any encoder
   for that vendor completed without error.
2. If multiple vendors work, prefer the one with higher average FPS
   (higher throughput).
3. If no hardware encoder works, recommend `none` (software).
4. The displayed "top encoders" list shows all tested encoders ranked by
   throughput, filtered to successful runs only.

---

## Compliance checklist

Items that need to be implemented or verified before the benchmark is
considered complete:

- [ ] **Single preset per encoder** — remove the preset variation loop;
      use the fixed presets in the table above.
- [ ] **Test video is 10 s, not 30 s** — reduces run time meaningfully.
- [ ] **Recommendation is hardware path only** — `nvenc`/`amf`/`qsv`/`none`.
      The current code already does this in `applyBenchmarkRecommendation`
      but the results UI should make it clear we are recommending the
      *path*, not the *preset*.
- [ ] **Results UI labels** — the results card should say
      "Recommended hardware acceleration: NVENC" not
      "Recommended encoder: hevc_nvenc (preset: fast)".
- [ ] **Progress label** — during the run, show "Testing NVENC (H.264)"
      instead of "Testing: h264_nvenc (preset: fast)".
- [ ] **av1_nvenc / av1_amf** — add to detection and test matrix where
      supported; currently missing.
- [ ] **hevc_amf** — currently detected but not in the test matrix;
      add it.
- [ ] **Per-test timeout** — already implemented (2 min). Keep.
- [ ] **Results history** — already implemented. Keep.
- [ ] **"Run New Benchmark" in action bar** — already implemented. Keep.

---

## Non-goals

- Recommending the *fastest* software preset. That is a user choice.
- Measuring PSNR or visual quality. Out of scope for a production tool.
- Comparing codecs for quality. Out of scope.
- Auto-applying the recommendation without user confirmation. Always
  require the "Apply to Settings" button press.
