package onnxrt

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type assetDownloadTestTransport struct {
	handler http.Handler
}

func (transport assetDownloadTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request.Clone(request.Context()))
	response := recorder.Result()
	response.Request = request
	return response, nil
}

// testAssetBaseURL keeps the asset specification on an HTTPS URL while
// assetDownloadTestTransport serves the test response entirely in memory.
// This avoids dependence on host-wide TLS interception and loopback capacity
// without weakening production TLS or bypassing ValidateAssetSpec's HTTPS
// requirement. Tests swap only the transport, so the production client's
// timeout and HTTPS-only redirect policy stay in force.
const testAssetBaseURL = "https://assets.invalid"

func newTestAssetCache(t *testing.T, handler http.Handler) (*AssetCache, string) {
	t.Helper()
	cache := NewAssetCache(t.TempDir(), "AuraGo/onnxrt-test", nil)
	cache.client.Transport = assetDownloadTestTransport{handler: handler}
	return cache, testAssetBaseURL
}

func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func pinnedSpec(id, url string, kind AssetKind, content []byte) AssetSpec {
	hash := sha256.Sum256(content)
	return AssetSpec{ID: id, URL: url, Size: int64(len(content)), SHA256: hex.EncodeToString(hash[:]), Kind: kind}
}

func TestSecureArchiveDestinationRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"../escape",
		"../../escape",
		`..\escape`,
		`nested\..\..\escape`,
		`C:\escape`,
		"C:/escape",
		"C:escape",
		`\\server\share\escape`,
		"//server/share/escape",
		"/absolute",
		`\absolute`,
		"invalid\x00name",
	} {
		if _, err := secureArchiveDestination(root, name); err == nil {
			t.Fatalf("secureArchiveDestination(%q) unexpectedly succeeded", name)
		}
	}
	for _, name := range []string{"runtime/bin/llama-server", `runtime\bin\llama-server`} {
		if destination, err := secureArchiveDestination(root, name); err != nil || !PathWithinRoot(destination, root) {
			t.Fatalf("secureArchiveDestination(%q) = %q, %v", name, destination, err)
		}
	}
}

func TestExtractZipSecureRejectsSymlink(t *testing.T) {
	temp := t.TempDir()
	archivePath := filepath.Join(temp, "runtime.zip")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	header := &zip.FileHeader{Name: "runtime/link"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("../outside")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractZipSecure(archivePath, filepath.Join(temp, "out")); err == nil {
		t.Fatal("expected symlink archive to be rejected")
	}
}

func TestExtractTarGzipSecureExtractsFilesWithoutMaterializingLinks(t *testing.T) {
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "runtime/bin/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}); err != nil {
		t.Fatal(err)
	}
	content := []byte("runtime")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "runtime/bin/libonnxruntime.so.1",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "runtime/bin/libonnxruntime.so",
		Typeflag: tar.TypeSymlink,
		Linkname: "libonnxruntime.so.1",
		Mode:     0o777,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "runtime.tar.gz")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := extractTarGzipSecure(archivePath, target); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(target, "runtime", "bin", "libonnxruntime.so.1"))
	if err != nil || string(raw) != "runtime" {
		t.Fatalf("extracted runtime = %q, %v", raw, err)
	}
	if _, err := os.Lstat(filepath.Join(target, "runtime", "bin", "libonnxruntime.so")); !os.IsNotExist(err) {
		t.Fatalf("archive symlink was materialized: %v", err)
	}
}

func TestFileMatchesRejectsManipulatedCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.bin")
	content := []byte("verified asset")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	if !FileMatches(path, int64(len(content)), hex.EncodeToString(hash[:])) {
		t.Fatal("expected valid cache file")
	}
	if err := os.WriteFile(path, []byte("tampered asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	if FileMatches(path, int64(len("tampered asset")), hex.EncodeToString(hash[:])) {
		t.Fatal("manipulated cache file unexpectedly validated")
	}
}

