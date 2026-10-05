//go:build !remote_minimal

package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHCommandCarriesSecretOnlyOnStdin(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	type captured struct {
		command, input string
		err            error
	}
	received := make(chan captured, 1)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			received <- captured{err: err}
			return
		}
		defer raw.Close()
		_, channels, requests, err := ssh.NewServerConn(raw, serverConfig)
		if err != nil {
			received <- captured{err: err}
			return
		}
		go ssh.DiscardRequests(requests)
		newChannel := <-channels
		if newChannel == nil {
			received <- captured{err: fmt.Errorf("missing session")}
			return
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			received <- captured{err: err}
			return
		}
		defer channel.Close()
		request := <-requests
		var command struct{ Command string }
		if request == nil || request.Type != "exec" {
			received <- captured{err: fmt.Errorf("missing exec request")}
			return
		}
		if err := ssh.Unmarshal(request.Payload, &command); err != nil {
			received <- captured{err: err}
			return
		}
		_ = request.Reply(true, nil)
		input, err := io.ReadAll(channel)
		received <- captured{command.Command, string(input), err}
		_, _ = io.WriteString(channel, "ok\n")
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
	}()
	previous := InsecureHostKey
	InsecureHostKey = true
	defer func() { InsecureHostKey = previous }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload := "private deployment fixture\n"
	output, err := ExecuteRemoteCommand(ctx, "127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "fixture", []byte("fixture"), "bash -s", strings.NewReader(payload))
	if err != nil || output != "ok\n" {
		t.Fatalf("SSH input failed: %v", err)
	}
	select {
	case got := <-received:
		if got.err != nil || got.command != "bash -s" || got.input != payload {
			t.Fatalf("secret input transport contract failed: %v", got.err)
		}
	case <-ctx.Done():
		t.Fatal("SSH fixture did not capture input")
	}
}
