package css

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrTitleKeyNotFound is returned when the known-plaintext attack cannot recover
// a title key from the given VOB files within their readable span.
var ErrTitleKeyNotFound = errors.New("could not recover CSS title key from scrambled sectors")

// known pack start prefix: every DVD-Video VOB sector begins with 0x00 0x00 0x01.
var packStart = [3]byte{0x00, 0x00, 0x01}

// CrackTitleKeyFiles recovers the CSS title key of a scrambled title set by
// scanning its VOB files sector by sector and running the known-plaintext
// attack (Frank Stevenson, 1999) against encrypted packs. This is the
// libdvdcss DVDCSS_METHOD_TITLE route: it needs no drive ioctls, only the
// scrambled bytes that a filesystem read of a CSS disc returns.
func CrackTitleKeyFiles(vobPaths []string) ([5]byte, error) {
	if len(vobPaths) == 0 {
		return [5]byte{}, fmt.Errorf("no VOB files to scan for title key: %w", ErrTitleKeyNotFound)
	}
	for _, path := range vobPaths {
		key, found, err := crackFile(path)
		if err != nil {
			return [5]byte{}, err
		}
		if found {
			return key, nil
		}
	}
	return [5]byte{}, ErrTitleKeyNotFound
}

// crackFile scans a single VOB file; found is false if no encrypted sector in
// the file yielded a key (the caller continues with the next file).
func crackFile(path string) (key [5]byte, found bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return [5]byte{}, false, err
	}
	defer f.Close()

	sec := make([]byte, SectorSize)
	reads := 0
	encrypted := 0

	for {
		if _, err := io.ReadFull(f, sec); err != nil {
			break // EOF (possibly a partial final sector) or read error — stop here
		}
		reads++

		// A non-MPEG block marks the end of the title stream.
		if sec[0] != packStart[0] || sec[1] != packStart[1] || sec[2] != packStart[2] {
			break
		}

		// PES_scrambling_control bits are not present in a system header,
		// padding_stream or private_stream2 (0x11 in {0xbb, 0xbe, 0xbf}).
		if sec[0x14]&0x30 != 0 && sec[0x11] != 0xbb && sec[0x11] != 0xbe && sec[0x11] != 0xbf {
			encrypted++
			if k, ok := attackPattern(sec); ok {
				return k, true, nil
			}
		}

		// Stop early if the file carries no scrambled sectors at all.
		if reads >= 2000 && encrypted == 0 {
			break
		}
	}

	return [5]byte{}, false, nil
}

// attackPattern performs the DeCSSPlus/Ethan Hawke-style known-plaintext attack
// on a single encrypted sector. The unencrypted region 0x00–0x7F of a CSS
// sector is searched for a repeating cycle; the attack assumes the 10 bytes
// after the cycle continue the same pattern into the encrypted region at 0x80.
func attackPattern(sec []byte) ([5]byte, bool) {
	bestPlen := 0
	bestP := 0

	for i := 2; i < 0x30; i++ {
		j := i + 1
		for ; j < 0x80 && sec[0x7f-(j%i)] == sec[0x7f-j]; j++ {
			if j > bestPlen {
				bestPlen = j
				bestP = i
			}
		}
	}

	if bestPlen > 3 && bestPlen/bestP >= 2 {
		offset := bestPlen / bestP * bestP
		key, ok := recoverTitleKey(0, sec[0x80:0x80+10], sec[0x80-offset:0x80-offset+10], sec[0x54:0x59])
		return key, ok
	}
	return [5]byte{}, false
}

// recoverTitleKey recovers the sector key from a guessed plaintext/ciphertext
// pair plus the sector seed at 0x54. On success the returned key is XORed with
// the seed, yielding the plaintext title key for the whole title set.
//
// Ported verbatim from RecoverTitleKey() in libdvdcss (GPL-2.0-or-later).
// i_start allows a previous partial scan to resume; 0 starts the full 16-bit
// candidate scan.
func recoverTitleKey(iStart int, crypted, decrypted, sectorSeed []byte) ([5]byte, bool) {
	var pBuffer [10]byte
	for i := 0; i < 10; i++ {
		pBuffer[i] = cssTab1[crypted[i]] ^ decrypted[i]
	}

	var exit int = -1
	var key [5]byte

	for iTry := iStart; iTry < 0x10000; iTry++ {
		iT1 := uint32(iTry>>8) | 0x100
		iT2 := uint32(iTry & 0xff)
		var iT3, iT5 uint32

		// Iterate the cipher 4 times to reconstruct LFSR2.
		var i uint32
		for i = 0; i < 4; i++ {
			iT4 := uint32(cssTab2[iT2]) ^ uint32(cssTab3[iT1])
			iT2 = iT1 >> 1
			iT1 = ((iT1 & 1) << 8) ^ iT4
			iT4 = uint32(cssTab5[iT4])

			iT6 := uint32(pBuffer[i])
			if iT5 != 0 {
				iT6 = (iT6 + 0xff) & 0x0ff
			}
			if iT6 < iT4 {
				iT6 += 0x100
			}
			iT6 -= iT4
			iT5 += iT6 + iT4
			iT6 = uint32(cssTab4[iT6])

			iT3 = (iT3 << 8) | iT6
			iT5 >>= 8
		}

		iCandidate := iT3

		// Iterate 6 more times to validate the candidate key.
		for ; i < 10; i++ {
			iT4 := uint32(cssTab2[iT2]) ^ uint32(cssTab3[iT1])
			iT2 = iT1 >> 1
			iT1 = ((iT1 & 1) << 8) ^ iT4
			iT4 = uint32(cssTab5[iT4])

			iT6 := (((((((iT3 >> 3) ^ iT3) >> 1) ^ iT3) >> 8) ^ iT3) >> 5) & 0xff
			iT3 = (iT3 << 8) | iT6
			iT6 = uint32(cssTab4[iT6])
			iT5 += iT6 + iT4
			if (iT5 & 0xff) != uint32(pBuffer[i]) {
				break
			}
			iT5 >>= 8
		}

		if i == 10 {
			// Do 4 backwards steps of iterating t3 to deduce the initial state.
			iT3 = iCandidate
			for i = 0; i < 4; i++ {
				iT1 := iT3 & 0xff
				iT3 >>= 8
				// Fast brute-force search for the byte shifted in.
				var j uint32
				for j = 0; j < 256; j++ {
					iT3 = (iT3 & 0x1ffff) | (j << 17)
					iT6 := (((((((iT3 >> 3) ^ iT3) >> 1) ^ iT3) >> 8) ^ iT3) >> 5) & 0xff
					if iT6 == iT1 {
						break
					}
				}
			}
			iT4 := (iT3 >> 1) - 4
			for iT5 := uint32(0); iT5 < 8; iT5++ {
				if ((iT4+iT5)*2+8-((iT4+iT5)&7)) == iT3 {
					key[0] = byte(iTry >> 8)
					key[1] = byte(iTry & 0xFF)
					key[2] = byte((iT4 + iT5) >> 0)
					key[3] = byte((iT4 + iT5) >> 8)
					key[4] = byte((iT4 + iT5) >> 16)
					exit = iTry + 1
				}
			}
		}
	}

	if exit >= 0 {
		key[0] ^= sectorSeed[0]
		key[1] ^= sectorSeed[1]
		key[2] ^= sectorSeed[2]
		key[3] ^= sectorSeed[3]
		key[4] ^= sectorSeed[4]
		return key, true
	}
	return [5]byte{}, false
}