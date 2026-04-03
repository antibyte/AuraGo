package tools

import (
	"sync"
	"unicode/utf8"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var (
	sharedBrowser     *rod.Browser
	sharedBrowserOnce sync.Once
	sharedBrowserErr  error
)

// getSharedBrowser returns a singleton rod.Browser instance.
// It initializes the browser lazily and handles the launcher configuration.
func getSharedBrowser() (*rod.Browser, error) {
	sharedBrowserOnce.Do(func() {
		u, err := launcher.New().
			Headless(true).
			NoSandbox(true).
			Launch()
		if err != nil {
			sharedBrowserErr = err
			return
		}

		browser := rod.New().ControlURL(u)
		if err := browser.Connect(); err != nil {
			sharedBrowserErr = err
			return
		}

		sharedBrowser = browser
	})

	return sharedBrowser, sharedBrowserErr
}

// truncateUTF8Safe truncates a string to a given maximum byte length
// without breaking UTF-8 multi-byte characters.
func truncateUTF8Safe(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// We know it exceeds maxBytes. We just iterate backwards from maxBytes
	// to find a valid utf8 boundary.
	for i := maxBytes; i >= 0; i-- {
		if utf8.RuneStart(s[i]) {
			// If we stop at a rune start, we just need to ensure the rune fits
			// inside maxBytes.
			// Actually, if we just slice up to i, it cleanly lops off the
			// overflowing rune.
			return s[:i]
		}
	}

	return ""
}
