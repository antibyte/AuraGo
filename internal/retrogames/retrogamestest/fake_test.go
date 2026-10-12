package retrogamestest_test

import (
	"context"
	"testing"
	"time"

	"aurago/internal/retrogames"
	"aurago/internal/retrogames/retrogamestest"
)

func TestFakeFetcherFeedsService(t *testing.T) {
	f := &retrogamestest.Fetcher{BySource: map[string][]retrogames.Item{
		retrogames.PlatformMSDOS: {retrogamestest.Item("msdos_Prince_of_Persia_1990", "Prince of Persia", "dosbox", 1990, 9)},
	}}
	svc := retrogamestest.NewService(t, f)
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Search(context.Background(), retrogames.Query{Text: "persia"})
	if err != nil || res.Total != 1 {
		t.Fatalf("search = %+v err=%v", res, err)
	}
	st, _ := svc.Status(context.Background())
	if st.TotalGames != 8 {
		t.Fatalf("every source should get a filler game, total = %d", st.TotalGames)
	}
}
