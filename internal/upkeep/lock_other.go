//go:build !linux

package upkeep

import "os"

func inheritedLock(string) (func(), bool, error) { return nil, false, nil }

// Legacy /tmp updater backups are Linux-only; never adopt them elsewhere.
func sameOwner(a, b os.FileInfo) bool { return false }

func allocatedSize(i os.FileInfo) int64 { return i.Size() }
