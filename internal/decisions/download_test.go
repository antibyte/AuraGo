package decisions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestArtifactDownloadResumesAndVerifiesBeforePublish(t *testing.T) {
	payload := []byte(strings.Repeat("pinned-model-bytes-", 700))
	checksum := sha256.Sum256(payload)
	artifact := Artifact{Name: "model", Filename: "model.gguf", Revision: "immutable", Size: int64(len(payload)), SHA256: hex.EncodeToString(checksum[:])}
	const offset = 1300
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Range") != fmt.Sprintf("bytes=%d-", offset) {
			t.Errorf("Range = %q", request.Header.Get("Range"))
		}
		return &http.Response{StatusCode: http.StatusPartialContent, Header: http.Header{"Content-Range": []string{fmt.Sprintf("bytes %d-%d/%d", offset, len(payload)-1, len(payload))}}, Body: io.NopCloser(strings.NewReader(string(payload[offset:]))), Request: request}, nil
	})}

	root := t.TempDir()
	partPath := filepath.Join(root, artifact.Filename+".part")
	if err := os.WriteFile(partPath, payload[:offset], 0o600); err != nil {
		t.Fatal(err)
	}
	cache := newArtifactCache(root, client, nil)
	cache.urlFor = func(Manifest, Artifact) string { return "https://model.test/model.gguf" }
	if err := cache.download(context.Background(), Manifest{Repository: "test"}, artifact, filepath.Join(root, artifact.Filename), "downloading_model"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, artifact.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatal("published artifact differs from pinned payload")
	}
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Fatalf("partial file remains after publication: %v", err)
	}
}

func TestArtifactDownloadRefusesWrongChecksum(t *testing.T) {
	payload := []byte("wrong bytes")
	artifact := Artifact{Name: "model", Filename: "model.gguf", Revision: "immutable", Size: int64(len(payload)), SHA256: strings.Repeat("0", 64)}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(payload))), Request: request}, nil
	})}

	root := t.TempDir()
	destination := filepath.Join(root, artifact.Filename)
	cache := newArtifactCache(root, client, nil)
	cache.urlFor = func(Manifest, Artifact) string { return "https://model.test/model.gguf" }
	err := cache.download(context.Background(), Manifest{Repository: "test"}, artifact, destination, "downloading_model")
	if ErrorCode(err) != CodeModelIntegrityFailed {
		t.Fatalf("ErrorCode(%v) = %q", err, ErrorCode(err))
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("unverified artifact was published: %v", err)
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatalf("unverified partial artifact was retained: %v", err)
	}
}

func TestPartialFileSizeReturnsRegularFileSize(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "partial")
	if size, err := partialFileSize(path); err != nil || size != 0 {
		t.Fatalf("missing partial size = %d, %v", size, err)
	}
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if size, err := partialFileSize(path); err != nil || size != 3 {
		t.Fatalf("regular partial size = %d, %v", size, err)
	}
}

func TestContentRangeStartRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "bytes */4", "bytes 1/4", "items 1-2/4"} {
		if _, err := contentRangeStart(value); err == nil {
			t.Fatalf("contentRangeStart(%q) succeeded", value)
		}
	}
}
