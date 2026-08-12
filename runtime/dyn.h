#ifndef DOT_DYN_H
#define DOT_DYN_H

#include "arc.h"

typedef struct {
    void* data;
    void* vtable;
} DotDyn;

void* dot_dyn_method(DotDyn* d, int32_t idx);

#endif
