#ifndef DOT_FUTURE_H
#define DOT_FUTURE_H

#include "arc.h"
#include <stdint.h>

typedef struct {
    DotRefcnt rc;
    DotAny    result;
    int8_t    ready;
} DotFuture;

DotFuture* dot_future_new(void);
void       dot_future_resolve(DotFuture* f, DotAny v);
DotAny     dot_future_get(DotFuture* f);
DotFuture* dot_spawn(void* fn, DotAny ctx);

#endif
