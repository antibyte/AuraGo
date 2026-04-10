// Package tools provides utility functions for the AuraGo agent tool system.
//
// This file (utils.go) serves as a collection point for helper functions that are used
// across multiple tool implementations. It intentionally contains mixed responsibilities:
//
//   - Web/Browser utilities (getSharedBrowser): Used by web_capture.go and
//     web_performance.go for headless browser operations via the rod library.
//     These are isolated here because browser setup is complex and benefits from
//     centralized singleton management.
//
//   - Text utilities (truncateUTF8Safe): Used by ansible.go and email.go for
//     safe UTF-8 string truncation without breaking multi-byte characters.
//
// Note: A future refactoring could move rod-specific code to a dedicated web/browser
// package and text utilities to a strings package. This was evaluated but deferred
// because the current structure is functional, the rod dependency is contained,
// and splitting would require updating multiple import sites without significant benefit.
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
