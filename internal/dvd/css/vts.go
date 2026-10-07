package css

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DecryptVideoTS copies a VIDEO_TS directory into dstDir with all
// CSS-scrambled VOB payloads decrypted, so that an external demuxer (our
// FFmpeg is not linked with libdvdcss) can read the tree like a clear disc.
//
// CSS scrambles only VOB data; IFO/BUP and every other non-VOB file pass
// through verbatim. VOBs are grouped per title set, because each VTS carries
// its own CSS title key (VTS_XX_1..N.VOB share the VTS_XX_0 menu key;
// VIDEO_TS.VOB is its own VMG-domain group). A group that is entirely clear is
// copied as-is; a group with any scrambled member has its key cracked from
// those members and then all scrambled members are decrypted with it.
//
// Decryption never silently passes scrambled data: if an encrypted group's
// title key cannot be recovered, an error is returned and the caller must
// abort (the copy is left partial for os.RemoveAll).
func DecryptVideoTS(srcDir, dstDir string, logf func(string)) error {
	if logf == nil {
		logf = func(string) {}
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create decrypted VIDEO_TS: %w", err)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read VIDEO_TS: %w", err)
	}

	groups := map[string][]string{}
	var order []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		if !strings.EqualFold(filepath.Ext(e.Name()), ".vob") {
			if err := copyFileNV(src, filepath.Join(dstDir, e.Name())); err != nil {
				return err
			}
			continue
		}
		trimmed := strings.TrimSuffix(strings.ToUpper(e.Name()), filepath.Ext(e.Name()))
		parts := strings.Split(trimmed, "_")
		group := strings.Join(parts[:min(2, len(parts))], "_")
		if _, ok := groups[group]; !ok {
			order = append(order, group)
		}
		groups[group] = append(groups[group], src)
	}
	sort.Strings(order)

	for _, g := range order {
		var scrambled, clear []string
		for _, m := range groups[g] {
			enc, err := DetectEncryption(m)
			if err != nil {
				return fmt.Errorf("detect encryption %s: %w", filepath.Base(m), err)
			}
			if enc {
				scrambled = append(scrambled, m)
			} else {
				clear = append(clear, m)
			}
		}

		for _, m := range clear {
			if err := copyFileNV(m, filepath.Join(dstDir, filepath.Base(m))); err != nil {
				return err
			}
		}

		if len(scrambled) == 0 {
			continue
		}

		key, err := CrackTitleKeyFiles(scrambled)
		if err != nil {
			return fmt.Errorf("cannot recover CSS title key for %s: %w", g, err)
		}
		d := NewDecryptor(key)
		for _, m := range scrambled {
			out := filepath.Join(dstDir, filepath.Base(m))
			n, err := DecryptVOB(m, out, d)
			if err != nil {
				return fmt.Errorf("decrypt %s: %w", filepath.Base(m), err)
			}
			logf(fmt.Sprintf("CSS: decrypted %s (%d bytes)", filepath.Base(m), n))
		}
	}

	return nil
}

// copyFileNV copies src to dst bit-for-bit (non-VOB files are never scrambled).
func copyFileNV(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", filepath.Base(src), err)
	}
	return nil
}