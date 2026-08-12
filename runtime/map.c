#include "map.h"
#include <stdio.h>
#include <stdlib.h>

#define DOT_MAP_INIT_CAP 16
#define DOT_MAP_MAX_LOAD  0.7
#define DOT_MAP_TOMBSTONE ((uint64_t)1)

static uint64_t hash_ptr(const void* ptr) {
    uint64_t h = 14695981039346656037ULL;
    const unsigned char* p = (const unsigned char*)&ptr;
    for (size_t i = 0; i < sizeof(ptr); i++) {
        h ^= p[i];
        h *= 1099511628211ULL;
    }
    return h;
}

static uint64_t slot_hash(uint64_t h) {
    return h < 2 ? 2 : h;
}

static DotMap* map_alloc(int64_t cap) {
    DotMap* m = (DotMap*)calloc(1, sizeof(DotMap));
    if (!m) {
        fprintf(stderr, "dot_map_new: out of memory\n");
        abort();
    }
    m->rc.count = 1;
    
    m->len = 0;
    m->cap = cap;
    m->entries = (DotMapEntry*)calloc((size_t)cap, sizeof(DotMapEntry));
    if (!m->entries) {
        fprintf(stderr, "dot_map_new: out of memory\n");
        free(m);
        abort();
    }
    return m;
}

static void map_rehash(DotMap* m) {
    int64_t old_cap = m->cap;
    DotMapEntry* old = m->entries;

    m->cap *= 2;
    m->len = 0;
    m->entries = (DotMapEntry*)calloc((size_t)m->cap, sizeof(DotMapEntry));
    if (!m->entries) {
        fprintf(stderr, "dot_map_set: out of memory\n");
        abort();
    }

    for (int64_t i = 0; i < old_cap; i++) {
        uint64_t h = old[i].hash;
        if (h < 2) continue;
        uint64_t idx = h % (uint64_t)m->cap;
        while (m->entries[idx].hash >= 2) {
            idx = (idx + 1) % (uint64_t)m->cap;
        }
        m->entries[idx] = old[i];
        m->len++;
    }

    free(old);
}

DotMap* dot_map_new(void) {
    return map_alloc(DOT_MAP_INIT_CAP);
}

DotAny dot_map_get(DotMap* m, DotAny key) {
    uint64_t h = slot_hash(hash_ptr(key));
    uint64_t idx = h % (uint64_t)m->cap;

    while (m->entries[idx].hash != 0) {
        if (m->entries[idx].hash == h && m->entries[idx].key == key) {
            return m->entries[idx].value;
        }
        idx = (idx + 1) % (uint64_t)m->cap;
    }
    return NULL;
}

void dot_map_set(DotMap* m, DotAny key, DotAny value) {
    if ((double)(m->len + 1) / (double)m->cap > DOT_MAP_MAX_LOAD) {
        map_rehash(m);
    }

    uint64_t h = slot_hash(hash_ptr(key));
    uint64_t idx = h % (uint64_t)m->cap;
    int64_t tomb = -1;

    while (m->entries[idx].hash != 0) {
        if (m->entries[idx].hash == DOT_MAP_TOMBSTONE) {
            if (tomb < 0) tomb = (int64_t)idx;
        } else if (m->entries[idx].hash == h && m->entries[idx].key == key) {
            dot_release((DotRefcnt*)m->entries[idx].value);
            m->entries[idx].value = value;
            if (value) dot_retain((DotRefcnt*)value);
            return;
        }
        idx = (idx + 1) % (uint64_t)m->cap;
    }

    if (tomb >= 0) idx = (uint64_t)tomb;

    m->entries[idx].hash = h;
    m->entries[idx].key = key;
    m->entries[idx].value = value;
    if (key) dot_retain((DotRefcnt*)key);
    if (value) dot_retain((DotRefcnt*)value);
    m->len++;
}

void dot_map_remove(DotMap* m, DotAny key) {
    uint64_t h = slot_hash(hash_ptr(key));
    uint64_t idx = h % (uint64_t)m->cap;

    while (m->entries[idx].hash != 0) {
        if (m->entries[idx].hash == h && m->entries[idx].key == key) {
            dot_release((DotRefcnt*)m->entries[idx].key);
            dot_release((DotRefcnt*)m->entries[idx].value);
            m->entries[idx].hash = DOT_MAP_TOMBSTONE;
            m->entries[idx].key = NULL;
            m->entries[idx].value = NULL;
            m->len--;
            return;
        }
        idx = (idx + 1) % (uint64_t)m->cap;
    }
}
