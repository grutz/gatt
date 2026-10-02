package gatt

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/grutz/gatt/constants"
)

// fakeL2CAP is a scripted l2cap connection for exercising the client code
// against a hostile peer.
type fakeL2CAP struct {
	in      chan []byte
	onWrite func(b []byte) [][]byte
	once    sync.Once
}

func newFakeL2CAP(onWrite func(b []byte) [][]byte) *fakeL2CAP {
	return &fakeL2CAP{in: make(chan []byte, 64), onWrite: onWrite}
}

func (f *fakeL2CAP) Read(b []byte) (int, error) {
	r, ok := <-f.in
	if !ok {
		return 0, io.EOF
	}
	return copy(b, r), nil
}

func (f *fakeL2CAP) Write(b []byte) (int, error) {
	if f.onWrite != nil {
		for _, r := range f.onWrite(b) {
			f.in <- r
		}
	}
	return len(b), nil
}

func (f *fakeL2CAP) Close() error {
	f.once.Do(func() { close(f.in) })
	return nil
}

func startPeripheral(f *fakeL2CAP) (*peripheral, chan struct{}) {
	p := &peripheral{
		l2c:   f,
		reqc:  make(chan message),
		quitc: make(chan struct{}),
		sub:   newSubscriber(),
	}
	done := make(chan struct{})
	go func() {
		p.loop()
		close(done)
	}()
	return p, done
}

func within(t *testing.T, what string, f func()) {
	t.Helper()
	c := make(chan struct{})
	go func() { f(); close(c) }()
	select {
	case <-c:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: timed out", what)
	}
}

func TestClientShortErrorResponse(t *testing.T) {
	defer func(d time.Duration) { attTimeout = d }(attTimeout)
	attTimeout = 100 * time.Millisecond

	// A 2-byte "error response" used to index past the end in the dequeue
	// goroutine, which has no recover and killed the process.
	f := newFakeL2CAP(func(b []byte) [][]byte {
		return [][]byte{{constants.AttOpError, b[0]}}
	})
	p, done := startPeripheral(f)
	within(t, "DiscoverServices", func() {
		if _, err := p.DiscoverServices(nil); err == nil {
			t.Error("expected an error")
		}
	})
	f.Close()
	<-done
}

func TestClientShortResponses(t *testing.T) {
	// Responses with the right opcode but no payload.
	for _, op := range []byte{constants.AttOpReadByGroupReq, constants.AttOpReadByTypeReq, constants.AttOpFindInfoReq} {
		op := op
		f := newFakeL2CAP(func(b []byte) [][]byte {
			return [][]byte{{constants.AttRspFor[op]}}
		})
		p, done := startPeripheral(f)
		within(t, "discover", func() {
			switch op {
			case constants.AttOpReadByGroupReq:
				p.DiscoverServices(nil)
			case constants.AttOpReadByTypeReq:
				p.DiscoverCharacteristics(nil, &Service{h: 1, endh: 10})
			case constants.AttOpFindInfoReq:
				p.DiscoverDescriptors(nil, &Characteristic{vh: 1, endh: 10, svc: &Service{}})
			}
		})
		f.Close()
		<-done
	}
}

func TestClientShortNotification(t *testing.T) {
	f := newFakeL2CAP(nil)
	_, done := startPeripheral(f)
	f.in <- []byte{constants.AttOpHandleNotify}
	f.in <- []byte{constants.AttOpHandleInd, 0x01}
	f.Close()
	within(t, "loop exit", func() { <-done })
}

func TestClientNoProgressDiscovery(t *testing.T) {
	// The peer keeps answering with the same group; discovery must give up
	// instead of looping forever.
	f := newFakeL2CAP(func(b []byte) [][]byte {
		return [][]byte{{constants.AttOpReadByGroupRsp, 6, 0x01, 0x00, 0x01, 0x00, 0x0F, 0x18}}
	})
	p, done := startPeripheral(f)
	within(t, "DiscoverServices", func() {
		if _, err := p.DiscoverServices(nil); err != ErrInvalidLength {
			t.Errorf("err = %v, want ErrInvalidLength", err)
		}
	})
	f.Close()
	<-done
}

func TestClientRequestAfterDisconnect(t *testing.T) {
	f := newFakeL2CAP(nil)
	p, done := startPeripheral(f)
	f.Close()
	<-done
	within(t, "ReadCharacteristic", func() {
		if _, err := p.ReadCharacteristic(&Characteristic{vh: 3}); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestClientUnsolicitedResponseDoesNotBlock(t *testing.T) {
	// Unsolicited responses must not wedge the read loop, so a later
	// notification is still delivered.
	f := newFakeL2CAP(nil)
	p, done := startPeripheral(f)
	got := make(chan []byte, 1)
	p.sub.subscribe(0x0003, func(b []byte, err error) { got <- b })
	for i := 0; i < 20; i++ {
		f.in <- []byte{constants.AttOpReadRsp, 0xAA}
	}
	f.in <- []byte{constants.AttOpHandleNotify, 0x03, 0x00, 0x42}
	select {
	case b := <-got:
		if len(b) != 1 || b[0] != 0x42 {
			t.Errorf("notification = % X", b)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification not delivered; read loop is blocked")
	}
	f.Close()
	<-done
}
