#include "channel.h"
#include <stdlib.h>
#include <stdio.h>

DotChannel* dot_channel_new(int64_t cap) {
    if (cap <= 0) cap = 1;
    DotChannel* ch = (DotChannel*)calloc(1, sizeof(DotChannel));
    if (!ch) {
        fprintf(stderr, "dot_channel_new: out of memory\n");
        abort();
    }
    ch->rc.count = 1;
    
    ch->cap         = cap;
    ch->buffer      = (DotAny*)calloc((size_t)cap, sizeof(DotAny));
    if (!ch->buffer) {
        fprintf(stderr, "dot_channel_new: out of memory\n");
        free(ch);
        abort();
    }
    ch->head  = 0;
    ch->tail  = 0;
    ch->count = 0;
    return ch;
}

void dot_channel_send(DotChannel* ch, DotAny v) {
    if (ch->count >= ch->cap) {
        fprintf(stderr, "dot_channel_send: channel full (cap=%lld)\n", (long long)ch->cap);
        abort();
    }
    ch->buffer[ch->tail] = v;
    ch->tail = (ch->tail + 1) % ch->cap;
    ch->count++;
}

DotAny dot_channel_recv(DotChannel* ch) {
    if (ch->count <= 0) {
        fprintf(stderr, "dot_channel_recv: channel empty\n");
        abort();
    }
    DotAny v = ch->buffer[ch->head];
    ch->head = (ch->head + 1) % ch->cap;
    ch->count--;
    return v;
}
