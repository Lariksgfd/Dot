#include "chan.h"
#include <stdlib.h>
#include <stdio.h>

#ifdef _WIN32
#include <windows.h>
#else
#include <time.h>
#include <sched.h>
#endif

static void chan_yield(void) {
#ifdef _WIN32
    Sleep(0);
#else
    sched_yield();
#endif
}

DotChan* dot_chan_new(int64_t cap) {
    if (cap <= 0) cap = 1;
    DotChan* ch = (DotChan*)calloc(1, sizeof(DotChan));
    if (!ch) {
        fprintf(stderr, "dot_chan_new: out of memory\n");
        abort();
    }
    ch->rc.count = 1;
    
    ch->cap = cap;
    ch->buffer = (DotAny*)calloc((size_t)cap, sizeof(DotAny));
    if (!ch->buffer) {
        fprintf(stderr, "dot_chan_new: out of memory\n");
        free(ch);
        abort();
    }
    ch->head = 0;
    ch->tail = 0;
    ch->count = 0;
    dot_mutex_init(&ch->mu);
    return ch;
}

void dot_chan_send(DotChan* ch, DotAny v) {
    while (1) {
        dot_mutex_lock(&ch->mu);
        if (ch->count < ch->cap) {
            ch->buffer[ch->tail] = v;
            ch->tail = (ch->tail + 1) % ch->cap;
            ch->count++;
            dot_mutex_unlock(&ch->mu);
            return;
        }
        dot_mutex_unlock(&ch->mu);
        chan_yield();
    }
}

DotAny dot_chan_recv(DotChan* ch) {
    while (1) {
        dot_mutex_lock(&ch->mu);
        if (ch->count > 0) {
            DotAny v = ch->buffer[ch->head];
            ch->head = (ch->head + 1) % ch->cap;
            ch->count--;
            dot_mutex_unlock(&ch->mu);
            return v;
        }
        dot_mutex_unlock(&ch->mu);
        chan_yield();
    }
}
