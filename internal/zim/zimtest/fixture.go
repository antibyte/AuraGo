package zimtest

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
	"testing"
	"time"

	"aurago/internal/fileutil"
)

// Real-world fixture: a small English Wikipedia selection with full-text and
// title Xapian indexes, CC BY-SA content from openzim/zim-testing-suite. It is
// downloaded on demand into the user cache and never committed.
const (
	RealFixtureEnv    = "AURAGO_ZIM_REAL_FIXTURE"
	RealFixtureName   = "wikipedia_en_climate_change_mini_2024-06.zim"
	RealFixtureURL    = "https://raw.githubusercontent.com/openzim/zim-testing-suite/d0df7fde04ff91c357404236d6b1a0ac42015b69/data/nons/wikipedia_en_climate_change_mini_2024-06.zim"
	RealFixtureSize   = 8978471
	RealFixtureSHA256 = "3485a2c475a2356331f33f137ce26ed7116e62c7062d73a7773362d6dff005f4"
)

// RealFixture returns the path of the verified real-world fixture. Unless
// AURAGO_ZIM_REAL_FIXTURE=1 it skips the test.
func RealFixture(tb testing.TB) string {
	tb.Helper()
	if os.Getenv(RealFixtureEnv) != "1" {
		tb.Skipf("set %s=1 to download and test the real ZIM fixture %s (%d bytes)", RealFixtureEnv, RealFixtureName, RealFixtureSize)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		tb.Fatalf("user cache dir: %v", err)
	}
	dir := filepath.Join(cache, "aurago", "test-fixtures", "zim")
	path := filepath.Join(dir, RealFixtureName)
	if verifyFixture(path) == nil {
		return path
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := downloadFixture(ctx, dir, path); err != nil {
		tb.Fatalf("download real ZIM fixture: %v", err)
	}
	return path
}

func verifyFixture(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return err
	}
	if n != RealFixtureSize {
		return fmt.Errorf("size %d, want %d", n, RealFixtureSize)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != RealFixtureSHA256 {
		return fmt.Errorf("sha256 %s, want %s", got, RealFixtureSHA256)
	}
	return nil
}

func downloadFixture(ctx context.Context, dir, path string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, RealFixtureURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", RealFixtureURL, resp.Status)
	}
	tmp, err := os.CreateTemp(dir, ".zim-fixture-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, RealFixtureSize+1)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := verifyFixture(tmp.Name()); err != nil {
		return fmt.Errorf("downloaded fixture: %w", err)
	}
	if err := fileutil.RenameContext(ctx, tmp.Name(), path); err != nil {
		// A parallel test process may have published the same file first.
		if verifyFixture(path) == nil {
			return nil
		}
		return errors.Join(err, verifyFixture(path))
	}
	return nil
}
