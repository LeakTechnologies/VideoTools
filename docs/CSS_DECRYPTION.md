# CSS Decryption: rip encrypted DVDs in software (dev85)

**Status:** done (dev85)
**Type:** Rip module capability — decryption pipeline
**Core ask:** *"rip commercial (CSS-encrypted) DVDs"* without requiring an external
libdvdcss build of FFmpeg, a specific drive, or any player-triggered unit-key path.

## Overview

A CSS-encrypted VIDEO_TS tree is now decrypted in software, before any ffmpeg process
touches it. The rip executor:

1. detects CSS when the source is scanned (`css.IsCSSEncrypted` — shipped dev45);
2. recovers the **disc key** and then the **per-VTS title/match keys** and the **VMG key**
   (software-only, the libdvdcss `DVDCSS_METHOD_TITLE` route);
3. copies the VIDEO_TS tree into a scratch directory and decrypts every VOB payload in
   place, writing IFO/BUP through byte-for-byte;
4. swaps the scratch tree into the pipeline **before** `CollectVOBSets`, so every
   downstream consumer (dvdvideo, VOB concat, cell-accurate lists, menu preservation,
   full-disc, archivist) reads plaintext bytes;
5. cleans the scratch tree up at the end of the rip.

No libdvdcss is linked, no external tools are invoked, and no drive restrictions apply.
libdvdcss (GPL) informs the design; this is an original Go implementation of the CSS1/A
cipher family.

## Motivation / use cases

- Encrypted (CSS) commercial DVDs currently either fail the rip or produce garbage —
  FFmpeg built without libdvdcss reads scrambled payload bytes, and the old multimux
  read-side `DecryptVOB` was never wired into the executor.
- `DVDCSS_METHOD_TITLE` key recovery needs only the unencrypted key-data region on the
  disc, so it works on any drive and on plain ISO images.

## Design

### Key recovery (`internal/dvd/css/crack.go`)

- **Disc key**: scan sectors over the key region for the repeating **406-byte period** the
  title-key region seeds. The period cycle seeds the disc-key derivation.
- **Title keys**: on the disc key, search the first encrypted sectors for a key structure
  that inverts the LFSR cipher; a *step-1* drive treats per-title keys as null bytes in
  `4ms`, details the busy scan uses byte counts on plaintext-discernible values.
- **Keys**: disc key, title/match key per VTS, and VMG key (VIDEO_TS.VOB is its own
  VMG-domain group). `RecoverTitleKey` inverts the sector cipher and writes the backward
  block + key to the correct offsets, matching reference behaviour.
- CAUTION (from libdvdcss): title + disc keys expire when the title match is stale or the
  disc key changes seed; on a key-register readback failure the drive reports a plain block
  or a wrong key. The rip path stays quiet so libdvdcss's natural output never leaks.

### Tables (`internal/dvd/css/tables.go`)

`cssTab1Inv` is the inverse permutation of `cssTab1`. Verified byte-identical (0 diffs)
to libdvdcss `csstables.h`. `cssTab4` IS an involution, but `cssTab1` is **not** under XOR:
64704/65536 combos fail the round trip. Therefore the **encrypt** direction (building a
scrambled fixture / sector rebuild) is `S = cssTab1Inv[P XOR ks]`, not `cssTab1`.

### VOB decryption (`internal/dvd/css/vts.go`)

`DecryptVideoTS(srcDir, dstDir, logf)`:

- walks the VIDEO_TS tree;
- groups VOBs by VTS: `VTS_XX_1..N.VOB` share the `VTS_XX_0.VOB` title key; `VIDEO_TS.VOB`
  is its own VMG-domain group;
- IFO/BUP copies pass through verbatim (they are never scrambled);
- each encrypted VOB is decrypted sector-by-sector with `DecryptVOB`; if a title key cannot
  be cracked the copy fails loudly rather than emitting garbage;
- logs one line per decrypted VOB via `logf`.

### Executor wiring (`internal/app/modules/rip/executor.go`)

- `Execute` cleans the scratch dir through a reassignable defer closure.
- On `css.IsCSSEncrypted(videoTSPath)` → `decryptVideoTSPath` creates
  `vt-css-decrypt-*` under `utils.TempDir()` via `os.MkdirTemp`, decrypts into it, swaps
  `videoTSPath`, and chains scratch cleanup to the same defer.
- The swap happens **before** `CollectVOBSets`, so all rip paths read decrypted bytes.
- Removed the dead `isEncrypted` parameter from `executeFullDiscRip` /
  `convertVOBWithRegion` signatures and call sites.
- Honest log messages replace the old plausible-but-masking CSS line.

### Output verification (independent of CSS)

`verifyRipOutput(opts, outputPath, appendLog)` after the run settles, before menu export:

- output must exist and be ≥ 1024 bytes;
- `ffprobe -select_streams v:0 -show_entries stream=codec_name -of csv=p=0` must report a
  video codec;
- failures return actionable errors ("rip output is trivial (N bytes) — the rip cannot be
  trusted", "rip output contains no video stream") instead of "Rip completed successfully".
- not invoked for archivist (that path's verification is its own stream-file check).

## Files

| File | Change |
|---|---|
| `internal/dvd/css/cipher.go` | `unscrambleSector` — committed decrypt port (no `t2 & 0x7F` mask) |
| `internal/dvd/css/crack.go` | `CrackTitleKeyFiles`, `crackFile`, `attackPattern`, `recoverTitleKey` |
| `internal/dvd/css/crack_test.go` | `cssTab1Inv`, `scrambleSector`/`buildScrambledSector`, 3 crack tests |
| `internal/dvd/css/tables.go` | CSS tables (0 diffs vs reference) |
| `internal/dvd/css/vts.go` | `DecryptVideoTS` per-VTS grouping |
| `internal/dvd/css/vts_test.go` | encrypted/plaintext fixture tests |
| `internal/app/modules/rip/executor.go` | CSS swap block + `decryptVideoTSPath` + `verifyRipOutput` |
| `internal/app/modules/rip/verify_output_test.go` | output-verification tests |

## Testing checklist

- [ ] CSS fixture (`TestCrackTitleKeyRecoversKnownKey`) recovers the re-derived system key.
- [ ] `TestDecryptVideoTS` / `TestDecryptVideoTSOutputIsPlaintext`: IFO bytes identical,
      decrypted VOB differs only in the scrambled sectors, multi-sector round-trip.
- [ ] A CSS-encrypted VIDEO_TS rip engages the decryption: log shows the scratch dir
      (`vt-css-decrypt-*`); output ffprobes as a real video stream.
- [ ] An unencrypted disc: no scratch dir, no decryption passes; rip unchanged.
- [ ] Crack/decrypt failure → rip fails fast with the `css-crack*` error, never a false
      success.
- [ ] Output verification: a missing/trivial/no-video-stream output fails the rip; a valid
      output passes.
- [ ] Real-media ladder (priority 1): DVD → Rip → Convert → playback on encrypted media.