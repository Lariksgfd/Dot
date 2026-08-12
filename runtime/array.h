#ifndef DOT_ARRAY_H
#define DOT_ARRAY_H

#include <stdint.h>
#include <stdbool.h>
#include "arc.h"

typedef struct DotSlice {
    DotRefcnt rc;
    int64_t len;
    int64_t cap;
    DotAny* data;
} DotSlice;

DotSlice* dot_slice_from_array(int64_t n, DotAny* arr);
DotAny dot_slice_get(DotSlice* s, int64_t i);
void dot_slice_set(DotSlice* s, int64_t i, DotAny v);
void dot_slice_push(DotSlice* s, DotAny v);
DotSlice* dot_slice_sub(DotSlice* s, int64_t lo, int64_t hi, bool inclusive);
DotSlice* dot_slice_pack(int64_t n, DotAny* args);
int64_t dot_slice_len(DotSlice* s);
int64_t dot_slice_cap(DotSlice* s);

#endif
