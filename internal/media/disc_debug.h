#pragma once
#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
 * DumpHex — writes hex bytes and printable ASCII of `data[0..len-1]`
 * to buf (null-terminated). Returns the number of chars written (not
 * including the null terminator) or < 0 on truncation.
 * Max output for 512 bytes at 16 bytes/line ≈ 512/16*(16*2 + 1 + 16 + 1) ≈ 4KB.
 */
int dump_hex(const unsigned char* data, int len, char* buf, size_t buf_sz);

/*
 * ReadFileHex — opens `path`, reads up to `max_bytes`, and writes a
 * hex dump of the bytes to `buf`. Returns > 0 on success, 0 on file-not-found,
 * negative on error.
 */
int read_file_hex(const char* path, int max_bytes, char* buf, size_t buf_sz);

/*
 * FileEntry — name + size for one directory entry.
 */
typedef struct {
    char  name[256];
    int64_t size;
    int    is_dir;
} FileEntry;

/*
 * ListDir — fills `entries` with up to `max_entries` files from `dir_path`.
 * Returns the number of entries written, or negative on error.
 */
int list_directory(const char* dir_path, FileEntry* entries, int max_entries);

/*
 * DirStat — returns total file count and total size for a directory.
 * file_count_out / total_size_out are set on success.
 * Returns 0 on success, negative on error.
 */
int dir_stat(const char* dir_path, int* file_count_out, int64_t* total_size_out);

#ifdef __cplusplus
}
#endif
