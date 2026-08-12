#ifndef DOT_RUNTIME_SYNC_H
#define DOT_RUNTIME_SYNC_H

#include <stddef.h>
#include <stdbool.h>

// Lock-free queue node
typedef struct dot_sync_node {
    void* data;
    _Atomic(struct dot_sync_node*) next;
} dot_sync_node_t;

// Lock-free queue (channel)
typedef struct {
    _Atomic(dot_sync_node_t*) head;
    _Atomic(dot_sync_node_t*) tail;
} dot_sync_chan_t;

// Initialize a new channel
dot_sync_chan_t* dot_sync_chan_new(void);

// Send data to the channel (lock-free)
void dot_sync_chan_send(dot_sync_chan_t* chan, void* data);

// Receive data from the channel (lock-free)
// Returns true if data was received, false if empty
bool dot_sync_chan_recv(dot_sync_chan_t* chan, void** data_out);

// Free the channel
void dot_sync_chan_free(dot_sync_chan_t* chan);

#endif // DOT_RUNTIME_SYNC_H