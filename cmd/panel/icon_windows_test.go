//go:build windows

package main

import "testing"

// TestCreateAppIcon verifies that Windows accepts the DIB payload produced by
// iconres for every size the panel asks for. A malformed header would make
// CreateIconFromResourceEx return 0 and leave the window/tray iconless.
func TestCreateAppIcon(t *testing.T) {
	for _, size := range []int{16, 32, 48} {
		h := createAppIcon(size)
		if h == 0 {
			t.Fatalf("createAppIcon(%d) returned a null HICON", size)
		}
		procDestroyIcon.Call(uintptr(h))
	}
}
