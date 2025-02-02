package gatt

import (
	"testing"

	"github.com/grutz/gatt/constants"
)

// TODO:
func TestAppendField(t *testing.T) {}

// TODO:
func TestAppendFlags(t *testing.T) {}

func TestAppendName(t *testing.T) {
	cases := []struct {
		curr      []byte
		name      string
		wantBytes []byte
		wantLen   int
	}{
		{
			curr:      []byte{},
			name:      "ABCDE",
			wantBytes: []byte{0x06, typeCompleteName, 'A', 'B', 'C', 'D', 'E'},
			wantLen:   7,
		},
		{
			curr:      []byte("111111111122222222223333"),
			name:      "ABCDE",
			wantBytes: append([]byte("111111111122222222223333"), []byte{0x06, typeCompleteName, 'A', 'B', 'C', 'D', 'E'}...),
			wantLen:   31,
		},
		{
			curr:      []byte("1111111111222222222233333"),
			name:      "ABCDE",
			wantBytes: append([]byte("1111111111222222222233333"), []byte{0x05, typeShortName, 'A', 'B', 'C', 'D'}...),
			wantLen:   31,
		},
	}
	for _, tt := range cases {
		a := (&AdvPacket{tt.curr}).AppendName(tt.name)
		wantBytes := [31]byte{}
		copy(wantBytes[:], tt.wantBytes)
		if a.Bytes() != wantBytes {
			t.Errorf("%q a.AppendName(%q) got %x want %x", tt.curr, tt.name, a.Bytes(), tt.wantBytes)
		}
		if a.Len() != tt.wantLen {
			t.Errorf("%q a.AppendName(%q) got %d want %d", tt.curr, tt.name, a.Len(), tt.wantLen)
		}
	}
}

// TODO:
func TestAppendManufacturerData(t *testing.T) {}

// TODO:
func TestAppendUUIDFit(t *testing.T) {
	cases := []struct {
		uu   []constants.UUID
		want string
		fit  []constants.UUID // if different than uu
	}{
		{
			uu:   []constants.UUID{constants.UUID16(0xFAFE)},
			want: "0201060302fefa",
		},
		{
			uu:   []constants.UUID{constants.UUID16(0xFAFE), constants.UUID16(0xFAF9)},
			want: "0201060302fefa0302f9fa",
		},
		{
			uu:   []constants.UUID{constants.MustParseUUID("ABABABABABABABABABABABABABABABAB")},
			want: "0201061106abababababababababababababababab",
		},
		{
			uu: []constants.UUID{
				constants.MustParseUUID("ABABABABABABABABABABABABABABABAB"),
				constants.MustParseUUID("CDCDCDCDCDCDCDCDCDCDCDCDCDCDCDCD"),
			},
			want: "0201061106abababababababababababababababab",
			fit:  []constants.UUID{constants.MustParseUUID("ABABABABABABABABABABABABABABABAB")},
		},
		{
			uu: []constants.UUID{
				constants.UUID16(0xaaaa), constants.UUID16(0xbbbb),
				constants.UUID16(0xcccc), constants.UUID16(0xdddd),
				constants.UUID16(0xeeee), constants.UUID16(0xffff),
				constants.UUID16(0xaaaa), constants.UUID16(0xbbbb),
			},
			want: "0201060302aaaa0302bbbb0302cccc0302dddd0302eeee0302ffff0302aaaa",
			fit: []constants.UUID{
				constants.UUID16(0xaaaa), constants.UUID16(0xbbbb),
				constants.UUID16(0xcccc), constants.UUID16(0xdddd),
				constants.UUID16(0xeeee), constants.UUID16(0xffff),
				constants.UUID16(0xaaaa),
			},
		},
	}

	_ = cases
	// for _, tt := range cases {
	// 	pack, fit := serviceAdvertisingPacket(tt.uu)
	// 	if got := fmt.Sprintf("%x", pack); got != tt.want {
	// 		t.Errorf("serviceAdvertisingPacket(%x) packet: got %q want %q", tt.uu, got, tt.want)
	// 	}
	// 	if tt.fit == nil {
	// 		tt.fit = tt.uu
	// 	}
	// 	if !reflect.DeepEqual(fit, tt.fit) {
	// 		t.Errorf("serviceAdvertisingPacket(%x) fit: got %x want %x", tt.uu, fit, tt.fit)
	// 	}
	// }
}
