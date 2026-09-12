package iso9660

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

const (
	SectorSize = 2048
	pvdSector  = 16
	pvdType    = 1
	isoMagic   = "CD001"
)

// byteRange is a single contiguous extent of a file.
type byteRange struct {
	extent uint32
	size   uint32
}

// fileNode describes one ISO 9660 directory entry.
type fileNode struct {
	name   string
	path   string
	dir    bool
	ranges []byteRange
}

// Reader reads an ISO 9660 volume from an io.ReadSeeker. It mirrors the small
// surface of the UDF reader (ReadFileData / ExtractDirectory) so DVD and
// Blu-ray ISO images that carry a broken or dummy UDF bridge can still be
// ripped through their ISO 9660 filesystem.
type Reader struct {
	rs   io.ReadSeeker
	mu   sync.Mutex
	root *fileNode
}

func NewReader(rs io.ReadSeeker) *Reader {
	return &Reader{rs: rs}
}

// Cleanup is a no-op kept for interface parity with the UDF reader.
func (r *Reader) Cleanup() {}

// rootDir returns the volume's root directory from the Primary Volume
// Descriptor at sector 16, validating the "CD001" magic and descriptor type.
func (r *Reader) rootDir() (*fileNode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.root != nil {
		return r.root, nil
	}

	buf := make([]byte, SectorSize)
	if _, err := r.rs.Seek(int64(pvdSector)*SectorSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to ISO 9660 PVD sector %d: %w", pvdSector, err)
	}
	if _, err := io.ReadFull(r.rs, buf); err != nil {
		return nil, fmt.Errorf("read ISO 9660 PVD sector %d: %w", pvdSector, err)
	}
	if buf[0] != pvdType || string(buf[1:1+len(isoMagic)]) != isoMagic {
		return nil, fmt.Errorf("no ISO 9660 primary volume descriptor at sector %d (type=%d magic=%q)",
			pvdSector, buf[0], string(buf[1:1+len(isoMagic)]))
	}

	// Root directory record: 34 bytes starting at PVD offset 156. Both-endian
	// extent and size are stored; prefer the little-endian values.
	extent := binary.LittleEndian.Uint32(buf[158:162])
	size := binary.LittleEndian.Uint32(buf[166:170])
	if extent == 0 {
		extent = binary.BigEndian.Uint32(buf[162:166])
		size = binary.BigEndian.Uint32(buf[170:174])
	}
	if extent == 0 || size == 0 {
		return nil, fmt.Errorf("ISO 9660 root directory has no extent (ext=%d size=%d)", extent, size)
	}

	r.root = &fileNode{name: "", path: "", dir: true, ranges: []byteRange{{extent, size}}}
	logging.Debug(logging.CatDVD, "ISO 9660 root dir at sector %d (%d bytes)", extent, size)
	return r.root, nil
}

// readDir parses the directory records of dir. Multi-extent files (flag 0x80)
// have their continuation records folded into the leading record's ranges.
func (r *Reader) readDir(dir *fileNode) ([]*fileNode, error) {
	data := make([]byte, dir.ranges[0].size)
	r.mu.Lock()
	if _, err := r.rs.Seek(int64(dir.ranges[0].extent)*SectorSize, io.SeekStart); err != nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("seek to directory %q: %w", dir.path, err)
	}
	if _, err := io.ReadFull(r.rs, data); err != nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("read directory %q: %w", dir.path, err)
	}
	r.mu.Unlock()

	var out []*fileNode
	var pending *fileNode
	for pos := 0; pos < len(data); {
		recLen := int(data[pos])
		if recLen == 0 {
			pos += SectorSize - (pos % SectorSize)
			continue
		}
		flags := data[pos+25]
		nLen := int(data[pos+32])
		end := pos + 33 + nLen
		if end > len(data) {
			break
		}
		name := data[pos+33 : end]

		// Continuation record of a multi-extent file: no name, flag 0x80.
		if len(name) == 0 && flags&0x80 != 0 && pending != nil {
			pending.ranges = append(pending.ranges, byteRange{
				extent: uint32(binary.LittleEndian.Uint32(data[pos+2 : pos+6])),
				size:   uint32(binary.LittleEndian.Uint32(data[pos+10 : pos+14])),
			})
			pos += recLen
			continue
		}

		// "." and ".." entries.
		if len(name) > 0 && (name[0] == 0 || name[0] == 1) {
			pos += recLen
			continue
		}

		fname := string(name)
		if idx := strings.LastIndexByte(fname, ';'); idx >= 0 {
			fname = fname[:idx]
		}
		if fname == "" {
			pos += recLen
			continue
		}

		node := &fileNode{
			name: fname,
			path: func() string {
				if dir.path == "" {
					return fname
				}
				return dir.path + "/" + fname
			}(),
			dir:  flags&0x02 != 0,
			ranges: []byteRange{{
				extent: uint32(binary.LittleEndian.Uint32(data[pos+2 : pos+6])),
				size:   uint32(binary.LittleEndian.Uint32(data[pos+10 : pos+14])),
			}},
		}
		if node.ranges[0].extent == 0 {
			node.ranges[0].extent = binary.BigEndian.Uint32(data[pos+6 : pos+10])
			node.ranges[0].size = binary.BigEndian.Uint32(data[pos+14 : pos+18])
		}
		if node.ranges[0].size == 0 && !node.dir {
			pos += recLen
			continue
		}

		if flags&0x80 != 0 {
			pending = node
		} else {
			pending = nil
		}
		out = append(out, node)
		pos += recLen
	}
	return out, nil
}

