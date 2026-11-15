// Package iconres holds the MetaTube application icon shared by the Windows
// binaries. The same ICO is compiled into both executables as a native resource
// (see cmd/*/icon_windows_amd64.syso) and embedded here so the panel can also
// use it for its window and tray icons at runtime.
package iconres

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
)

//go:embed metatube.ico
var ico []byte

// DIB returns the RT_ICON payload of the entry whose size best matches want.
// Entries are 32bpp BMP/DIB (BITMAPINFOHEADER + BGRA pixels + AND mask), which
// is exactly what CreateIconFromResourceEx expects. Prefer a larger entry so
// Windows can downscale rather than upscale.
func DIB(want int) ([]byte, error) {
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		return nil, errors.New("iconres: invalid ICO header")
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if count == 0 || 6+16*count > len(ico) {
		return nil, errors.New("iconres: truncated ICO directory")
	}

	best, bestScore := -1, int(^uint(0)>>1)
	for i := 0; i < count; i++ {
		off := 6 + 16*i
		w := int(ico[off])
		if w == 0 {
			w = 256 // ICO encodes 256 as 0
		}
		score := w - want
		if score < 0 {
			score = -score + 1000 // penalise upscaling
		}
		if score < bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		return nil, errors.New("iconres: no icon entry")
	}

	ent := 6 + 16*best
	length := int(binary.LittleEndian.Uint32(ico[ent+8 : ent+12]))
	start := int(binary.LittleEndian.Uint32(ico[ent+12 : ent+16]))
	if length <= 40 || start < 0 || start+length > len(ico) {
		return nil, errors.New("iconres: icon entry out of range")
	}
	if binary.LittleEndian.Uint32(ico[start:start+4]) != 40 {
		return nil, fmt.Errorf("iconres: entry %d is not a 32bpp DIB", best)
	}
	return ico[start : start+length], nil
}

// Sizes lists the embedded icon sizes, smallest first.
func Sizes() []int {
	if len(ico) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if 6+16*count > len(ico) {
		return nil
	}
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		w := int(ico[6+16*i])
		if w == 0 {
			w = 256
		}
		out = append(out, w)
	}
	return out
}
