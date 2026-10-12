package retrogames

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"aurago/internal/security"
)

func requireLive(t *testing.T) {
	t.Helper()
	if os.Getenv("AURAGO_RETROGAMES_LIVE") != "1" {
		t.Skip("set AURAGO_RETROGAMES_LIVE=1 to run against archive.org")
	}
}

// TestLiveHallOfFameIdentifiersExist checks every curated id on archive.org.
func TestLiveHallOfFameIdentifiersExist(t *testing.T) {
	requireLive(t)
	for _, g := range HallOfFame("") {
		url := "https://archive.org/metadata/" + g.Identifier + "/metadata"
		client, err := security.NewStrictPublicHTTPClientForURL(url, 30*time.Second)
		if err != nil {
			t.Fatalf("%s: client: %v", g.Identifier, err)
		}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", g.Identifier, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		client.CloseIdleConnections()
		var meta struct {
			Result struct {
				MediaType string   `json:"mediatype"`
				Emulator  FlexText `json:"emulator"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &meta); err != nil {
			t.Fatalf("%s: decode: %v", g.Identifier, err)
		}
		if meta.Result.MediaType != "software" || meta.Result.Emulator.First() == "" {
			t.Errorf("%s: mediatype=%q emulator=%q", g.Identifier, meta.Result.MediaType, meta.Result.Emulator.First())
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// TestLiveScrapeFirstPagePerSource parses one real page per source.
func TestLiveScrapeFirstPagePerSource(t *testing.T) {
	requireLive(t)
	f := NewHTTPFetcher()
	for _, src := range Sources() {
		page, err := f.FetchPage(context.Background(), src.Query(), "")
		if err != nil {
			t.Fatalf("%s: %v", src.Key, err)
		}
		valid := 0
		for _, it := range page.Items {
			if _, ok := normalizeItem(src, it); ok {
				valid++
			}
		}
		if len(page.Items) == 0 || valid*10 < len(page.Items)*9 {
			t.Errorf("%s: %d items, %d valid, total %d", src.Key, len(page.Items), valid, page.Total)
		}
		t.Logf("%s: %d items on the first page, %d valid, total %d, cursor=%v", src.Key, len(page.Items), valid, page.Total, page.Cursor != "")
		time.Sleep(time.Second)
	}
}
