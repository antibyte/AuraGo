package whatsapp

import (
	"context"
	"testing"

	"go.mau.fi/whatsmeow/store"
)

func TestSQLiteBackendOpensUnlinkedStoreAndNamesDevice(t *testing.T) {
	var _ waClient = (*fakeClient)(nil)
	var _ deviceStore = (*fakeStore)(nil)
	var _ backend = (*fakeBackend)(nil)

	ctx := context.Background()
	s, err := (sqliteBackend{}).open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	client := s.NewClient()
	if client.IsConnected() || client.IsLoggedIn() || !client.OwnJID().IsEmpty() {
		t.Fatal("an unlinked client reports a session")
	}
	if err := s.DeleteLocal(ctx); err != nil {
		t.Fatalf("DeleteLocal on an unlinked store: %v", err)
	}
	if got := store.DeviceProps.GetOs(); got != "AuraGo" {
		t.Fatalf("linked device name = %q, want AuraGo", got)
	}
}