func TestAssetDownloadCleansInterruptedPartAndReusesOfflineCache(t *testing.T) {
	content := []byte("checksum-verified embedding asset")
	interrupt := true
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		if interrupt {
			_, _ = w.Write(content[:len(content)/2])
			return
		}
		_, _ = w.Write(content)
	}))
	spec := pinnedSpec("test-asset", baseURL+"/asset", AssetFile, content)
	destination := filepath.Join(cache.Root(), "models", "asset.bin")
	if err := cache.EnsureFile(context.Background(), spec, destination); err == nil {
		t.Fatal("interrupted download unexpectedly succeeded")
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatalf("partial download was not cleaned up: %v", err)
	}
	interrupt = false
	if err := cache.EnsureFile(context.Background(), spec, destination); err != nil {
		t.Fatal(err)
	}
	if err := cache.EnsureFile(context.Background(), spec, destination); err != nil {
		t.Fatalf("verified offline cache was not reused: %v", err)
	}
}

func TestAssetDownloadRejectsMatchingSizeWithWrongHash(t *testing.T) {
	content := []byte("expected")
	tampered := []byte("tampered")
	if len(content) != len(tampered) {
		t.Fatal("test data must have matching lengths")
	}
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tampered)
	}))
	spec := pinnedSpec("tampered", baseURL, AssetFile, content)
	destination := filepath.Join(cache.Root(), "asset.bin")
	if err := cache.EnsureFile(context.Background(), spec, destination); err == nil {
		t.Fatal("tampered download unexpectedly succeeded")
	} else if !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("tampered download failed before checksum verification: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("tampered destination exists: %v", err)
	}
}

