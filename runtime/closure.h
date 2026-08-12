#ifndef DOT_CLOSURE_H
#define DOT_CLOSURE_H

#include "arc.h"

typedef struct {
    DotRefcnt rc;
    void* fn;
    DotAny* captures;
    int32_t ncaptures;
} DotClosure;

DotClosure* dot_closure_new(void* fn, DotAny* captures, int32_t n);
DotAny dot_closure_call(DotClosure* c, DotAny* args);

#endif
