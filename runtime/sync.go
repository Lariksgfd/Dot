package runtime

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -lws2_32
#include "sync.h"
*/
import "C"
import "unsafe"

// NewChan creates a new channel.
func NewChan() unsafe.Pointer {
	return unsafe.Pointer(C.dot_sync_chan_new())
}

// SendChan sends data to the channel (lock-free).
func SendChan(ch unsafe.Pointer, data unsafe.Pointer) {
	C.dot_sync_chan_send((*C.dot_sync_chan_t)(ch), data)
}

// RecvChan receives data from the channel.
// Returns false if the channel is empty.
func RecvChan(ch unsafe.Pointer) (unsafe.Pointer, bool) {
	var out unsafe.Pointer
	ok := C.dot_sync_chan_recv((*C.dot_sync_chan_t)(ch), &out)
	return out, bool(ok)
}

// FreeChan frees the channel and all pending data.
func FreeChan(ch unsafe.Pointer) {
	C.dot_sync_chan_free((*C.dot_sync_chan_t)(ch))
}
