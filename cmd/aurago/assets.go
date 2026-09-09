package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/webassets"
)

func assetsNeedFreshRecovery(configFile string) bool {
	if webassets.Default.Ready() {
		return false
	}
	_, err := os.Stat(configFile)
	return os.IsNotExist(err)
}

func handleAssetFlags(installDir, root string, info, check bool, archive, importDir string) bool {
	pin := webassets.ReleasePin()
	if info {
		json.NewEncoder(os.Stdout).Encode(pin)
		return true
	}
	if root == "" {
		// The standard installer keeps executables in <installation>/bin.
		if filepath.Base(installDir) == "bin" {
			installDir = filepath.Dir(installDir)
		}
		root = filepath.Join(installDir, "assets", "web")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	store := webassets.Open(root, pin)
	if importDir != "" {
		if err := store.Import(context.Background(), importDir); err != nil {
			fmt.Fprintln(os.Stderr, "ASSETS ERROR:", err)
			os.Exit(1)
		}
		fmt.Println("Imported asset set", pin.ID)
		return true
	}
	if archive != "" {
		f, err := os.Open(archive)
		if err == nil {
			err = store.Install(context.Background(), f)
			f.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "ASSETS ERROR:", err)
			os.Exit(1)
		}
		fmt.Println("Installed asset set", pin.ID)
		return true
	}
	if check {
		if !store.Ready() {
			fmt.Fprintln(os.Stderr, "ASSETS ERROR:", store.Error)
			os.Exit(1)
		}
		fmt.Println("Verified asset set", pin.ID)
		return true
	}
	webassets.Default = store
	return false
}
