package runtime

import (
	"testing"
	"unsafe"
)

func TestChanNewAndFree(t *testing.T) {
	ch := NewChan()
	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	FreeChan(ch)
}

func TestChanSendRecv(t *testing.T) {
	ch := NewChan()
	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	defer FreeChan(ch)

	val := 42
	SendChan(ch, unsafe.Pointer(&val))

	out, ok := RecvChan(ch)
	if !ok {
		t.Fatal("expected recv to succeed")
	}
	if out == nil || *(*int)(out) != val {
		t.Fatalf("expected %d, got %v", val, out)
	}
}

func TestChanRecvEmpty(t *testing.T) {
	ch := NewChan()
	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	defer FreeChan(ch)

	if _, ok := RecvChan(ch); ok {
		t.Fatal("expected recv to fail on empty channel")
	}
}
