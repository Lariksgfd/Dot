#include "closure.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

DotClosure* dot_closure_new(void* fn, DotAny* captures, int32_t n) {
    DotClosure* c = (DotClosure*)calloc(1, sizeof(DotClosure));
    if (!c) {
        fprintf(stderr, "dot_closure_new: out of memory\n");
        abort();
    }
    c->rc.count = 1;
    
    c->fn = fn;
    c->ncaptures = n;
    if (n > 0 && captures) {
        c->captures = (DotAny*)calloc((size_t)n, sizeof(DotAny));
        if (!c->captures) {
            fprintf(stderr, "dot_closure_new: out of memory\n");
            free(c);
            abort();
        }
        memcpy(c->captures, captures, (size_t)n * sizeof(DotAny));
    } else {
        c->captures = NULL;
    }
    return c;
}

typedef DotAny (*ClosureFn)(DotAny* captures, DotAny* args);

DotAny dot_closure_call(DotClosure* c, DotAny* args) {
    ClosureFn fn = (ClosureFn)c->fn;
    return fn(c->captures, args);
}
