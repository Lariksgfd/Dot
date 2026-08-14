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

// dot_box_struct boxes a struct or enum-pointer VALUE into a fresh heap
// object carrying a proper DotRefcnt header at offset 0 (refcount 1, no
// dtor) followed by the payload. The returned pointer is the ARC header
// pointer, so dot_retain/dot_release work on it directly and dot_release
// frees the whole allocation without ever invoking a dtor on the payload.
// Consumers of the payload (slice/iterator element reads) offset past the
// header: (char*)ptr + sizeof(DotRefcnt).
static inline void* dot_box_struct(size_t sz, void* src) {
    void* p = malloc(sizeof(DotRefcnt) + sz);
    if (!p) abort();
    DotRefcnt* rc = (DotRefcnt*)p;
    atomic_init(&rc->count, 1);
    rc->dtor = NULL;
    memcpy((char*)p + sizeof(DotRefcnt), src, sz);
    return p;
}

// dot_box_payload returns the payload of a dot_box_struct box.
static inline void* dot_box_payload(void* box) {
    return box ? (char*)box + sizeof(DotRefcnt) : NULL;
}

#endif
