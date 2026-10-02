package linux

import (
	"sync"
	"testing"
	"time"

	"github.com/grutz/gatt/constants"
)

func newTestHCI() *HCI {
	return &HCI{
		plist:   make(map[bdaddr]*PlatData),
		plistmu: &sync.Mutex{},
		advsem:  make(chan struct{}, maxAdvHandlers),
	}
}

// advReport builds an LE advertising report event (starting at the subevent
// code) containing a single report.
func advReport(et uint8, addr [6]byte, data []byte) []byte {
	b := []byte{0x02, 0x01, et, 0x00}
	b = append(b, addr[:]...)
	b = append(b, byte(len(data)))
	b = append(b, data...)
	return append(b, 0xC0) // RSSI
}

func TestPlistBounded(t *testing.T) {
	h := newTestHCI()
	h.AdvertisementHandler = func(*PlatData) {}
	for i := 0; i < maxPlist+500; i++ {
		addr := [6]byte{byte(i), byte(i >> 8), 1, 2, 3, 4}
		h.handleAdvertisement(advReport(constants.AdvInd, addr, nil))
	}
	if n := len(h.plist); n != maxPlist {
		t.Errorf("plist has %d entries, want %d", n, maxPlist)
	}
	// The most recent advertiser must still be there.
	n := maxPlist + 499
	last := bdaddr{4, 3, 2, 1, byte(n >> 8), byte(n)} // the parser reverses the address
	if _, ok := h.plist[last]; !ok {
		t.Error("most recent advertiser was evicted")
	}
}

func TestPlistEvictsOldest(t *testing.T) {
	h := newTestHCI()
	now := time.Now()
	for i := 0; i < maxPlist; i++ {
		h.plist[bdaddr{byte(i), byte(i >> 8)}] = &PlatData{seen: now.Add(time.Duration(i) * time.Second)}
	}
	h.storePlistLocked(bdaddr{0xFF, 0xFF}, &PlatData{seen: now})
	if _, ok := h.plist[bdaddr{0, 0}]; ok {
		t.Error("oldest entry was not evicted")
	}
	if len(h.plist) != maxPlist {
		t.Errorf("plist has %d entries, want %d", len(h.plist), maxPlist)
	}
}

func TestScanResponseLabelled(t *testing.T) {
	h := newTestHCI()
	var got []*PlatData
	h.AdvertisementHandler = func(pd *PlatData) { got = append(got, pd) }
	addr := [6]byte{1, 2, 3, 4, 5, 6}
	h.handleAdvertisement(advReport(constants.AdvInd, addr, []byte{0x02, 0x01, 0x06}))
	h.handleAdvertisement(advReport(constants.ScanRsp, addr, []byte{0x03, 0x09, 'a', 'b'}))
	if len(got) != 2 {
		t.Fatalf("got %d reports", len(got))
	}
	if got[1].EventType != constants.ScanRsp {
		t.Errorf("scan response EventType = %v", got[1].EventType)
	}
	if len(got[1].Data) != 7 {
		t.Errorf("scan response data = % X, want adv+rsp data", got[1].Data)
	}
	if got[0].EventType != constants.AdvInd || len(got[0].Data) != 3 || len(h.plist[bdaddr(got[0].Address)].Data) != 3 {
		t.Error("scan response modified the stored advertisement")
	}
}

func TestHandleAdvertisementRecovers(t *testing.T) {
	h := newTestHCI()
	h.AdvertisementHandler = func(*PlatData) { panic("boom") }
	h.handleAdvertisement(advReport(constants.AdvInd, [6]byte{1}, nil)) // must not panic
}

func TestHandleLEMetaCopiesBuffer(t *testing.T) {
	h := newTestHCI()
	done := make(chan *PlatData)
	h.AdvertisementHandler = func(pd *PlatData) { done <- pd }
	// handleAdvertisement parses asynchronously; clobbering the caller's
	// buffer right after handleLEMeta returns must not affect the result.
	b := append([]byte{0x02}, advReport(constants.AdvInd, [6]byte{9, 8, 7, 6, 5, 4}, []byte{0x02, 0x01, 0x06})[1:]...)
	b[0] = 0x02
	if err := h.handleLEMeta(b); err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = 0xFF
	}
	select {
	case pd := <-done:
		if pd.Address != [6]byte{4, 5, 6, 7, 8, 9} {
			t.Errorf("address = % X, buffer was aliased", pd.Address)
		}
	case <-time.After(time.Second):
		t.Fatal("no advertisement delivered")
	}
}

func TestHandleLEMetaEmpty(t *testing.T) {
	if err := newTestHCI().handleLEMeta(nil); err == nil {
		t.Error("expected error for empty event")
	}
}

func TestAdvHandlersBounded(t *testing.T) {
	h := newTestHCI()
	block := make(chan struct{})
	h.AdvertisementHandler = func(*PlatData) { <-block }
	ev := advReport(constants.AdvInd, [6]byte{1}, nil)
	for i := 0; i < maxAdvHandlers*3; i++ {
		h.handleLEMeta(ev)
	}
	time.Sleep(50 * time.Millisecond)
	if n := len(h.advsem); n > maxAdvHandlers {
		t.Errorf("%d handlers running, limit %d", n, maxAdvHandlers)
	}
	close(block)
}
