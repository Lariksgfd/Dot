#ifndef DOT_CHAN_H
#define DOT_CHAN_H

#include "sync.h"
#include "arc.h"
#include <stdint.h>

typedef struct {
    DotRefcnt rc;
    int64_t   cap;
    DotAny*   buffer;
    int64_t   head;
    int64_t   tail;
    int64_t   count;
    DotMutex  mu;
} DotChan;

DotChan* dot_chan_new(int64_t cap);
void     dot_chan_send(DotChan* ch, DotAny v);
DotAny   dot_chan_recv(DotChan* ch);

#endif
