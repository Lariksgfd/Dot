#ifndef DOT_MAP_H
#define DOT_MAP_H

#include "arc.h"

typedef struct {
    uint64_t hash;
    DotAny key;
    DotAny value;
} DotMapEntry;

typedef struct DotMap {
    DotRefcnt rc;
    int64_t len;
    int64_t cap;
    DotMapEntry* entries;
} DotMap;

DotMap* dot_map_new(void);
DotAny dot_map_get(DotMap* m, DotAny key);
void dot_map_set(DotMap* m, DotAny key, DotAny value);
void dot_map_remove(DotMap* m, DotAny key);

#endif
