package iconres

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestSizes(t *testing.T) {
	want := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	if got := Sizes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Sizes() = %v, want %v", got, want)
	}
}

func TestDIB(t *testing.T) {
	for _, size := range Sizes() {
		dib, err := DIB(size)
		if err != nil {
			t.Fatalf("DIB(%d): %v", size, err)
		}
		if len(dib) < 40 {
			t.Fatalf("DIB(%d) too short: %d", size, len(dib))
		}
		if got := binary.LittleEndian.Uint32(dib[0:4]); got != 40 {
			t.Errorf("DIB(%d) header size = %d", size, got)
		}
		if got := int(int32(binary.LittleEndian.Uint32(dib[4:8]))); got != size {
			t.Errorf("DIB(%d) width = %d", size, got)
		}
		if got := int(int32(binary.LittleEndian.Uint32(dib[8:12]))); got != size*2 {
			t.Errorf("DIB(%d) height = %d, want %d (XOR + AND)", size, got, size*2)
		}
		if got := binary.LittleEndian.Uint16(dib[14:16]); got != 32 {
			t.Errorf("DIB(%d) bpp = %d", size, got)
		}
		// XOR bitmap + AND mask must fit inside the entry.
		row := ((size + 31) / 32) * 4
		wantLen := 40 + size*size*4 + row*size
		if len(dib) != wantLen {
			t.Errorf("DIB(%d) length = %d, want %d", size, len(dib), wantLen)
		}
	}
}

func TestDIBPrefersLargerEntry(t *testing.T) {
	for _, tc := range []struct{ want, size int }{
		{17, 20}, {33, 40}, {100, 128}, {999, 256},
	} {
		dib, err := DIB(tc.want)
		if err != nil {
			t.Fatalf("DIB(%d): %v", tc.want, err)
		}
		if got := int(int32(binary.LittleEndian.Uint32(dib[4:8]))); got != tc.size {
			t.Errorf("DIB(%d) picked %d, want %d", tc.want, got, tc.size)
		}
	}
}
