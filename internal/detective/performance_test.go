package detective

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// benchCase creates a running case with `sources` archived sources of 20 KB evidence
// each and returns the service and the case ID. It is the fixture the audit measured.
func benchCase(b *testing.B, sources int) (*Service, string) {
	b.Helper()
	s, err := New(Options{Path: filepath.Join(b.TempDir(), "bench.db")})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = s.Close() })
	c, err := s.Create(Request{Topic: "benchmark", Effort: "maximum"})
	if err != nil {
		b.Fatal(err)
	}
	s.mu.Lock()
	c.Run.Status = "running"
	c.Run.Phase = "research"
	c.Run.Profile = Profiles()["maximum"]
	if err = s.saveLocked(&c); err != nil {
		s.mu.Unlock()
		b.Fatal(err)
	}
	s.mu.Unlock()
	x := &Session{service: s, CaseID: c.ID, last: s.now()}
	for i := 0; i < sources; i++ {
		if _, err = x.RecordSource(fmt.Sprintf("https://benchmark.example/%d", i), "Benchmark source", "web_scraper", strings.Repeat("recorded evidence. ", 1000), true); err != nil {
			b.Fatal(err)
		}
	}
	return s, c.ID
}

// BenchmarkCaseHotPaths measures the three paths the audit found scaling with the case
// body. Run it after a storage change:
//
//	go test ./internal/detective -bench CaseHotPaths -run '^$' -benchmem
//
// The targets are < 2 ms for a live read, < 1 ms for the list and < 15 ms for one state
// write at 200 sources; `internal/detective/AGENTS.md` records the decision that follows
// from the result (one mutex as long as reads stay in the microseconds).
func BenchmarkCaseHotPaths(b *testing.B) {
	for _, sources := range []int{0, 20, 200} {
		b.Run(fmt.Sprintf("sources=%d", sources), func(b *testing.B) {
			s, key := benchCase(b, sources)
			x := &Session{service: s, CaseID: key, last: s.now()}
			b.Run("live", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := s.Live(key); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("list", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := s.ListSummaries(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("state-write", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if err := x.update(func(*Case) error { return nil }); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
