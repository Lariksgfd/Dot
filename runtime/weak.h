#ifndef DOT_WEAK_H
#define DOT_WEAK_H

#include "arc.h"

typedef struct {
    DotRefcnt  rc;
    DotRefcnt* target;
} DotWeak;

DotWeak* dot_weak_new(DotAny target);
DotAny   dot_weak_upgrade(DotWeak* w);

#endif
