package homedash

import (
	"context"
	"fmt"
	"testing"
)

func TestStoreSetMaxDashboardsWhileCreating(t *testing.T) {
	st, _, _ := openTestStore(t)
	ctx := context.Background()
	st.SetMaxDashboards(1)
	if _, err := st.CreateDashboard(ctx, storeDoc("one"), SavedByUser); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDashboard(ctx, storeDoc("two"), SavedByUser); code(err) != CodeLimitExceeded {
		t.Fatalf("second dashboard over a cap of 1: %v", err)
	}
	st.SetMaxDashboards(0) // ignored
	if _, err := st.CreateDashboard(ctx, storeDoc("three"), SavedByUser); code(err) != CodeLimitExceeded {
		t.Fatalf("cap 0 must be ignored: %v", err)
	}
	// Hot reload changes the cap while requests save (meaningful under -race).
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			st.SetMaxDashboards(10 + i%5)
		}
	}()
	for i := 0; i < 5; i++ {
		_, _ = st.CreateDashboard(ctx, storeDoc(fmt.Sprintf("race-%d", i)), SavedByUser)
	}
	<-done
}
