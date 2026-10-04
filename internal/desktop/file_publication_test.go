package desktop

import (
	"context"
	"errors"
	"testing"

	"aurago/internal/fileutil"
)

func TestFilePublicationFailurePreservesOriginal(t *testing.T) {
	s := testService(t)
	if err := s.WriteFile(context.Background(), "Documents/atomic.txt", "original", SourceUser); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("publication revoked")
	ctx := fileutil.WithPublicationGate(context.Background(), func(func() error) error { return refused })
	if err := s.WriteFile(ctx, "Documents/atomic.txt", "replacement", SourceUser); !errors.Is(err, refused) {
		t.Fatalf("write error: %v", err)
	}
	got, _, err := s.ReadFile(context.Background(), "Documents/atomic.txt")
	if err != nil || got != "original" {
		t.Fatal("failed replacement lost original")
	}
	if err := s.DeletePath(ctx, "Documents/atomic.txt", SourceUser); !errors.Is(err, refused) {
		t.Fatalf("delete error: %v", err)
	}
	if _, _, err := s.ReadFile(context.Background(), "Documents/atomic.txt"); err != nil {
		t.Fatal("revoked delete lost original")
	}
}
