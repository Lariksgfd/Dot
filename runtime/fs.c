#include "fs.h"

#ifdef _WIN32
#include <windows.h>
#include <sys/stat.h>
#include <direct.h>
#include <stdio.h>
#else
#include <sys/stat.h>
#include <dirent.h>
#include <unistd.h>
#include <stdio.h>
#endif

extern DotAny dot_alloc(int32_t size);

bool dot_fs_exists(DotString* path) {
    if (!path) return false;
#ifdef _WIN32
    struct _stat st;
    return _stat(path->data, &st) == 0;
#else
    struct stat st;
    return stat(path->data, &st) == 0;
#endif
}

bool dot_fs_is_dir(DotString* path) {
    if (!path) return false;
#ifdef _WIN32
    struct _stat st;
    if (_stat(path->data, &st) != 0) return false;
    return (st.st_mode & _S_IFDIR) != 0;
#else
    struct stat st;
    if (stat(path->data, &st) != 0) return false;
    return S_ISDIR(st.st_mode);
#endif
}

bool dot_fs_is_file(DotString* path) {
    if (!path) return false;
#ifdef _WIN32
    struct _stat st;
    if (_stat(path->data, &st) != 0) return false;
    return (st.st_mode & _S_IFREG) != 0;
#else
    struct stat st;
    if (stat(path->data, &st) != 0) return false;
    return S_ISREG(st.st_mode);
#endif
}

int64_t dot_fs_mkdir(DotString* path) {
    if (!path) return -1;
#ifdef _WIN32
    return _mkdir(path->data);
#else
    return mkdir(path->data, 0755);
#endif
}

int64_t dot_fs_remove(DotString* path) {
    if (!path) return -1;
#ifdef _WIN32
    return _unlink(path->data);
#else
    return unlink(path->data);
#endif
}

int64_t dot_fs_rename(DotString* old, DotString* new_path) {
    if (!old || !new_path) return -1;
    return rename(old->data, new_path->data);
}

DotSlice* dot_fs_read_dir(DotString* path) {
    DotSlice* s = dot_slice_from_array(0, NULL);
    if (!path) return s;

#ifdef _WIN32
    char pattern[8192];
    int plen = snprintf(pattern, sizeof(pattern), "%s\\*", path->data);
    if (plen < 0 || (size_t)plen >= sizeof(pattern)) return s;

    WIN32_FIND_DATAA fd;
    HANDLE h = FindFirstFileA(pattern, &fd);
    if (h == INVALID_HANDLE_VALUE) return s;

    do {
        if (strcmp(fd.cFileName, ".") == 0 || strcmp(fd.cFileName, "..") == 0) continue;
        DotString* name = dot_string_from_lit(fd.cFileName, (int64_t)strlen(fd.cFileName));
        dot_slice_push(s, (DotAny)name);
    } while (FindNextFileA(h, &fd));

    FindClose(h);
#else
    DIR* dir = opendir(path->data);
    if (!dir) return s;

    struct dirent* entry;
    while ((entry = readdir(dir)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) continue;
        DotString* name = dot_string_from_lit(entry->d_name, (int64_t)strlen(entry->d_name));
        dot_slice_push(s, (DotAny)name);
    }

    closedir(dir);
#endif

    return s;
}

DotFileInfo* dot_fs_stat(DotString* path) {
    if (!path) return NULL;
    DotFileInfo* fi = (DotFileInfo*)dot_alloc((int32_t)sizeof(DotFileInfo));
    fi->name = path;
#ifdef _WIN32
    struct _stat st;
    if (_stat(path->data, &st) == 0) {
        fi->size = (int64_t)st.st_size;
        fi->is_dir = (st.st_mode & _S_IFDIR) != 0;
        fi->mod_time = (int64_t)st.st_mtime;
    } else {
        fi->size = -1;
        fi->is_dir = false;
        fi->mod_time = 0;
    }
#else
    struct stat st;
    if (stat(path->data, &st) == 0) {
        fi->size = (int64_t)st.st_size;
        fi->is_dir = S_ISDIR(st.st_mode);
        fi->mod_time = (int64_t)st.st_mtime;
    } else {
        fi->size = -1;
        fi->is_dir = false;
        fi->mod_time = 0;
    }
#endif
    return fi;
}

DotString* dot_fs_ext(DotString* path) {
    if (!path || path->len == 0) return dot_string_from_lit("", 0);
    const char* p = path->data;
    int64_t len = path->len;
    int64_t i;
    for (i = len - 1; i >= 0; i--) {
        if (p[i] == '.') {
            return dot_string_from_lit(p + i + 1, len - i - 1);
        }
        if (p[i] == '/' || p[i] == '\\') break;
    }
    return dot_string_from_lit("", 0);
}

DotString* dot_fs_basename(DotString* path) {
    if (!path || path->len == 0) return dot_string_from_lit("", 0);
    const char* p = path->data;
    int64_t len = path->len;
    int64_t i;
    for (i = len - 1; i >= 0; i--) {
        if (p[i] == '/' || p[i] == '\\') {
            return dot_string_from_lit(p + i + 1, len - i - 1);
        }
    }
    return dot_string_from_lit(p, len);
}
