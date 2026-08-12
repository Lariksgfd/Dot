#include "sync.h"
#include <stdlib.h>
#include <stdio.h>

#ifdef _WIN32

DotMutex* dot_mutex_new(void) {
    DotMutex* mu = (DotMutex*)calloc(1, sizeof(DotMutex));
    if (!mu) {
        fprintf(stderr, "dot_mutex_new: out of memory\n");
        abort();
    }
    InitializeCriticalSection(&mu->cs);
    return mu;
}

void dot_mutex_init(DotMutex* mu) {
    InitializeCriticalSection(&mu->cs);
}

void dot_mutex_lock(DotMutex* mu) {
    EnterCriticalSection(&mu->cs);
}

void dot_mutex_unlock(DotMutex* mu) {
    LeaveCriticalSection(&mu->cs);
}

void dot_mutex_destroy(DotMutex* mu) {
    DeleteCriticalSection(&mu->cs);
}

DotAtomicI32* dot_atomic_new(void) {
    DotAtomicI32* a = (DotAtomicI32*)calloc(1, sizeof(DotAtomicI32));
    if (!a) {
        fprintf(stderr, "dot_atomic_new: out of memory\n");
        abort();
    }
    a->value = 0;
    return a;
}

int32_t dot_atomic_load(DotAtomicI32* a) {
    return (int32_t)InterlockedCompareExchange((volatile LONG*)&a->value, 0, 0);
}

void dot_atomic_store(DotAtomicI32* a, int32_t v) {
    InterlockedExchange((volatile LONG*)&a->value, (LONG)v);
}

int32_t dot_atomic_fetch_add(DotAtomicI32* a, int32_t delta) {
    return (int32_t)InterlockedExchangeAdd((volatile LONG*)&a->value, (LONG)delta);
}

int32_t dot_atomic_fetch_sub(DotAtomicI32* a, int32_t delta) {
    return (int32_t)InterlockedExchangeAdd((volatile LONG*)&a->value, -(LONG)delta);
}

int32_t dot_atomic_exchange(DotAtomicI32* a, int32_t new_val) {
    return (int32_t)InterlockedExchange((volatile LONG*)&a->value, (LONG)new_val);
}

int dot_atomic_compare_exchange(DotAtomicI32* a, int32_t expected, int32_t desired) {
    return (int32_t)InterlockedCompareExchange((volatile LONG*)&a->value, (LONG)desired, (LONG)expected) == expected;
}

void dot_mutex_free(DotMutex* mu) {
    DeleteCriticalSection(&mu->cs);
    free(mu);
}

void dot_atomic_free(DotAtomicI32* a) {
    free(a);
}

#else

DotMutex* dot_mutex_new(void) {
    DotMutex* mu = (DotMutex*)calloc(1, sizeof(DotMutex));
    if (!mu) {
        fprintf(stderr, "dot_mutex_new: out of memory\n");
        abort();
    }
    if (pthread_mutex_init(&mu->m, NULL) != 0) {
        fprintf(stderr, "dot_mutex_new: pthread_mutex_init failed\n");
        free(mu);
        abort();
    }
    return mu;
}

void dot_mutex_init(DotMutex* mu) {
    if (pthread_mutex_init(&mu->m, NULL) != 0) {
        fprintf(stderr, "dot_mutex_init: pthread_mutex_init failed\n");
        abort();
    }
}

void dot_mutex_lock(DotMutex* mu) {
    if (pthread_mutex_lock(&mu->m) != 0) {
        fprintf(stderr, "dot_mutex_lock: pthread_mutex_lock failed\n");
        abort();
    }
}

void dot_mutex_unlock(DotMutex* mu) {
    if (pthread_mutex_unlock(&mu->m) != 0) {
        fprintf(stderr, "dot_mutex_unlock: pthread_mutex_unlock failed\n");
        abort();
    }
}

void dot_mutex_destroy(DotMutex* mu) {
    if (pthread_mutex_destroy(&mu->m) != 0) {
        fprintf(stderr, "dot_mutex_destroy: pthread_mutex_destroy failed\n");
        abort();
    }
}

void dot_mutex_free(DotMutex* mu) {
    dot_mutex_destroy(mu);
    free(mu);
}

DotAtomicI32* dot_atomic_new(void) {
    DotAtomicI32* a = (DotAtomicI32*)calloc(1, sizeof(DotAtomicI32));
    if (!a) {
        fprintf(stderr, "dot_atomic_new: out of memory\n");
        abort();
    }
    a->value = 0;
    return a;
}

void dot_atomic_free(DotAtomicI32* a) {
    free(a);
}

int32_t dot_atomic_load(DotAtomicI32* a) {
    int32_t r;
    __atomic_load(&a->value, &r, __ATOMIC_SEQ_CST);
    return r;
}

void dot_atomic_store(DotAtomicI32* a, int32_t v) {
    __atomic_store(&a->value, &v, __ATOMIC_SEQ_CST);
}

int32_t dot_atomic_fetch_add(DotAtomicI32* a, int32_t delta) {
    return __atomic_fetch_add(&a->value, delta, __ATOMIC_SEQ_CST);
}

int32_t dot_atomic_fetch_sub(DotAtomicI32* a, int32_t delta) {
    return __atomic_fetch_sub(&a->value, delta, __ATOMIC_SEQ_CST);
}

int32_t dot_atomic_exchange(DotAtomicI32* a, int32_t new_val) {
    int32_t r;
    __atomic_exchange(&a->value, &new_val, &r, __ATOMIC_SEQ_CST);
    return r;
}

int dot_atomic_compare_exchange(DotAtomicI32* a, int32_t expected, int32_t desired) {
    return __atomic_compare_exchange_n(&a->value, &expected, desired, 1,
                                        __ATOMIC_SEQ_CST, __ATOMIC_SEQ_CST);
}

#endif
