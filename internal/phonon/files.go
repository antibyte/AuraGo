package phonon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
)

// Root is an operator-controlled resource location, independent of writable data volumes.
func Root() string {
	if root := strings.TrimSpace(os.Getenv("AURAGO_PHONON_DIR")); root != "" {
		return filepath.Join(root, BundleID())
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "phonon", BundleID())
}

func Binary(root string) string {
	name := "ptts"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(root, "runtime", runtime.GOOS+"_"+runtime.GOARCH, name)
}

var verifiedFiles sync.Map

// Keep the immutable model off the per-utterance hashing path; changed files are rechecked.
func verifyFile(path string, expected File) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != expected.Size {
		return errors.New("Phonon resource is missing or has an invalid size")
	}
	stamp := fmt.Sprintf("%s:%d:%d:%s", path, info.Size(), info.ModTime().UnixNano(), expected.SHA256)
	if _, ok := verifiedFiles.Load(stamp); ok {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Phonon resource: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, io.LimitReader(file, expected.Size+1))
	if err != nil || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
		return errors.New("Phonon resource checksum mismatch")
	}
	verifiedFiles.Store(stamp, true)
	return nil
}

func Ready(root string) error {
	if root == "" {
		return errors.New("Phonon resources are not installed")
	}
	var runtimeFile File
	for _, candidate := range manifest.Runtimes {
		if candidate.Target == runtime.GOOS+"_"+runtime.GOARCH {
			runtimeFile = File{Size: candidate.BinarySize, SHA256: candidate.BinarySHA256}
		}
	}
	if runtimeFile.SHA256 == "" || verifyFile(Binary(root), runtimeFile) != nil {
		return errors.New("Phonon runtime is unavailable on this platform")
	}
	for _, file := range manifest.ModelFiles {
		if err := verifyFile(filepath.Join(root, "model", file.Name), file); err != nil {
			return err
		}
	}
	return nil
}

func VoicePath(root, dataDir, id string) (string, error) {
	voice, ok := Lookup(id)
	if !ok {
		return "", errors.New("unknown Phonon voice")
	}
	expected := File{Size: voice.Size, SHA256: voice.SHA256}
	paths := []string{filepath.Join(dataDir, "phonon", "voices", id+".safetensors"), filepath.Join(root, filepath.FromSlash(voice.Filename))}
	for _, path := range paths {
		if verifyFile(path, expected) == nil {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("Phonon voice is not installed")
}

func Voices(root, dataDir string) []Voice {
	voices := append([]Voice(nil), manifest.Voices...)
	for i := range voices {
		_, err := VoicePath(root, dataDir, voices[i].ID)
		voices[i].Installed = err == nil
	}
	return voices
}

var downloadClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
	host := req.URL.Hostname()
	if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.User != nil || (req.URL.Port() != "" && req.URL.Port() != "443") ||
		!(host == "huggingface.co" || strings.HasSuffix(host, ".huggingface.co") || strings.HasSuffix(host, ".hf.co")) {
		return errors.New("untrusted Phonon download redirect")
	}
	return nil
}}

var downloadSlot = make(chan struct{}, 1)

// DownloadVoice only downloads authenticated-by-hash catalog entries, never caller URLs or paths.
func DownloadVoice(ctx context.Context, root, dataDir, id string) error {
	voice, ok := Lookup(id)
	if !ok {
		return errors.New("unknown Phonon voice")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case downloadSlot <- struct{}{}:
		defer func() { <-downloadSlot }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := VoicePath(root, dataDir, id); err == nil {
		return nil
	}
	url := "https://huggingface.co/" + manifest.VoicesRepo + "/resolve/" + manifest.VoicesRevision + "/" + voice.Filename
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return errors.New("Phonon voice download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Phonon voice download returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, voice.Size+1))
	if err != nil || int64(len(data)) != voice.Size {
		return errors.New("incomplete Phonon voice download")
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != voice.SHA256 {
		return errors.New("Phonon voice download checksum mismatch")
	}
	dir := filepath.Join(dataDir, "phonon", "voices")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Phonon voice directory: %w", err)
	}
	return fileutil.WriteFileContext(ctx, filepath.Join(dir, id+".safetensors"), data, 0o600)
}
