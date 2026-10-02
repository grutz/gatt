package linux

import (
	"bytes"
	"testing"
	"time"
)

func acl(flags uint8, b ...byte) *aclData { return &aclData{flags: flags, b: b} }

func TestReassemblerSingle(t *testing.T) {
	var r reassembler
	cid, sdu, ok := r.push(acl(0x2, 0x03, 0x00, 0x04, 0x00, 1, 2, 3))
	if !ok || cid != 4 || !bytes.Equal(sdu, []byte{1, 2, 3}) {
		t.Fatalf("got cid=%d sdu=%v ok=%v", cid, sdu, ok)
	}
}

func TestReassemblerFragments(t *testing.T) {
	var r reassembler
	if _, _, ok := r.push(acl(0x2, 0x04, 0x00, 0x04, 0x00, 1, 2)); ok {
		t.Fatal("completed early")
	}
	_, sdu, ok := r.push(acl(0x1, 3, 4))
	if !ok || !bytes.Equal(sdu, []byte{1, 2, 3, 4}) {
		t.Fatalf("got %v ok=%v", sdu, ok)
	}
}

func TestReassemblerMalformed(t *testing.T) {
	cases := map[string][]*aclData{
		"short header":           {acl(0x2, 0x01, 0x00)},
		"orphan continuation":    {acl(0x1, 1, 2, 3)},
		"oversized length":       {acl(0x2, 0xFF, 0xFF, 0x04, 0x00, 1)},
		"data longer than tlen":  {acl(0x2, 0x01, 0x00, 0x04, 0x00, 1, 2, 3)},
		"overshooting fragments": {acl(0x2, 0x02, 0x00, 0x04, 0x00, 1), acl(0x1, 2, 3, 4)},
		"restart drops partial":  {acl(0x2, 0x04, 0x00, 0x04, 0x00, 1), acl(0x2, 0x01, 0x00)},
	}
	for name, seq := range cases {
		var r reassembler
		for _, a := range seq {
			if _, sdu, ok := r.push(a); ok {
				t.Errorf("%s: unexpected SDU %v", name, sdu)
			}
		}
	}
}

// A bad frame must not stop conn.loop from draining aclc, otherwise the HCI
// reader (which sends on aclc) would block forever.
func TestConnLoopKeepsDraining(t *testing.T) {
	c := &conn{aclc: make(chan *aclData), datac: make(chan []byte, 1)}
	go c.loop()

	send := func(a *aclData) {
		select {
		case c.aclc <- a:
		case <-time.After(time.Second):
			t.Fatal("conn.loop stopped draining aclc")
		}
	}
	send(acl(0x2, 0x01))                      // short, corrupt
	send(acl(0x2, 0xFF, 0xFF, 0x04, 0x00))    // oversized
	send(acl(0x2, 0x04, 0x00, 0x04, 0x00, 1)) // partial...
	send(acl(0x2, 0x01, 0x00, 0x04, 0x00, 9)) // ...replaced by a good one
	for i := 0; i < 100; i++ {                // slow reader must not block either
		send(acl(0x2, 0x01, 0x00, 0x04, 0x00, byte(i)))
	}
	select {
	case d := <-c.datac:
		if !bytes.Equal(d, []byte{9}) {
			t.Errorf("first SDU = %v, want [9]", d)
		}
	case <-time.After(time.Second):
		t.Fatal("no SDU delivered")
	}
	close(c.aclc)
}
