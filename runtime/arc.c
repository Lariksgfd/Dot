#include "arc.h"
#include <stdlib.h>

void* dot_retain(void* ptr) {
    if (!ptr) return ptr;
    DotRefcnt* ref = (DotRefcnt*)ptr;
    atomic_fetch_add_explicit(&ref->count, 1, memory_order_relaxed);
    return ptr;
}

void dot_release(void* ptr) {
    if (!ptr) return;
    DotRefcnt* ref = (DotRefcnt*)ptr;
    if (atomic_fetch_sub_explicit(&ref->count, 1, memory_order_acq_rel) == 1) {
        if (ref->dtor) {
            ref->dtor(ptr);
        }
        free(ptr);
    }
}

DotAny dot_alloc(int32_t size) {
    DotRefcnt* obj = (DotRefcnt*)calloc(1, (size_t)size);
    if (!obj) abort();
    atomic_init(&obj->count, 1);
    obj->dtor = NULL;
    return (DotAny)obj;
}
