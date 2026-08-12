#include "future.h"
#include <stdlib.h>
#include <stdio.h>

DotFuture* dot_future_new(void) {
    DotFuture* f = (DotFuture*)calloc(1, sizeof(DotFuture));
    if (!f) {
        fprintf(stderr, "dot_future_new: out of memory\n");
        abort();
    }
    f->rc.count = 1;
    
    f->ready       = 0;
    f->result      = NULL;
    return f;
}

void dot_future_resolve(DotFuture* f, DotAny v) {
    f->result = v;
    f->ready  = 1;
}

DotAny dot_future_get(DotFuture* f) {
    if (!f->ready) {
        fprintf(stderr, "dot_future_get: future not ready\n");
        abort();
    }
    return f->result;
}

typedef DotAny (*SpawnFn)(DotAny ctx);

DotFuture* dot_spawn(void* fn, DotAny ctx) {
    DotFuture* f = dot_future_new();
    SpawnFn func = (SpawnFn)fn;
    DotAny result = func(ctx);
    dot_future_resolve(f, result);
    return f;
}
