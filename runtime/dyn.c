#include "dyn.h"
#include <stddef.h>

void* dot_dyn_method(DotDyn* d, int32_t idx) {
    if (!d || !d->vtable) return NULL;
    void** vt = (void**)d->vtable;
    return vt[idx];
}
