#ifndef DOT_ARC_H
#define DOT_ARC_H

#include <stdint.h>
#include <stdatomic.h>

typedef void* DotAny;

typedef struct {
    _Atomic int32_t count;
    void (*dtor)(void*);
} DotRefcnt;

void* dot_retain(void* ptr);
void dot_release(void* ptr);
DotAny dot_alloc(int32_t size);

#endif // DOT_ARC_H
