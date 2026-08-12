#ifndef RUNTIME_FS_H
#define RUNTIME_FS_H

#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include "dot_string.h"
#include "array.h"

typedef struct {
    DotString* name;
    int64_t size;
    bool is_dir;
    int64_t mod_time;
} DotFileInfo;

bool dot_fs_exists(DotString* path);
bool dot_fs_is_dir(DotString* path);
bool dot_fs_is_file(DotString* path);
int64_t dot_fs_mkdir(DotString* path);
int64_t dot_fs_remove(DotString* path);
int64_t dot_fs_rename(DotString* old, DotString* new_path);
DotSlice* dot_fs_read_dir(DotString* path);
DotFileInfo* dot_fs_stat(DotString* path);
DotString* dot_fs_ext(DotString* path);
DotString* dot_fs_basename(DotString* path);

#endif

