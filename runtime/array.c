#include "array.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const int64_t SLICE_INIT_CAP = 4;

static DotSlice* slice_alloc(int64_t cap) {
    DotSlice* s = (DotSlice*)calloc(1, sizeof(DotSlice));
    if (!s) {
        fprintf(stderr, "dot_slice_alloc: out of memory\n");
        abort();
    }
    s->rc.count = 1;
    
    s->len = 0;
    s->cap = cap;
    s->data = (DotAny*)calloc((size_t)cap, sizeof(DotAny));
    if (!s->data) {
        fprintf(stderr, "dot_slice_alloc: out of memory\n");
        free(s);
        abort();
    }
    return s;
}

DotSlice* dot_slice_from_array(int64_t n, DotAny* arr) {
    if (n <= 0) {
        return slice_alloc(SLICE_INIT_CAP);
    }
    int64_t cap = SLICE_INIT_CAP;
    while (cap < n) {
        cap *= 2;
    }
    DotSlice* s = slice_alloc(cap);
    memcpy(s->data, arr, (size_t)n * sizeof(DotAny));
    s->len = n;
    return s;
}

DotAny dot_slice_get(DotSlice* s, int64_t i) {
    if (i < 0 || i >= s->len) {
        fprintf(stderr, "dot_slice_get: index %ld out of bounds (len=%ld)\n", (long)i, (long)s->len);
        abort();
    }
    return s->data[i];
}

void dot_slice_set(DotSlice* s, int64_t i, DotAny v) {
    if (i < 0 || i >= s->len) {
        fprintf(stderr, "dot_slice_set: index %ld out of bounds (len=%ld)\n", (long)i, (long)s->len);
        abort();
    }
    s->data[i] = v;
}

static void slice_grow(DotSlice* s) {
    int64_t new_cap = s->cap * 2;
    DotAny* new_data = (DotAny*)realloc(s->data, (size_t)new_cap * sizeof(DotAny));
    if (!new_data) {
        fprintf(stderr, "dot_slice_push: out of memory\n");
        abort();
    }
    memset(new_data + s->cap, 0, (size_t)s->cap * sizeof(DotAny));
    s->data = new_data;
    s->cap = new_cap;
}

void dot_slice_push(DotSlice* s, DotAny v) {
    if (s->len >= s->cap) {
        slice_grow(s);
    }
    s->data[s->len] = v;
    s->len++;
}

DotSlice* dot_slice_sub(DotSlice* s, int64_t lo, int64_t hi, bool inclusive) {
    if (inclusive) {
        hi += 1;
    }
    if (lo < 0) lo = 0;
    if (hi > s->len) hi = s->len;
    if (lo > hi) lo = hi;
    int64_t n = hi - lo;
    if (n <= 0) {
        return slice_alloc(SLICE_INIT_CAP);
    }
    return dot_slice_from_array(n, s->data + lo);
}

DotSlice* dot_slice_pack(int64_t n, DotAny* args) {
    return dot_slice_from_array(n, args);
}

int64_t dot_slice_len(DotSlice* s) {
    return s->len;
}

int64_t dot_slice_cap(DotSlice* s) {
    return s->cap;
}
