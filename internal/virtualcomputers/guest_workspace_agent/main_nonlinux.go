//go:build !linux

package main

import "log"

func main() {
	log.Fatal("aurago-workspace-agent requires Linux with AF_VSOCK support")
}
