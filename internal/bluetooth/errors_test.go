package bluetooth

import (
	"errors"
	"fmt"
	"testing"
)

func TestMapBlueZErrorProducesStableCodes(t *testing.T) {
	for _, tc := range []struct {
		op   string
		err  error
		want string
	}{
		{"pair", &busError{Name: "org.bluez.Error.AuthenticationRejected"}, ErrorPairingRejected},
		{"pair", &busError{Name: "org.bluez.Error.Rejected"}, ErrorPairingRejected},
		{"pair", &busError{Name: "org.bluez.Error.AuthenticationFailed"}, ErrorPairingFailed},
		{"connect", &busError{Name: "org.bluez.Error.InProgress"}, ErrorOperationBusy},
		{"connect", &busError{Name: "org.bluez.Error.NotReady"}, ErrorPoweredOff},
		{"power", &busError{Name: "org.bluez.Error.Failed", Message: "Blocked through rfkill"}, ErrorBlocked},
		{"connect", &busError{Name: "org.bluez.Error.Failed", Message: "Page Timeout"}, ErrorDeviceUnreachable},
		{"connect", &busError{Name: "org.bluez.Error.Failed", Message: "br-connection-page-timeout"}, ErrorDeviceUnreachable},
		{"connect", &busError{Name: "org.bluez.Error.DoesNotExist"}, ErrorDeviceNotFound},
		{"connect", fmt.Errorf("wrapped: %w", &busError{Name: "org.bluez.Error.ConnectionAttemptFailed"}), ErrorDeviceUnreachable},
		{"connect", errors.New("socket closed"), ErrorGeneric},
		{"pair", codedError(ErrorPairingInteractionRequired, "needs confirmation", nil), ErrorPairingInteractionRequired},
	} {
		if got := ErrorCode(mapBlueZError(tc.op, tc.err)); got != tc.want {
			t.Errorf("mapBlueZError(%s, %v) = %q, want %q", tc.op, tc.err, got, tc.want)
		}
	}
	if err := mapBlueZError("pair", &busError{Name: "org.bluez.Error.AlreadyExists"}); err != nil {
		t.Fatalf("AlreadyExists while pairing must count as success, got %v", err)
	}
	if err := mapBlueZError("connect", nil); err != nil {
		t.Fatalf("nil must stay nil, got %v", err)
	}
}