func TestRuntimeManifestRepairsTamperedMissingAndUnexpectedFilesOffline(t *testing.T) {
	content := buildTestZip(t, map[string]string{
		"runtime/bin/runtime.dll": "verified runtime",
		"runtime/bin/helper.exe":  "verified helper",
	})
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	spec := pinnedSpec("test-runtime", baseURL+"/runtime.zip", AssetZip, content)
	root, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	runtimeFile := filepath.Join(root, "runtime", "bin", "runtime.dll")
	helperFile := filepath.Join(root, "runtime", "bin", "helper.exe")
	assertRepaired := func(path, want string) {
		t.Helper()
		raw, readErr := os.ReadFile(path)
		if readErr != nil || string(raw) != want {
			t.Fatalf("repaired file %s = %q, %v; want %q", path, raw, readErr, want)
		}
	}
	if err := os.WriteFile(runtimeFile, []byte("tampered runtime"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.EnsureRuntime(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	assertRepaired(runtimeFile, "verified runtime")

	if err := os.Remove(helperFile); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.EnsureRuntime(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	assertRepaired(helperFile, "verified helper")

	unexpected := filepath.Join(root, "runtime", "bin", "injected.dll")
	if err := os.WriteFile(unexpected, []byte("unexpected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.EnsureRuntime(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unexpected); !os.IsNotExist(err) {
		t.Fatalf("unexpected runtime file was not removed: %v", err)
	}

	if err := os.Remove(runtimeFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(helperFile, runtimeFile); err == nil {
		if _, err := cache.EnsureRuntime(context.Background(), spec); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(runtimeFile)
		if err != nil {
			t.Fatalf("linked runtime file was not replaced: %v", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("linked runtime file was not replaced: mode=%v", info.Mode())
		}
		assertRepaired(runtimeFile, "verified runtime")
	}
}

func TestAssetCacheSendsUserAgentAndReportsProgress(t *testing.T) {
	content := []byte("pinned asset with user agent")
	var userAgent string
	var last DownloadStatus
	cache := NewAssetCache(t.TempDir(), "AuraGo/onnxrt-test", func(status DownloadStatus) { last = status })
	cache.client.Transport = assetDownloadTestTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		_, _ = w.Write(content)
	})}
	spec := pinnedSpec("ua-asset", testAssetBaseURL+"/asset", AssetFile, content)
	if err := cache.EnsureFile(context.Background(), spec, filepath.Join(cache.Root(), "models", "asset.bin")); err != nil {
		t.Fatal(err)
	}
	if userAgent != "AuraGo/onnxrt-test" {
		t.Fatalf("User-Agent = %q", userAgent)
	}
	if last.Asset != "ua-asset" || last.Downloaded != int64(len(content)) || last.Total != int64(len(content)) || last.Percent != 100 {
		t.Fatalf("last progress = %+v", last)
	}
}

func TestRuntimeLayoutMatchesEmbeddingsCache(t *testing.T) {
	root := filepath.Join("data", "embeddings")
	spec := AssetSpec{ID: "ort-linux-amd64-cpu", Kind: AssetTarGzip}
	if got, want := RuntimeDir(root, spec), filepath.Join(root, "runtimes", "ort-linux-amd64-cpu"); got != want {
		t.Fatalf("RuntimeDir = %q, want %q", got, want)
	}
	for kind, suffix := range map[AssetKind]string{AssetZip: ".zip", AssetTarGzip: ".tar.gz", AssetWheel: ".whl", AssetFile: ".bin"} {
		got := ArchivePath(root, AssetSpec{ID: "x", Kind: kind})
		if want := filepath.Join(root, "downloads", "x"+suffix); got != want {
			t.Fatalf("ArchivePath(%s) = %q, want %q", kind, got, want)
		}
	}
}

func TestRuntimeMarkerKeepsEmbeddingsFormat(t *testing.T) {
	content := buildTestZip(t, map[string]string{"lib/runtime.bin": "runtime"})
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	spec := pinnedSpec("marker-runtime", baseURL+"/runtime.zip", AssetZip, content)
	root, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if root != RuntimeDir(cache.Root(), spec) {
		t.Fatalf("runtime root = %q", root)
	}
	if _, err := os.Stat(ArchivePath(cache.Root(), spec)); err != nil {
		t.Fatalf("verified archive missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache.Root(), ".download.lock")); err != nil {
		t.Fatalf("process lock file missing: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".complete.json"))
	if err != nil {
		t.Fatal(err)
	}
	var marker map[string]any
	if err := json.Unmarshal(raw, &marker); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(marker))
	for key := range marker {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "asset,files,sha256,size,version" {
		t.Fatalf("marker keys = %v", keys)
	}
	if marker["version"] != float64(1) || marker["asset"] != "marker-runtime" || marker["sha256"] != spec.SHA256 {
		t.Fatalf("marker = %v", marker)
	}
	files, ok := marker["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("marker files = %v", marker["files"])
	}
	file := files[0].(map[string]any)
	if file["path"] != "lib/runtime.bin" || file["size"] != float64(len("runtime")) || len(file["sha256"].(string)) != 64 {
		t.Fatalf("marker file = %v", file)
	}
}

func TestValidateAssetSpecRequiresHTTPSAndPins(t *testing.T) {
	valid := pinnedSpec("valid", "https://example.invalid/a.bin", AssetFile, []byte("a"))
	if err := ValidateAssetSpec(valid); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	insecure := valid
	insecure.URL = "http://example.invalid/a.bin"
	sizeless := valid
	sizeless.Size = 0
	shortHash := valid
	shortHash.SHA256 = "abc"
	for name, spec := range map[string]AssetSpec{"http": insecure, "size": sizeless, "hash": shortHash} {
		if err := ValidateAssetSpec(spec); err == nil {
			t.Fatalf("%s spec accepted", name)
		}
	}
}

func TestSafeRemoveAllRefusesRootAndOutside(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "runtimes", "x")
	if err := os.MkdirAll(inside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := SafeRemoveAll(root, root); err == nil {
		t.Fatal("SafeRemoveAll removed its own root")
	}
	if err := SafeRemoveAll(filepath.Join(root, "runtimes"), t.TempDir()); err == nil {
		t.Fatal("SafeRemoveAll removed a directory outside its root")
	}
	if err := SafeRemoveAll(root, inside); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("inside directory still exists: %v", err)
	}
}

func TestAssetDownloadRefusesNonHTTPSRedirect(t *testing.T) {
	content := []byte("asset served over plain HTTP")
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			http.Redirect(w, r, "http://assets.invalid/plain", http.StatusFound)
			return
		}
		_, _ = w.Write(content)
	}))
	spec := pinnedSpec("redirected-asset", baseURL+"/asset", AssetFile, content)
	destination := filepath.Join(cache.Root(), "models", "asset.bin")
	err := cache.EnsureFile(context.Background(), spec, destination)
	if err == nil || !strings.Contains(err.Error(), "refusing non-HTTPS redirect") {
		t.Fatalf("EnsureFile error = %v, want a refused non-HTTPS redirect", err)
	}
	for _, leftover := range []string{destination, destination + ".part"} {
		if _, statErr := os.Stat(leftover); !os.IsNotExist(statErr) {
			t.Fatalf("%s exists after the refused redirect: %v", leftover, statErr)
		}
	}
}

func TestAssetDownloadFollowsHTTPSRedirect(t *testing.T) {
	content := []byte("asset behind an HTTPS redirect")
	var requested []string
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Scheme+"://"+r.URL.Host+r.URL.Path)
		if r.URL.Path == "/asset" {
			http.Redirect(w, r, "https://mirror.invalid/blob/asset.bin", http.StatusFound)
			return
		}
		_, _ = w.Write(content)
	}))
	spec := pinnedSpec("redirected-asset", baseURL+"/asset", AssetFile, content)
	destination := filepath.Join(cache.Root(), "models", "asset.bin")
	if err := cache.EnsureFile(context.Background(), spec, destination); err != nil {
		t.Fatal(err)
	}
	if !FileMatches(destination, spec.Size, spec.SHA256) {
		t.Fatal("redirected download did not produce the pinned file")
	}
	if got, want := strings.Join(requested, ","), "https://assets.invalid/asset,https://mirror.invalid/blob/asset.bin"; got != want {
		t.Fatalf("requested = %s, want %s", got, want)
	}
}

