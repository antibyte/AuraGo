package desktop

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSFTPMutationHandlersSucceedOverLocalSSHAndSFTP(t *testing.T) {
	previousInsecureHostKey := remote.InsecureHostKey
	remote.InsecureHostKey = false
	t.Cleanup(func() { remote.InsecureHostKey = previousInsecureHostKey })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	const deviceID = "sftp-test-device"
	const username = "sftp-fixture"
	const password = "fixture-password"
	listener, hostKey := startSFTPHandlerTestServer(t, username, password)
	knownHosts := knownhosts.Line([]string{listener.Addr().String()}, hostKey) + "\n"
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(knownHosts), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("sftp/test-device", password); err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().(*net.TCPAddr)
	if err := inventory.AddDevice(db, inventory.DeviceRecord{
		ID:            deviceID,
		Name:          "Local SFTP fixture",
		Type:          "server",
		Protocol:      inventory.ProtocolSSH,
		IPAddress:     address.IP.String(),
		Port:          address.Port,
		Username:      username,
		VaultSecretID: "sftp/test-device",
	}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	assertSFTPJSONMutation(t, deviceID, HandleSFTPMkdir(db, vault, logger), map[string]string{
		"device_id": deviceID,
		"path":      "docs",
	})
	assertSFTPUpload(t, deviceID, HandleSFTPUpload(db, vault, logger), "docs/source.txt", "mutation fixture contents")
	assertSFTPJSONMutation(t, deviceID, HandleSFTPRename(db, vault, logger), map[string]string{
		"device_id": deviceID,
		"old_path":  "docs/source.txt",
		"new_path":  "docs/renamed.txt",
	})
	assertSFTPJSONMutation(t, deviceID, HandleSFTPCopy(db, vault, logger), map[string]string{
		"device_id": deviceID,
		"src_path":  "docs/renamed.txt",
		"dst_path":  "docs/copied.txt",
	})
	assertSFTPJSONMutation(t, deviceID, HandleSFTPMove(db, vault, logger), map[string]string{
		"device_id": deviceID,
		"src_path":  "docs/copied.txt",
		"dst_path":  "docs/moved.txt",
	})

	client, cleanup, err := connectSFTP(deviceID, db, vault, logger)
	if err != nil {
		t.Fatalf("connect SFTP for post-move verification: %v", err)
	}
	defer cleanup()
	assertSFTPFileContents(t, client, "docs/renamed.txt", "mutation fixture contents")
	assertSFTPFileContents(t, client, "docs/moved.txt", "mutation fixture contents")
	if _, err := client.Stat("docs/copied.txt"); err == nil {
		t.Fatal("move left the source file in place")
	}

	assertSFTPJSONMutation(t, deviceID, HandleSFTPDelete(db, vault, logger), map[string]string{
		"device_id": deviceID,
		"path":      "docs/moved.txt",
	})
	if _, err := client.Stat("docs/moved.txt"); err == nil {
		t.Fatal("delete left the destination file in place")
	}
	assertSFTPFileContents(t, client, "docs/renamed.txt", "mutation fixture contents")
}

func startSFTPHandlerTestServer(t *testing.T, username, password string) (net.Listener, ssh.PublicKey) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(conn ssh.ConnMetadata, candidate []byte) (*ssh.Permissions, error) {
		if conn.User() != username || string(candidate) != password {
			return nil, fmt.Errorf("invalid SFTP fixture credentials")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handlers := sftp.InMemHandler()
	var connections sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				serveSFTPHandlerTestConnection(conn, config, handlers)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-acceptDone
		connections.Wait()
	})
	return listener, signer.PublicKey()
}

func serveSFTPHandlerTestConnection(conn net.Conn, config *ssh.ServerConfig, handlers sftp.Handlers) {
	defer conn.Close()
	serverConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "expected session")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			return
		}
		for request := range channelRequests {
			if request.Type != "subsystem" {
				_ = request.Reply(false, nil)
				continue
			}
			var subsystem struct{ Name string }
			if err := ssh.Unmarshal(request.Payload, &subsystem); err != nil || subsystem.Name != "sftp" {
				_ = request.Reply(false, nil)
				continue
			}
			_ = request.Reply(true, nil)
			server := sftp.NewRequestServer(channel, handlers)
			_ = server.Serve()
			_ = server.Close()
			return
		}
		_ = channel.Close()
	}
}

func assertSFTPJSONMutation(t *testing.T, deviceID string, handler http.HandlerFunc, payload map[string]string) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"device_id": []string{deviceID}}.Encode()
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/sftp/mutation?"+query, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertSFTPHandlerOK(t, response)
}

func assertSFTPUpload(t *testing.T, deviceID string, handler http.HandlerFunc, remotePath, contents string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("device_id", deviceID); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("remote_path", remotePath); err != nil {
		t.Fatal(err)
	}
	file, err := form.CreateFormFile("file", filepath.Base(remotePath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, contents); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"device_id": []string{deviceID}}.Encode()
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/sftp/upload?"+query, &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertSFTPHandlerOK(t, response)
}

func assertSFTPHandlerOK(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("SFTP mutation response = %d %s", response.Code, response.Body.String())
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || !result.OK {
		t.Fatalf("SFTP mutation response body = %s (decode error %v)", response.Body.String(), err)
	}
}

func assertSFTPFileContents(t *testing.T, client *sftp.Client, remotePath, want string) {
	t.Helper()
	file, err := client.Open(remotePath)
	if err != nil {
		t.Fatalf("open remote fixture %q: %v", remotePath, err)
	}
	got, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		t.Fatalf("read remote fixture %q: %v", remotePath, readErr)
	}
	if closeErr != nil {
		t.Fatalf("close remote fixture %q: %v", remotePath, closeErr)
	}
	if string(got) != want {
		t.Fatalf("remote fixture %q = %q, want %q", remotePath, got, want)
	}
}
