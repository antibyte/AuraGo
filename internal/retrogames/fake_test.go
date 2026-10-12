package retrogames

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fakeResponse struct {
	page Page
	err  error
}

type fakeFetcher struct {
	mu        sync.Mutex
	responses map[string][]fakeResponse
	fallback  func(query, cursor string) (Page, error)
	calls     []string
	block     chan struct{}
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{responses: map[string][]fakeResponse{}}
}

func (f *fakeFetcher) add(platform, cursor string, page Page, err error) {
	src, _ := SourceByKey(platform)
	key := src.Query() + "|" + cursor
	f.mu.Lock()
	f.responses[key] = append(f.responses[key], fakeResponse{page: page, err: err})
	f.mu.Unlock()
}

func (f *fakeFetcher) FetchPage(ctx context.Context, query, cursor string) (Page, error) {
	f.mu.Lock()
	key := query + "|" + cursor
	f.calls = append(f.calls, key)
	block := f.block
	var resp *fakeResponse
	if queue := f.responses[key]; len(queue) > 0 {
		r := queue[0]
		f.responses[key] = queue[1:]
		resp = &r
	}
	fallback := f.fallback
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return Page{}, ctx.Err()
		}
	}
	if resp != nil {
		return resp.page, resp.err
	}
	if fallback != nil {
		return fallback(query, cursor)
	}
	return Page{Items: []Item{}}, nil
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// callsFor counts the requests made for one platform's query.
func (f *fakeFetcher) callsFor(platform string) int {
	src, _ := SourceByKey(platform)
	prefix := src.Query() + "|"
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// onePerSource answers every source with a single valid item.
func onePerSource(query, cursor string) (Page, error) {
	for _, src := range Sources() {
		if src.Query() == query {
			return Page{Items: []Item{testItem(src.Key+"_default", "Default "+src.Label, "dosbox", 1990, 1)}}, nil
		}
	}
	return Page{Items: []Item{}}, nil
}

func testItem(id, title, emulator string, year int, downloads int64) Item {
	return Item{Identifier: id, Title: FlexText{title}, Year: FlexText{strconv.Itoa(year)}, Emulator: FlexText{emulator}, Downloads: FlexText{strconv.FormatInt(downloads, 10)}}
}

type sleepRecorder struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *sleepRecorder) sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	r.waits = append(r.waits, d)
	r.mu.Unlock()
	return nil
}

func (r *sleepRecorder) count(d time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, w := range r.waits {
		if w == d {
			n++
		}
	}
	return n
}
