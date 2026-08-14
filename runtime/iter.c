#include "iter.h"
#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>

typedef struct {
    DotRefcnt rc;
    int64_t len;
    int64_t cap;
    DotAny* data;
} SlicePriv;

typedef struct {
    uint64_t hash;
    DotAny   key;
    DotAny   value;
} MapEntry;

typedef struct {
    DotRefcnt rc;
    int64_t   len;
    int64_t   cap;
    MapEntry* entries;
} MapPriv;

DotIter dot_iter_slice(DotSlice* s) {
    DotIter it;
    SlicePriv* sp = (SlicePriv*)s;
    it.idx       = 0;
    it.end       = sp->len;
    it.collection = (DotAny)s;
    it.kind      = 0;
    it.map_keys  = NULL;
    it.map_idx   = 0;
    it.map_len   = 0;
    return it;
}

DotIter dot_iter_map(DotMap* m) {
    DotIter it;
    MapPriv* mp = (MapPriv*)m;
    it.idx       = 0;
    it.end       = 0;
    it.collection = (DotAny)m;
    it.kind      = 1;
    it.map_len   = mp->len;
    it.map_idx   = 0;

    if (mp->len > 0 && mp->entries) {
        it.map_keys = (DotAny*)calloc((size_t)mp->len, sizeof(DotAny));
        if (!it.map_keys) {
            fprintf(stderr, "dot_iter_map: out of memory\n");
            abort();
        }
        int64_t j = 0;
        for (int64_t i = 0; i < mp->cap && j < mp->len; i++) {
            if (mp->entries[i].key != NULL) {
                it.map_keys[j] = mp->entries[i].key;
                j++;
            }
        }
    } else {
        it.map_keys = NULL;
    }
    return it;
}

bool dot_iter_next(DotIter* it, DotAny* key, DotAny* val) {
    if (it->kind == 0) {
        if (it->idx >= it->end) return false;
        SlicePriv* sp = (SlicePriv*)it->collection;
        if (key) *key = (DotAny)(intptr_t)it->idx;
        if (val) *val = sp->data[it->idx];
        it->idx++;
        return true;
    }

    if (it->map_idx >= it->map_len) return false;
    MapPriv* mp = (MapPriv*)it->collection;
    DotAny k = it->map_keys[it->map_idx];
    if (key) *key = k;
    for (int64_t i = 0; i < mp->cap; i++) {
        if (mp->entries[i].key == k) {
            if (val) *val = mp->entries[i].value;
            break;
        }
    }
    it->map_idx++;
    return true;
}
