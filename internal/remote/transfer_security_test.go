//go:build !remote_minimal

package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type failedTransferReader struct{}

func (failedTransferReader) Read([]byte) (int, error) { return 0, errors.New("connection lost") }

func TestTransferPublicationPreservesPriorFile(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("prior"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishTransferDownload(context.Background(), root, "target", failedTransferReader{}); err == nil {
		t.Fatal("read error ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publishTransferDownload(ctx, root, "target", strings.NewReader("replacement")); err == nil {
		t.Fatal("cancel ignored")
	}
	data, _ := root.ReadFile("target")
	if string(data) != "prior" {
		t.Fatal("prior destination changed")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := publishTransferDownload(context.Background(), root, "escape/stolen", strings.NewReader("secret")); err == nil {
		t.Fatal("symlink escaped root")
	}
	if _, err := root.Open("escape/stolen"); err == nil {
		t.Fatal("rooted read escaped")
	}
	if _, err := os.Stat(filepath.Join(outside, "stolen")); !os.IsNotExist(err) {
		t.Fatal("outside target exists")
	}
}

func TestSSHContextCancelsSilentHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		c, err := listener.Accept()
		if err == nil {
			defer c.Close()
			_, _ = io.Copy(io.Discard, c)
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	prior := InsecureHostKey
	InsecureHostKey = true
	defer func() { InsecureHostKey = prior }()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := ExecuteRemoteCommand(ctx, host, port, "fixture", []byte("fixture"), "true"); err == nil {
		t.Fatal("silent peer authenticated")
	}
	if time.Since(start) > time.Second {
		t.Fatal("SSH handshake ignored cancellation")
	}
}
