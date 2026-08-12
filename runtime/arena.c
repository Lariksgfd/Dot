#include "arena.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

DotArena* dot_arena_new(int64_t initial_size) {
    if (initial_size <= 0) {
        initial_size = 1024;
    }
    DotArena* a = (DotArena*)malloc(sizeof(DotArena));
    if (!a) {
        fprintf(stderr, "dot_arena_new: out of memory\n");
        abort();
    }
    a->buffer = (char*)malloc((size_t)initial_size);
    if (!a->buffer) {
        fprintf(stderr, "dot_arena_new: out of memory\n");
        free(a);
        abort();
    }
    a->len = 0;
    a->cap = initial_size;
    return a;
}

void* dot_arena_alloc(DotArena* a, int64_t size) {
    if (a->len + size > a->cap) {
        int64_t new_cap = a->cap * 2;
        while (new_cap < a->len + size) {
            new_cap *= 2;
        }
        char* new_buffer = (char*)realloc(a->buffer, (size_t)new_cap);
        if (!new_buffer) {
            fprintf(stderr, "dot_arena_alloc: out of memory\n");
            abort();
        }
        a->buffer = new_buffer;
        a->cap = new_cap;
    }
    void* ptr = a->buffer + a->len;
    a->len += size;
    return ptr;
}

void dot_arena_free(DotArena* a) {
    if (a) {
        free(a->buffer);
        free(a);
    }
}

void dot_arena_destroy(DotArena* a) {
    if (a) {
        free(a->buffer);
        a->buffer = NULL;
        a->len = 0;
        a->cap = 0;
        free(a);
    }
}

DotArenaStats dot_arena_stats(DotArena* a) {
    DotArenaStats s;
    s.allocated = a->cap;
    s.used = a->len;
    s.cap = a->cap;
    return s;
}

void dot_arena_reset(DotArena* a) {
    a->len = 0;
}
