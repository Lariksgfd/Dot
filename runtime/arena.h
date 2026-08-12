#ifndef DOT_ARENA_H
#define DOT_ARENA_H

#include <stdint.h>

typedef struct {
    char* buffer;
    int64_t len;
    int64_t cap;
} DotArena;

typedef struct {
    int64_t allocated;
    int64_t used;
    int64_t cap;
} DotArenaStats;

DotArena* dot_arena_new(int64_t initial_size);
void* dot_arena_alloc(DotArena* a, int64_t size);
void dot_arena_free(DotArena* a);
void dot_arena_destroy(DotArena* a);
DotArenaStats dot_arena_stats(DotArena* a);
void dot_arena_reset(DotArena* a);

#endif
