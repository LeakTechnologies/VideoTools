//go:build !native_media

package media

// DiscDebugHexDump reads up to maxBytes from path and returns a hex+ASCII dump.
func DiscDebugHexDump(path string, maxBytes int) string { return "" }

// DiscFileEntry represents a single file or directory entry.
type DiscFileEntry struct {
	Name  string
	Size  int64
	IsDir bool
}

// DiscDebugListDir lists up to maxEntries files in a directory.
func DiscDebugListDir(dirPath string, maxEntries int) []DiscFileEntry { return nil }

// DiscDebugDirStat returns file count and total size for a directory.
func DiscDebugDirStat(dirPath string) (fileCount int, totalSize int64, ok bool) {
	return 0, 0, false
}
