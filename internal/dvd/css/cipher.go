package css

// unscrambleSector decrypts an encrypted DVD sector in-place.
//
// Ported from dvdcss_unscramble() in libdvdcss (GPL-2.0-or-later).
//
// The sector must be 2048 bytes. Bytes 0x14 & 0x30 indicate whether the sector
// is scrambled; encrypted payload starts at offset 0x80 and runs to 0x800.
// Key mixing uses the 5-byte title key XORed with bytes 0x54–0x58 of the sector.
func unscrambleSector(key [5]byte, sec []byte) {
	if len(sec) < 0x800 {
		return
	}
	// Only scrambled sectors need decryption.
	if sec[0x14]&0x30 == 0 {
		return
	}

	// LFSR1: 9-bit (t1 carries the 9th bit as 0x100).
	t1 := uint32(key[0]^sec[0x54]) | 0x100
	t2 := uint32(key[1]^sec[0x55]) & 0x7F

	// LFSR2: 32-bit, left-shifting.
	t3 := (uint32(key[2]) ^ uint32(sec[0x56])) |
		((uint32(key[3]) ^ uint32(sec[0x57])) << 8) |
		((uint32(key[4]) ^ uint32(sec[0x58])) << 16)
	t4 := t3 & 7
	t3 = t3*2 + 8 - t4

	var t5 uint32
	for i := 0x80; i < 0x800; i++ {
		// Clock LFSR1 (9-bit).
		t4 = uint32(cssTab2[t2]) ^ uint32(cssTab3[t1])
		t2 = t1 >> 1
		t1 = ((t1 & 1) << 8) ^ t4

		// Tab5 lookup on the combined LFSR1 output byte.
		t6lfsr1 := uint32(cssTab5[t4])

		// Clock LFSR2 (32-bit, left-shift Galois LFSR).
		t6 := ((((((t3 >> 3) ^ t3) >> 1) ^ t3) >> 8) ^ t3) >> 5 & 0xff
		t3 = (t3 << 8) | t6
		t6lfsr2 := uint32(cssTab4[t6])

		// Accumulate both LFSR contributions.
		t5 += t6lfsr2 + t6lfsr1

		sec[i] = cssTab1[sec[i]] ^ uint8(t5)
		t5 >>= 8
	}

	// Clear the scrambling flag.
	sec[0x14] &^= 0x30
}

// decryptKey implements the CSS key decryption cipher.
//
// Ported from DecryptKey() in libdvdcss (GPL-2.0-or-later).
//
// invert is 0x00 for disc-key decryption and 0xff for title-key decryption.
// key is the 5-byte base key; crypted is the 5-byte encrypted input.
func decryptKey(invert byte, key [5]byte, crypted [5]byte) (result [5]byte) {
	// LFSR1: 9-bit.
	lfsr1Lo := uint(key[0]) | 0x100
	lfsr1Hi := uint(key[1])

	// LFSR0: 32-bit, right-shifting.  Initial value from key[2..4].
	lfsr0 := (uint32(key[4]) << 17) | (uint32(key[3]) << 9) | (uint32(key[2]) << 1)
	lfsr0 += 8 - uint32(key[2]&7)
	lfsr0 = (uint32(cssTab4[lfsr0&0xff]) << 24) |
		(uint32(cssTab4[(lfsr0>>8)&0xff]) << 16) |
		(uint32(cssTab4[(lfsr0>>16)&0xff]) << 8) |
		uint32(cssTab4[(lfsr0>>24)&0xff])

	var combined uint32
	var k [5]byte
	for i := 0; i < 5; i++ {
		// Clock LFSR1 (9-bit).
		oLfsr1 := uint(cssTab2[lfsr1Hi]) ^ uint(cssTab3[lfsr1Lo])
		lfsr1Hi = lfsr1Lo >> 1
		lfsr1Lo = ((lfsr1Lo & 1) << 8) ^ oLfsr1
		oLfsr1b := uint32(cssTab4[oLfsr1])

		// Clock LFSR0 (32-bit, right-shift Galois LFSR).
		oLfsr0 := ((((((lfsr0 >> 8) ^ lfsr0) >> 1) ^ lfsr0) >> 3) ^ lfsr0) >> 7 & 0xff
		lfsr0 = (lfsr0 >> 8) | (oLfsr0 << 24)

		combined += (oLfsr0 ^ uint32(invert)) + oLfsr1b
		k[i] = byte(combined)
		combined >>= 8
	}

	// XOR chain to produce the 5-byte result.
	result[4] = k[4] ^ cssTab1[crypted[4]] ^ crypted[3]
	result[3] = k[3] ^ cssTab1[crypted[3]] ^ crypted[2]
	result[2] = k[2] ^ cssTab1[crypted[2]] ^ crypted[1]
	result[1] = k[1] ^ cssTab1[crypted[1]] ^ crypted[0]
	result[0] = k[0] ^ cssTab1[crypted[0]] ^ result[4]
	return result
}

// DecryptTitleKey decrypts an encrypted title key using the disc key.
//
// discKey is the 5-byte disc key obtained from the drive.
// encTitleKey is the 5-byte encrypted title key from the IFO file.
func DecryptTitleKey(discKey [5]byte, encTitleKey [5]byte) [5]byte {
	return decryptKey(0xff, discKey, encTitleKey)
}

// DecryptDiscKey decrypts a candidate disc key from the disc key block.
//
// playerKey is a known DVD player key.
// encDiscKey is the 5-byte encrypted disc key entry from the disc key block.
func DecryptDiscKey(playerKey [5]byte, encDiscKey [5]byte) [5]byte {
	return decryptKey(0x00, playerKey, encDiscKey)
}
