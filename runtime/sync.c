#include "sync.h"
#include <stdlib.h>
#include <stdatomic.h>

dot_sync_chan_t* dot_sync_chan_new(void) {
    dot_sync_chan_t* chan = (dot_sync_chan_t*)malloc(sizeof(dot_sync_chan_t));
    if (!chan) return NULL;
    
    dot_sync_node_t* dummy = (dot_sync_node_t*)malloc(sizeof(dot_sync_node_t));
    if (!dummy) {
        free(chan);
        return NULL;
    }
    dummy->data = NULL;
    atomic_init(&dummy->next, NULL);
    
    atomic_init(&chan->head, dummy);
    atomic_init(&chan->tail, dummy);
    
    return chan;
}

void dot_sync_chan_send(dot_sync_chan_t* chan, void* data) {
    dot_sync_node_t* new_node = (dot_sync_node_t*)malloc(sizeof(dot_sync_node_t));
    if (!new_node) return;
    new_node->data = data;
    atomic_init(&new_node->next, NULL);
    
    dot_sync_node_t* tail;
    while (1) {
        tail = atomic_load(&chan->tail);
        dot_sync_node_t* next = atomic_load(&tail->next);
        
        if (tail == atomic_load(&chan->tail)) {
            if (next == NULL) {
                if (atomic_compare_exchange_weak(&tail->next, &next, new_node)) {
                    break;
                }
            } else {
                atomic_compare_exchange_weak(&chan->tail, &tail, next);
            }
        }
    }
    atomic_compare_exchange_weak(&chan->tail, &tail, new_node);
}

bool dot_sync_chan_recv(dot_sync_chan_t* chan, void** data_out) {
    dot_sync_node_t* head;
    while (1) {
        head = atomic_load(&chan->head);
        dot_sync_node_t* tail = atomic_load(&chan->tail);
        dot_sync_node_t* next = atomic_load(&head->next);
        
        if (head == atomic_load(&chan->head)) {
            if (head == tail) {
                if (next == NULL) {
                    return false;
                }
                atomic_compare_exchange_weak(&chan->tail, &tail, next);
            } else {
                *data_out = next->data;
                if (atomic_compare_exchange_weak(&chan->head, &head, next)) {
                    break;
                }
            }
        }
    }
    free(head);
    return true;
}

void dot_sync_chan_free(dot_sync_chan_t* chan) {
    if (!chan) return;
    void* dummy_data;
    while (dot_sync_chan_recv(chan, &dummy_data)) {
        // Drain queue
    }
    dot_sync_node_t* head = atomic_load(&chan->head);
    if (head) {
        free(head);
    }
    free(chan);
}
void* dot_sync_chan_recv_value(dot_sync_chan_t* chan, bool* ok) {
    void* data;
    if (dot_sync_chan_recv(chan, &data)) {
        if (ok) *ok = true;
        return data;
    }
    if (ok) *ok = false;
    return NULL;
}

#include <stdint.h>
typedef struct {
    int64_t count;
} DotRefcntSyncHack;

#include <stdint.h>
typedef struct {
    DotRefcntSyncHack rc;
    void** data;
    int64_t len;
    int64_t cap;
} DotSliceSyncHack;

DotSliceSyncHack* dot_sync_chan_recv_slice(dot_sync_chan_t* chan) {
    void* data;
    if (dot_sync_chan_recv(chan, &data)) {
        DotSliceSyncHack* s = (DotSliceSyncHack*)calloc(1, sizeof(DotSliceSyncHack));
        s->rc.count = 1;
        s->len = 1;
        s->cap = 1;
        s->data = (void**)calloc(1, sizeof(void*));
        s->data[0] = data;
        return s;
    }
    DotSliceSyncHack* s = (DotSliceSyncHack*)calloc(1, sizeof(DotSliceSyncHack));
    s->rc.count = 1;
    s->len = 0;
    s->cap = 0;
    return s;
}


void* dot_sync_chan_zero(void) {
    return NULL;
}
