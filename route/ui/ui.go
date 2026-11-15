// Package ui holds the single-file console served at /ui.
//
// The page is embedded into the server binary so that a Windows build stays a
// single self-contained executable: no extra files to ship next to it, and no
// network access is required to load the page.
package ui

import _ "embed"

// Index is the console page, served verbatim at /ui.
//
//go:embed index.html
var Index []byte
