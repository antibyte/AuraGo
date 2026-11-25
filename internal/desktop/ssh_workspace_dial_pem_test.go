package desktop

import (
	"context"
	"strings"
	"testing"
)

func TestDialSSHRejectsUnparsablePEMInsteadOfSendingItAsPassword(t *testing.T) {
	secret := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nnot-a-real-key\n-----END OPENSSH PRIVATE KEY-----\n")
	_, err := DialSSH(context.Background(), SSHAccess{Host: "127.0.0.1", Port: 1, Username: "u", Secret: secret}, SSHDialOptions{})
	if err == nil || !strings.Contains(err.Error(), "private key could not be used") {
		t.Fatalf("err = %v; want a clear private key error before any dial", err)
	}
}