func TestValidateAssetSpecRejectsUnsafeIDsAndNonHexHashes(t *testing.T) {
	valid := pinnedSpec("ort-linux-amd64-cpu", "https://example.invalid/a.bin", AssetFile, []byte("a"))
	accepted := []string{
		"a",
		"0",
		"granite-gguf-q8_0",
		"llama-windows-amd64-cuda-runtime",
		"ort-1.27.0-webgpu",
	}
	for _, id := range accepted {
		spec := valid
		spec.ID = id
		if err := ValidateAssetSpec(spec); err != nil {
			t.Errorf("ID %q rejected: %v", id, err)
		}
	}
	rejected := []string{
		"",
		".",
		"..",
		"../escape",
		"a/b",
		`a\b`,
		"C:",
		"Upper",
		"-leading-dash",
		".hidden",
		"_leading-underscore",
		"with space",
		"nul\x00byte",
		"ünicode",
	}
	for _, id := range rejected {
		spec := valid
		spec.ID = id
		if err := ValidateAssetSpec(spec); err == nil {
			t.Errorf("ID %q accepted", id)
		}
	}
	upper := valid
	upper.SHA256 = strings.ToUpper(valid.SHA256)
	if err := ValidateAssetSpec(upper); err != nil {
		t.Errorf("upper-case SHA-256 rejected: %v", err)
	}
	for name, hash := range map[string]string{
		"empty":   "",
		"short":   strings.Repeat("a", 63),
		"long":    strings.Repeat("a", 65),
		"non-hex": strings.Repeat("g", 64),
		"padded":  " " + strings.Repeat("a", 62) + " ",
		"prefix":  "0x" + strings.Repeat("a", 62),
	} {
		spec := valid
		spec.SHA256 = hash
		if err := ValidateAssetSpec(spec); err == nil {
			t.Errorf("%s SHA-256 %q accepted", name, hash)
		}
	}
}

func TestEnsureRuntimeRejectsUnsupportedKindBeforeDownload(t *testing.T) {
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected download of %s", r.URL)
		http.Error(w, "unexpected download", http.StatusTeapot)
	}))
	content := []byte("not a runtime archive")
	for _, kind := range []AssetKind{AssetFile, AssetKind("rar"), AssetKind("")} {
		spec := pinnedSpec("unsupported-runtime", baseURL+"/runtime", kind, content)
		root, err := cache.EnsureRuntime(context.Background(), spec)
		if err == nil || !strings.Contains(err.Error(), "unsupported archive kind") {
			t.Fatalf("EnsureRuntime(kind %q) = %q, %v; want an unsupported archive kind error", kind, root, err)
		}
	}
	for _, dir := range []string{"downloads", "runtimes"} {
		if _, err := os.Stat(filepath.Join(cache.Root(), dir)); !os.IsNotExist(err) {
			t.Fatalf("%s directory created for an unsupported runtime: %v", dir, err)
		}
	}
}
