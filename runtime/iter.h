#ifndef DOT_ITER_H
#define DOT_ITER_H

#include <stdbool.h>
#include <stdint.h>
#include "arc.h"

typedef struct DotSlice DotSlice;
typedef struct DotMap DotMap;

typedef struct {
    int64_t idx;
    int64_t end;
    DotAny  collection;
    int8_t  kind;
    DotAny* map_keys;
    int64_t map_idx;
    int64_t map_len;
} DotIter;

DotIter dot_iter_slice(DotSlice* s);
DotIter dot_iter_map(DotMap* m);
bool    dot_iter_next(DotIter* it, DotAny* key, DotAny* val);

#endif
