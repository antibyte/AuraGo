//go:build linux || darwin

package main

import "os"

func secureConfigDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return os.Chmod(path, 0700)
}

func runServiceClient(client *Client) error { client.Run(); return nil }
