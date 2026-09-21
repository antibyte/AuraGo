//go:build !linux

package rtlsdr

func Devices() []Device { return []Device{} }
