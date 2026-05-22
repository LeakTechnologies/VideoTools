//go:build native_media

#include "disc_debug.h"
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>

#define HEX_LINE_BYTES 16

/* ── DOS directory entry (simple readdir for portability) ────────────── */
#ifdef _WIN32
#include <windows.h>
#else
#include <dirent.h>
#include <sys/stat.h>
#endif

int dump_hex(const unsigned char* data, int len, char* buf, size_t buf_sz) {
    int pos = 0;
    int i;
    for (i = 0; i < len && (size_t)pos < buf_sz; i += HEX_LINE_BYTES) {
        int remain = len - i;
        if (remain > HEX_LINE_BYTES) remain = HEX_LINE_BYTES;
        int j;

        /* offset */
        pos += snprintf(buf + pos, buf_sz - (size_t)pos, "%04x  ", i);

        /* hex bytes */
        for (j = 0; j < HEX_LINE_BYTES; j++) {
            if (j < remain)
                pos += snprintf(buf + pos, buf_sz - (size_t)pos, "%02x ", data[i + j]);
            else
                pos += snprintf(buf + pos, buf_sz - (size_t)pos, "   ");
            if (j == 7)
                pos += snprintf(buf + pos, buf_sz - (size_t)pos, " ");
        }
        pos += snprintf(buf + pos, buf_sz - (size_t)pos, " |");

        /* ASCII */
        for (j = 0; j < remain; j++) {
            unsigned char c = data[i + j];
            if (c >= 32 && c < 127)
                pos += snprintf(buf + pos, buf_sz - (size_t)pos, "%c", c);
            else
                pos += snprintf(buf + pos, buf_sz - (size_t)pos, ".");
        }
        pos += snprintf(buf + pos, buf_sz - (size_t)pos, "|\n");

        if ((size_t)pos >= buf_sz) return -1;
    }
    return pos;
}

int read_file_hex(const char* path, int max_bytes, char* buf, size_t buf_sz) {
    FILE* f = fopen(path, "rb");
    if (!f) {
        if (errno == ENOENT) return 0;
        return -errno;
    }

    unsigned char* data = (unsigned char*)malloc((size_t)max_bytes);
    if (!data) { fclose(f); return -2; }

    int nread = (int)fread(data, 1, (size_t)max_bytes, f);
    fclose(f);

    int result;
    if (nread <= 0) {
        result = 0;
    } else {
        result = dump_hex(data, nread, buf, buf_sz);
    }

    free(data);
    return result;
}

int list_directory(const char* dir_path, FileEntry* entries, int max_entries) {
    if (!dir_path || !entries || max_entries <= 0) return -1;

    int count = 0;

#ifdef _WIN32
    char pattern[1024];
    snprintf(pattern, sizeof(pattern), "%s\\*", dir_path);

    WIN32_FIND_DATAA ffd;
    HANDLE hFind = FindFirstFileA(pattern, &ffd);
    if (hFind == INVALID_HANDLE_VALUE) return -GetLastError();

    do {
        if (count >= max_entries) break;
        strncpy(entries[count].name, ffd.cFileName, sizeof(entries[count].name) - 1);
        entries[count].name[sizeof(entries[count].name) - 1] = '\0';
        entries[count].size = ((int64_t)ffd.nFileSizeHigh << 32) | ffd.nFileSizeLow;
        entries[count].is_dir = (ffd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) ? 1 : 0;
        count++;
    } while (FindNextFileA(hFind, &ffd) != 0);

    FindClose(hFind);
#else
    DIR* d = opendir(dir_path);
    if (!d) return -errno;

    struct dirent* entry;
    while ((entry = readdir(d)) != NULL && count < max_entries) {
        strncpy(entries[count].name, entry->d_name, sizeof(entries[count].name) - 1);
        entries[count].name[sizeof(entries[count].name) - 1] = '\0';

        char full_path[1024];
        snprintf(full_path, sizeof(full_path), "%s/%s", dir_path, entry->d_name);
        struct stat st;
        if (stat(full_path, &st) == 0) {
            entries[count].size = st.st_size;
            entries[count].is_dir = S_ISDIR(st.st_mode) ? 1 : 0;
        } else {
            entries[count].size = 0;
            entries[count].is_dir = 0;
        }
        count++;
    }
    closedir(d);
#endif

    return count;
}

int dir_stat(const char* dir_path, int* file_count_out, int64_t* total_size_out) {
    if (!dir_path || !file_count_out || !total_size_out) return -1;

    int count = 0;
    int64_t total = 0;

#ifdef _WIN32
    char pattern[1024];
    snprintf(pattern, sizeof(pattern), "%s\\*", dir_path);

    WIN32_FIND_DATAA ffd;
    HANDLE hFind = FindFirstFileA(pattern, &ffd);
    if (hFind == INVALID_HANDLE_VALUE) return -GetLastError();

    do {
        if (!(ffd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) {
            count++;
            total += ((int64_t)ffd.nFileSizeHigh << 32) | ffd.nFileSizeLow;
        }
    } while (FindNextFileA(hFind, &ffd) != 0);

    FindClose(hFind);
#else
    DIR* d = opendir(dir_path);
    if (!d) return -errno;

    struct dirent* entry;
    while ((entry = readdir(d)) != NULL) {
        char full_path[1024];
        snprintf(full_path, sizeof(full_path), "%s/%s", dir_path, entry->d_name);
        struct stat st;
        if (stat(full_path, &st) == 0 && S_ISREG(st.st_mode)) {
            count++;
            total += st.st_size;
        }
    }
    closedir(d);
#endif

    *file_count_out = count;
    *total_size_out = total;
    return 0;
}
