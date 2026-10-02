package bluetooth

import (
	"errors"
	"strings"
)

// busError is a platform-neutral D-Bus error reply.
type busError struct {
	Name    string
	Message string
}

func (e *busError) Error() string {
	if e.Message != "" {
		return e.Name + ": " + e.Message
	}
	return e.Name
}

// mapBlueZError turns BlueZ failures into stable coded errors. op names the
// operation ("pair", "connect", "power", ...). Coded errors pass through. The
// raw BlueZ text stays available through errors.Unwrap for server logs only.
func mapBlueZError(op string, err error) error {
	if err == nil || ErrorCode(err) != "" {
		return err
	}
	var bus *busError
	if !errors.As(err, &bus) {
		return codedError(ErrorGeneric, "The Bluetooth operation failed.", err)
	}
	message := strings.ToLower(bus.Message)
	switch bus.Name {
	case "org.bluez.Error.AlreadyExists":
		if op == "pair" {
			return nil
		}
	case "org.bluez.Error.Rejected", "org.bluez.Error.Canceled", "org.bluez.Error.AuthenticationCanceled", "org.bluez.Error.AuthenticationRejected":
		return codedError(ErrorPairingRejected, "The pairing was rejected or cancelled.", err)
	case "org.bluez.Error.AuthenticationFailed", "org.bluez.Error.AuthenticationTimeout":
		return codedError(ErrorPairingFailed, "Pairing failed; the confirmation or code did not match.", err)
	case "org.bluez.Error.InProgress":
		return codedError(ErrorOperationBusy, "Another Bluetooth operation is already running for this device.", err)
	case "org.bluez.Error.NotReady":
		return codedError(ErrorPoweredOff, "The Bluetooth adapter is turned off.", err)
	case "org.bluez.Error.DoesNotExist":
		return codedError(ErrorDeviceNotFound, "The Bluetooth device no longer exists.", err)
	case "org.bluez.Error.ConnectionAttemptFailed", "org.bluez.Error.NotAvailable":
		return codedError(ErrorDeviceUnreachable, "The Bluetooth device did not answer.", err)
	case "org.bluez.Error.Failed":
		switch {
		case strings.Contains(message, "rfkill") || strings.Contains(message, "blocked"):
			return codedError(ErrorBlocked, "Bluetooth is blocked.", err)
		case strings.Contains(message, "page timeout") || strings.Contains(message, "page-timeout") ||
			strings.Contains(message, "host is down") || strings.Contains(message, "connection refused") ||
			strings.Contains(message, "abort-by-local"):
			return codedError(ErrorDeviceUnreachable, "The Bluetooth device did not answer.", err)
		}
	}
	return codedError(ErrorGeneric, "The Bluetooth operation failed.", err)
}
