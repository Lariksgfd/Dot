#include "weak.h"
#include <stdlib.h>
#include <stdio.h>

DotWeak* dot_weak_new(DotAny target) {
    DotWeak* w = (DotWeak*)calloc(1, sizeof(DotWeak));
    if (!w) {
        fprintf(stderr, "dot_weak_new: out of memory\n");
        abort();
    }
    w->rc.count = 1;
    
    w->target      = (DotRefcnt*)target;
    return w;
}

DotAny dot_weak_upgrade(DotWeak* w) {
    if (!w->target) return NULL;
    if (w->target->count <= 0) return NULL;
    dot_retain(w->target); return w->target;
}
