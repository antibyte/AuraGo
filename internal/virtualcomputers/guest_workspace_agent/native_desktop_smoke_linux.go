//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// This is used only by the image builder's disposable VM, never by status.
func nativeDesktopSmoke(ctx context.Context) error {
	call := func(args []string, input string) (desktopEnvelope, error) {
		r, err := runNativeDesktop(ctx, args, input)
		if err == nil && !r.OK {
			err = fmt.Errorf("native image probe %s failed: %s", args[0], r.Error)
		}
		return r, err
	}
	if _, err := call([]string{"status"}, ""); err != nil {
		return err
	}
	if _, err := call([]string{"launch", "mousepad"}, ""); err != nil {
		return err
	}
	var ref string
	for attempt := 0; attempt < 40; attempt++ {
		r, err := call([]string{"find", "--app", "mousepad", "--role", "textfield", "--limit", "2", "--timeout-ms", "1000"}, "")
		if err == nil {
			var found struct {
				Matches []struct {
					Ref string `json:"ref_id"`
				} `json:"matches"`
			}
			if json.Unmarshal(r.Data, &found) == nil && len(found.Matches) == 1 {
				ref = found.Matches[0].Ref
				break
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if ref == "" {
		return fmt.Errorf("Mousepad accessibility text field is absent or ambiguous")
	}
	const marker = "AuraGo native desktop image verification"
	if _, err := call([]string{"type", ref, "--stdin", "--timeout-ms", "5000"}, marker); err != nil {
		return err
	}
	r, err := call([]string{"snapshot", "--app", "mousepad", "--compact", "--timeout-ms", "5000"}, "")
	if err != nil {
		return err
	}
	if !strings.Contains(string(r.Data), marker) {
		return fmt.Errorf("Mousepad snapshot did not confirm typed text")
	}
	var snapshot struct {
		Window struct {
			ID string `json:"id"`
		} `json:"window"`
	}
	if err = json.Unmarshal(r.Data, &snapshot); err != nil || snapshot.Window.ID == "" {
		return fmt.Errorf("Mousepad window is missing")
	}
	if _, err = call([]string{"--headed", "press", "Ctrl+s", "--window-id", snapshot.Window.ID}, ""); err != nil {
		return err
	}
	windows, err := call([]string{"list-windows", "--app", "mousepad", "--timeout-ms", "5000"}, "")
	if err != nil {
		return err
	}
	var candidates []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err = json.Unmarshal(windows.Data, &candidates); err != nil {
		return err
	}
	saveWindow := ""
	for _, window := range candidates {
		if window.Title == "Save As" {
			if saveWindow != "" {
				return fmt.Errorf("native save dialog is ambiguous")
			}
			saveWindow = window.ID
		}
	}
	if saveWindow == "" {
		return fmt.Errorf("native save dialog is missing")
	}
	find := func(role, name string) (string, error) {
		result, err := call([]string{"find", "--window-id", saveWindow, "--role", role, "--name", name, "--exact", "--timeout-ms", "5000"}, "")
		if err != nil {
			return "", err
		}
		var matches struct {
			Matches []struct {
				Ref string `json:"ref_id"`
			} `json:"matches"`
		}
		if json.Unmarshal(result.Data, &matches) != nil || len(matches.Matches) != 1 {
			return "", fmt.Errorf("native save dialog control %s is absent or ambiguous", name)
		}
		return matches.Matches[0].Ref, nil
	}
	nameRef, err := find("textfield", "Name:")
	if err != nil {
		return err
	}
	const path = "/workspace/.native-desktop-image-probe.txt"
	if _, err = call([]string{"type", nameRef, "--stdin"}, path); err != nil {
		return err
	}
	saveRef, err := find("button", "Save")
	if err != nil {
		return err
	}
	if _, err = call([]string{"click", saveRef}, ""); err != nil {
		return err
	}
	for attempt := 0; attempt < 20; attempt++ {
		data, readErr := os.ReadFile(path)
		if readErr == nil && strings.TrimSpace(string(data)) == marker {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("native save dialog did not produce the expected file")
}
