#ifndef DOT_SYNC_H
#define DOT_SYNC_H

#include <stdint.h>

#ifdef _WIN32
#include <windows.h>
typedef struct {
    CRITICAL_SECTION cs;
} DotMutex;
#else
#include <pthread.h>
typedef struct {
    pthread_mutex_t m;
} DotMutex;
#endif

typedef struct {
    volatile int32_t value;
} DotAtomicI32;

DotMutex*    dot_mutex_new(void);
void         dot_mutex_init(DotMutex* mu);
void         dot_mutex_lock(DotMutex* mu);
void         dot_mutex_unlock(DotMutex* mu);
void         dot_mutex_destroy(DotMutex* mu);
void         dot_mutex_free(DotMutex* mu);

DotAtomicI32* dot_atomic_new(void);
void          dot_atomic_free(DotAtomicI32* a);

int32_t       dot_atomic_load(DotAtomicI32* a);
void          dot_atomic_store(DotAtomicI32* a, int32_t v);
int32_t       dot_atomic_fetch_add(DotAtomicI32* a, int32_t delta);
int32_t       dot_atomic_fetch_sub(DotAtomicI32* a, int32_t delta);
int32_t       dot_atomic_exchange(DotAtomicI32* a, int32_t new_val);
int           dot_atomic_compare_exchange(DotAtomicI32* a, int32_t expected, int32_t desired);

#endif
