//go:build !windows

package main

import (
	"fmt"
	"os"
)

// The panel is a Windows desktop application (WebView2 + tray); other platforms
// build the stub so `go build ./...` still succeeds.
func main() {
	fmt.Fprintln(os.Stderr, "metatube-panel 只支持 Windows；请直接运行 metatube-server。")
	os.Exit(1)
}
