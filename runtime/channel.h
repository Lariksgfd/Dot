#ifndef DOT_CHANNEL_H
#define DOT_CHANNEL_H

#include "arc.h"
#include <stdint.h>

typedef struct {
    DotRefcnt rc;
    int64_t   cap;
    DotAny*   buffer;
    int64_t   head;
    int64_t   tail;
    int64_t   count;
} DotChannel;

DotChannel* dot_channel_new(int64_t cap);
void        dot_channel_send(DotChannel* ch, DotAny v);
DotAny      dot_channel_recv(DotChannel* ch);

#endif
