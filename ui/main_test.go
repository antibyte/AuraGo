package ui

import (
	"os"
	"testing"
)

// TestMain runs the package's tests unchanged and then closes the Chrome that the EasyDrag browser
// tests share (sharedEasyDragBrowser). It does nothing when no test started that browser.
func TestMain(m *testing.M) {
	code := m.Run()
	closeSharedEasyDragBrowser()
	os.Exit(code)
}
