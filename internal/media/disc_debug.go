//go:build native_media

package media

/*
#include "disc_debug.h"
#include "disc_debug.c"
*/
import "C"
import (
	"unsafe"
)

// DiscDebugHexDump reads up to maxBytes from path and returns a hex+ASCII dump.
// Returns empty string on error.
func DiscDebugHexDump(path string, maxBytes int) string {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var buf [8192]byte
	n := C.read_file_hex(cPath, C.int(maxBytes), (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}

// DiscFileEntry represents a single file or directory entry.
type DiscFileEntry struct {
	Name  string
	Size  int64
	IsDir bool
}

// DiscDebugListDir lists up to maxEntries files in a directory via C (FindFirstFile / opendir).
func DiscDebugListDir(dirPath string, maxEntries int) []DiscFileEntry {
	cPath := C.CString(dirPath)
	defer C.free(unsafe.Pointer(cPath))

	entries := make([]C.FileEntry, maxEntries)
	n := C.list_directory(cPath, &entries[0], C.int(maxEntries))
	if n <= 0 {
		return nil
	}

	result := make([]DiscFileEntry, int(n))
	for i := 0; i < int(n); i++ {
		result[i] = DiscFileEntry{
			Name:  C.GoString(&entries[i].name[0]),
			Size:  int64(entries[i].size),
			IsDir: entries[i].is_dir != 0,
		}
	}
	return result
}

// DiscDebugDirStat returns file count and total size for a directory.
func DiscDebugDirStat(dirPath string) (fileCount int, totalSize int64, ok bool) {
	cPath := C.CString(dirPath)
	defer C.free(unsafe.Pointer(cPath))

	var cCount C.int
	var cSize C.int64_t
	ret := C.dir_stat(cPath, &cCount, &cSize)
	if ret != 0 {
		return 0, 0, false
	}
	return int(cCount), int64(cSize), true
}
