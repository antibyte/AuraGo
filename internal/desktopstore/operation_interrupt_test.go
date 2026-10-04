package desktopstore

import (
	"context"
	"testing"
)

func TestStoreInterruptedOperationReleasesPendingReservation(t *testing.T) {
	svc := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(19180))
	op, err := svc.StartInstall(context.Background(), InstallRequest{AppID: "n8n", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.RunOperation(ctx, op.ID); err == nil {
		t.Fatal("cancelled operation ran")
	}
	after, err := svc.Operation(context.Background(), op.ID)
	if err != nil || after.Status != OperationFailed {
		t.Fatalf("reservation survived: %s %v", after.Status, err)
	}
	if err := svc.InterruptOperation(op.ID, context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	again, _ := svc.Operation(context.Background(), op.ID)
	if again.Error != after.Error {
		t.Fatal("terminal record was overwritten")
	}
	if _, err := svc.StartInstall(context.Background(), InstallRequest{AppID: "n8n", BindMode: BindModeLocal}); err != nil {
		t.Fatalf("retry stayed blocked: %v", err)
	}
}
