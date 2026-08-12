#ifndef DOT_RUNTIME_H
#define DOT_RUNTIME_H

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "arc.h"
#include "dot_string.h"
#include "array.h"
#include "map.h"
#include "arena.h"
#include "closure.h"
#include "iter.h"
#include "future.h"
#include "weak.h"
#include "dyn.h"
#include "builtins.h"
#include "io.h"
#include "dot_math.h"
#include "time.h"
#include "os.h"
#include "fs.h"
#include "async.h"
#include "crypto.h"
#include "regex.h"
#include "sync.h"

// stdlib C bindings mapping
#define Dot_stdin dot_file_stdin
#define Dot_stdout dot_file_stdout
#define Dot_stderr dot_file_stderr
#define Dot_open dot_file_open
#define Dot_close dot_file_close
#define Dot_read dot_file_read
#define Dot_read_all dot_file_read_all
#define Dot_write dot_file_write
#define Dot_write_all dot_file_write_all
#define Dot_print_str dot_print_str

#define Dot_async_spawn async_spawn
#define Dot_async_yield async_yield

#define Dot_dot_regex_compile dot_regex_compile
#define Dot_dot_regex_match dot_regex_match
#define Dot_dot_regex_free dot_regex_free

static inline void* dot_box_struct(size_t sz, void* src) {
    void* p = malloc(sz);
    memcpy(p, src, sz);
    return p;
}

#define dot_box_enum(dot_type, dot_val_ptr) \
    ((dot_type*)dot_box_struct(sizeof(dot_type), (void*)(dot_val_ptr)))

#endif