// resolve walks path components ("VIDEO_TS/VIDEO_TS.IFO") from the root,
// matching each component case-insensitively.
func (r *Reader) resolve(parts []string) (*fileNode, error) {
	cur, err := r.rootDir()
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return cur, nil
	}
	for i, part := range parts {
		children, err := r.readDir(cur)
		if err != nil {
			return nil, err
		}
		var next *fileNode
		for _, c := range children {
			if strings.EqualFold(c.name, part) {
				next = c
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("path component %q not found in ISO 9660 image", part)
		}
		if i < len(parts)-1 && !next.dir {
			return nil, fmt.Errorf("expected directory at component %q", part)
		}
		cur = next
	}
	return cur, nil
}

// ReadFileData returns the raw bytes of a single file identified by its
// ISO 9660 path (e.g. "VIDEO_TS/VIDEO_TS.IFO"). Matching is case-insensitive.
func (r *Reader) ReadFileData(path string) ([]byte, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	node, err := r.resolve(parts)
	if err != nil {
		return nil, err
	}
	if node.dir {
		return nil, fmt.Errorf("expected file but found directory %q", path)
	}

	var buf bytes.Buffer
	for _, sp := range node.ranges {
		chunk := make([]byte, sp.size)
		r.mu.Lock()
		_, seekErr := r.rs.Seek(int64(sp.extent)*SectorSize, io.SeekStart)
		var readErr error
		if seekErr == nil {
			_, readErr = io.ReadFull(r.rs, chunk)
		}
		r.mu.Unlock()
		if seekErr != nil {
			return nil, fmt.Errorf("seek to file extent at %d: %w", sp.extent, seekErr)
		}
		if readErr != nil {
			return nil, fmt.Errorf("read file extent at %d: %w", sp.extent, readErr)
		}
		buf.Write(chunk)
	}
	return buf.Bytes(), nil
}

// ExtractDirectory extracts a directory (like VIDEO_TS) and every descendant
// from the ISO to destPath, mirroring the UDF reader's behaviour. ctx may be
// used to cancel mid-flight.
func (r *Reader) ExtractDirectory(ctx context.Context, targetDir, destPath string) error {
	root, err := r.rootDir()
	if err != nil {
		return err
	}

	var target *fileNode
	var toVisit = []*fileNode{root}
	for len(toVisit) > 0 {
		dir := toVisit[0]
		toVisit = toVisit[1:]
		children, err := r.readDir(dir)
		if err != nil {
			logging.Warning(logging.CatDVD, "ISO 9660: skipping unreadable directory %q: %v", dir.path, err)
			continue
		}
		for _, c := range children {
			if c.dir && strings.EqualFold(c.name, targetDir) && !strings.Contains(c.path, "/") {
				target = c
			}
			if c.dir {
				toVisit = append(toVisit, c)
			}
		}
	}
	if target == nil {
		return fmt.Errorf("directory %q not found in ISO 9660 image", targetDir)
	}

	prefix := target.path + "/"
	var all []*fileNode
	toVisit = []*fileNode{target}
	for len(toVisit) > 0 {
		dir := toVisit[0]
		toVisit = toVisit[1:]
		children, err := r.readDir(dir)
		if err != nil {
			return fmt.Errorf("read directory %q: %w", dir.path, err)
		}
		for _, c := range children {
			all = append(all, c)
			if c.dir {
				toVisit = append(toVisit, c)
			}
		}
	}

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	for _, n := range all {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel := strings.TrimPrefix(n.path, prefix)
		if n.path == prefix[:len(prefix)-1] || !strings.HasPrefix(n.path, prefix) {
			continue
		}
		dest := filepath.Join(destPath, filepath.FromSlash(rel))
		if n.dir {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return fmt.Errorf("create dir %s: %w", rel, err)
			}
			continue
		}
		if err := r.writeFile(ctx, n, dest); err != nil {
			return fmt.Errorf("extract %s: %w", n.path, err)
		}
	}
	return nil
}

func (r *Reader) writeFile(ctx context.Context, n *fileNode, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, sp := range n.ranges {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		r.mu.Lock()
		_, seekErr := r.rs.Seek(int64(sp.extent)*SectorSize, io.SeekStart)
		var copyErr error
		if seekErr == nil {
			_, copyErr = io.CopyN(f, io.LimitReader(r.rs, int64(sp.size)), int64(sp.size))
		}
		r.mu.Unlock()
		if seekErr != nil {
			return fmt.Errorf("seek to file extent at %d: %w", sp.extent, seekErr)
		}
		if copyErr != nil {
			return fmt.Errorf("copy file extent at %d: %w", sp.extent, copyErr)
		}
	}
	return nil
}