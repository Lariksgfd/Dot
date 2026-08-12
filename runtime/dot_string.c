#include <stdlib.h>
#include "dot_string.h"
#include <stdio.h>
#include <string.h>
#include <stdint.h>

extern DotAny dot_alloc(int32_t size);

extern int64_t dot_slice_len(DotSlice* s);
extern DotAny dot_slice_get(DotSlice* s, int64_t i);
extern DotSlice* dot_slice_from_array(int64_t n, DotAny* arr);

DotString* dot_string_from_lit(const char* ptr, int64_t len) {
    DotString* s = (DotString*)dot_alloc((int32_t)(sizeof(DotString) + (size_t)(len + 1)));
    s->len = len;
    memcpy(s->data, ptr, (size_t)len);
    s->data[len] = '\0';
    return s;
}

DotString* dot_string_from_bytes(DotSlice* bytes) {
    if (!bytes) {
        return dot_string_from_lit("", 0);
    }
    int64_t len = dot_slice_len(bytes);
    DotString* s = (DotString*)dot_alloc((int32_t)(sizeof(DotString) + (size_t)(len + 1)));
    s->len = len;
    for (int64_t i = 0; i < len; i++) {
        s->data[i] = (char)(intptr_t)dot_slice_get(bytes, i);
    }
    s->data[len] = '\0';
    return s;
}

DotSlice* dot_string_to_bytes(DotString* s) {
    if (!s) {
        return dot_slice_from_array(0, NULL);
    }
    DotAny* tmp = (DotAny*)malloc((size_t)s->len * sizeof(DotAny));
    if (!tmp && s->len > 0) {
        fprintf(stderr, "dot_string_to_bytes: out of memory\n");
        abort();
    }
    for (int64_t i = 0; i < s->len; i++) {
        tmp[i] = (DotAny)(intptr_t)(unsigned char)s->data[i];
    }
    DotSlice* out = dot_slice_from_array(s->len, tmp);
    free(tmp);
    return out;
}

DotString* dot_string_concat(DotString* a, DotString* b) {
    if (!a && !b) return dot_string_from_lit("", 0);
    if (!a) { dot_retain((DotRefcnt*)b); return b; }
    if (!b) { dot_retain((DotRefcnt*)a); return a; }
    int64_t len_a = a->len;
    int64_t len_b = b->len;
    int64_t total = len_a + len_b;
    DotString* s = (DotString*)dot_alloc((int32_t)(sizeof(DotString) + (size_t)(total + 1)));
    s->len = total;
    memcpy(s->data, a->data, (size_t)len_a);
    memcpy(s->data + len_a, b->data, (size_t)len_b);
    s->data[total] = '\0';
    return s;
}

bool dot_string_eq(DotString* a, DotString* b) {
    if (a == b) return true;
    if (!a || !b) return false;
    if (a->len != b->len) return false;
    return memcmp(a->data, b->data, (size_t)a->len) == 0;
}
