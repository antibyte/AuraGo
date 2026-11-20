package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type homepageLibraryTransport func(*http.Request) (*http.Response, error)

func (f homepageLibraryTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func writeHomepageLibraryFixture(t *testing.T, root, name, data string) {
	t.Helper()
	name = filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func homepageLibraryTestResponse(t *testing.T, body any) *http.Response {
	t.Helper()
	data, ok := body.([]byte)
	if !ok {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	return &http.Response{StatusCode: 200, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}
}

func TestHomepageLibraryInstallAndReuse(t *testing.T) {
	workspace := t.TempDir()
	writeHomepageLibraryFixture(t, workspace, "site/index.html", "<h1>Homepage fixture</h1>")
	writeHomepageLibraryFixture(t, workspace, "site/package.json", `{"devDependencies":{"vite":"*"}}`)
	version, corrupt, downloads := "5.0.0", false, 0
	var cancelDownload context.CancelFunc
	library := homepageLibraries["bootstrap"]
	assets := []homepageLibraryAsset{}
	for _, name := range []string{library.CSS, library.Script, "LICENSE"} {
		content := []byte("fixture for /" + name)
		hash := sha256.Sum256(content)
		assets = append(assets, homepageLibraryAsset{Name: "/" + name, Size: int64(len(content)), Hash: base64.StdEncoding.EncodeToString(hash[:])})
	}
	client := &http.Client{Transport: homepageLibraryTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case homepageCDNMetadata + "bootstrap/resolved?specifier=latest":
			return homepageLibraryTestResponse(t, map[string]string{"version": version}), nil
		case homepageCDNMetadata + "bootstrap@" + version + "?structure=flat":
			return homepageLibraryTestResponse(t, map[string]any{"files": assets}), nil
		}
		for _, asset := range assets {
			if req.URL.String() == homepageCDNFiles+"bootstrap@"+version+asset.Name {
				downloads++
				body := []byte("fixture for " + asset.Name)
				if corrupt && asset.Name == "/LICENSE" {
					body[0] = 'F'
				}
				if cancelDownload != nil && asset.Name == "/LICENSE" {
					cancelDownload()
				}
				return homepageLibraryTestResponse(t, body), nil
			}
		}
		return nil, fmt.Errorf("unexpected request %s", req.URL)
	})}
	install := func(ctx context.Context) string {
		return runHomepageLibrary(HomepageConfig{Context: ctx, WorkspacePath: workspace}, "site", "bootstrap", "", client, nil)
	}
	if out := install(t.Context()); !strings.Contains(out, `"script_mode":"defer"`) || !strings.Contains(out, `"site/public/vendor/bootstrap/5.0.0/bootstrap.min.css"`) {
		t.Fatal(out)
	}
	if out := install(t.Context()); !strings.Contains(out, `"reused":true`) || downloads != 3 {
		t.Fatalf("second install: %s, downloads %d", out, downloads)
	}
	version, corrupt = "6.0.0", true
	if out := install(t.Context()); !strings.Contains(out, "SHA-256 verification") {
		t.Fatal(out)
	}
	base := filepath.Join(workspace, "site/public/vendor/bootstrap")
	if entries, err := os.ReadDir(base); err != nil || len(entries) != 1 || entries[0].Name() != "5.0.0" {
		t.Fatalf("failed install leaked files: %v, %v", entries, err)
	}
	corrupt = false
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cancelDownload = cancel
	if out := install(ctx); !strings.Contains(out, "context canceled") {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(base, "6.0.0")); !os.IsNotExist(err) {
		t.Fatalf("canceled release was published: %v", err)
	}
	cancelDownload = nil
	if out := install(t.Context()); !strings.Contains(out, `"script_mode":"module"`) {
		t.Fatal(out)
	}
	writeHomepageLibraryFixture(t, base, "6.0.0/bootstrap.min.css", "modified")
	if out := install(t.Context()); !strings.Contains(out, "existing library file") {
		t.Fatal(out)
	}
}

func TestHomepageLibraryAssetRoots(t *testing.T) {
	for framework, want := range map[string]string{"": ".", "vite": "public", "next": "public", "astro": "public", "nuxt": "public", "@sveltejs/kit": "static"} {
		t.Run(want+framework, func(t *testing.T) {
			directory := t.TempDir()
			writeHomepageLibraryFixture(t, directory, "index.html", "html")
			if framework != "" {
				writeHomepageLibraryFixture(t, directory, "package.json", fmt.Sprintf(`{"devDependencies":{%q:"*","example":"*"}}`, framework))
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if got, err := homepageLibraryAssetRoot(root); got != want || err != nil {
				t.Fatalf("asset root = %q, %v, want %q", got, err, want)
			}
		})
	}
}

func TestHomepageLibraryRefusesLinkedDirectories(t *testing.T) {
	workspace := t.TempDir()
	writeHomepageLibraryFixture(t, workspace, "site/index.html", "html")
	if err := os.Symlink(filepath.Join(workspace, "site"), filepath.Join(workspace, "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if root, err := openHomepageLibraryProject(HomepageConfig{WorkspacePath: workspace}, "alias"); err == nil {
		root.Close()
		t.Fatal("accepted a link to a different project")
	}
	root, err := os.OpenRoot(filepath.Join(workspace, "site"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Symlink(workspace, filepath.Join(workspace, "site/vendor")); err != nil {
		t.Fatal(err)
	}
	if err := makeHomepageLibraryDirectory(root, "vendor/three"); err == nil {
		t.Fatal("accepted a linked destination")
	}
}

func TestHomepageTailwindRecompileKeepsPinnedCSSOnFailure(t *testing.T) {
	workspace := t.TempDir()
	writeHomepageLibraryFixture(t, workspace, "site/index.html", `<p class="block">fixture</p>`)
	writeHomepageLibraryFixture(t, workspace, "site/vendor/other/asset.js", `const className = 'excluded';`)
	writeHomepageLibraryFixture(t, workspace, "site/.env", "SECRET=ignored")
	client := &http.Client{Transport: homepageLibraryTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://registry.npmjs.org/@tailwindcss%2fcli/latest" {
			t.Fatalf("unexpected Tailwind version request: %s", req.URL)
		}
		return homepageLibraryTestResponse(t, map[string]string{"version": "4.1.0"}), nil
	})}
	cfg := HomepageConfig{Context: t.Context(), WorkspacePath: workspace}
	css := ".block{display:block}"
	compile := func(cfg HomepageConfig, project *os.Root, version string) ([]byte, error) {
		const stage = ".aurago-tailwind-test"
		if version != "4.1.0" {
			t.Fatalf("compiled Tailwind %s", version)
		}
		if err := project.Mkdir(stage, 0700); err != nil {
			return nil, err
		}
		defer project.RemoveAll(stage)
		if err := stageHomepageTailwindSources(cfg, project, stage); err != nil {
			return nil, err
		}
		sources, err := fsReadNames(project, stage+"/source")
		if err != nil || sources != "index.html" {
			t.Fatalf("staged outside the source boundary: %s, %v", sources, err)
		}
		return []byte(css), nil
	}
	if out := runHomepageLibrary(cfg, "site", "tailwindcss", "", client, compile); !strings.Contains(out, `"stylesheet":"vendor/tailwindcss/4.1.0/tailwind.css"`) {
		t.Fatal(out)
	}
	css = ".hidden{display:none}"
	if out := runHomepageLibrary(cfg, "site", "tailwindcss", "4.1.0", nil, compile); !strings.Contains(out, `"reused":false`) {
		t.Fatal(out)
	}
	badCompile := func(HomepageConfig, *os.Root, string) ([]byte, error) {
		return nil, fmt.Errorf("fixture compile failed")
	}
	if out := runHomepageLibrary(cfg, "site", "tailwindcss", "4.1.0", nil, badCompile); !strings.Contains(out, "fixture compile failed") {
		t.Fatal(out)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "site/vendor/tailwindcss/4.1.0/tailwind.css"))
	if err != nil || string(data) != css {
		t.Fatalf("failed compile changed the stylesheet: %s, %v", data, err)
	}
}

func fsReadNames(root *os.Root, dir string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root.Name(), dir))
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return strings.Join(names, ","), err
}

func TestHomepageLibrariesOfficialCDN(t *testing.T) {
	if os.Getenv("AURAGO_HOMEPAGE_CDN_SMOKE") != "1" {
		t.Skip("set AURAGO_HOMEPAGE_CDN_SMOKE=1 to check the live Homepage CDN")
	}
	workspace := t.TempDir()
	writeHomepageLibraryFixture(t, workspace, "site/index.html", "<h1>Homepage CDN smoke</h1>")
	installed := make(map[string]map[string]any)
	for library := range homepageLibraries {
		t.Run(library, func(t *testing.T) {
			out := InstallHomepageLibrary(HomepageConfig{Context: t.Context(), WorkspacePath: workspace}, "site", library)
			if !strings.Contains(out, `"status":"ok"`) {
				t.Fatal(out)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			installed[library] = result
			t.Log(out)
		})
	}
	if chrome := os.Getenv("CHROME_BIN"); !t.Failed() && chrome != "" {
		t.Run("sandboxed_offline_preview", func(t *testing.T) {
			smokeHomepageLibraryPreview(t, chrome, workspace, installed)
		})
	}
}

func smokeHomepageLibraryPreview(t *testing.T, chrome, workspace string, installed map[string]map[string]any) {
	t.Helper()
	var head strings.Builder
	for _, library := range []string{"bootstrap", "alpinejs", "htmx", "chartjs", "swiper"} {
		result := installed[library]
		if css, ok := result["stylesheet"].(string); ok {
			fmt.Fprintf(&head, `<link rel="stylesheet" href="%s">`, css)
		}
		mode := "defer"
		if result["script_mode"] == "module" {
			mode = `type="module"`
		}
		fmt.Fprintf(&head, `<script %s src="%s"></script>`, mode, result["script"])
	}
	writeHomepageLibraryFixture(t, workspace, "site/previewed.html", fmt.Sprintf(`<!doctype html><html><head><title>Local libraries</title>%s</head>
<body><p class="d-none">Hidden by Bootstrap</p><div class="swiper"></div>
<script type="module">
import { Vector3 } from './%s';
window.addEventListener('load', () => {
  const libraries = !!(window.Alpine && window.htmx && window.Chart && window.Swiper);
  const css = getComputedStyle(document.querySelector('p')).display === 'none' && getComputedStyle(document.querySelector('.swiper')).position === 'relative';
  window.parent.postMessage(libraries && css && new Vector3(1, 2, 2).length() === 3 ? 'passed' : 'failed', '*');
});
</script></body></html>`, head.String(), installed["three"]["script"]))
	writeHomepageLibraryFixture(t, workspace, "site/index.html", `<!doctype html><html><head><title>Studio preview shell</title></head><body>
<script>window.addEventListener('message', event => { document.body.dataset.smoke = event.data; });</script>
<iframe sandbox="allow-scripts allow-forms" src="previewed.html"></iframe></body></html>`)
	snapshot, _, err := homepageLocalSnapshot(HomepageConfig{WorkspacePath: workspace}, "site", ".")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	url, _, err := startPythonServer(port, snapshot.root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		activePythonServerMu.Lock()
		defer activePythonServerMu.Unlock()
		if activePythonServerCmd != nil {
			_ = activePythonServerCmd.Process.Kill()
			_ = activePythonServerCmd.Wait()
			activePythonServerCmd = nil
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, chrome, "--headless", "--disable-gpu", "--no-sandbox", "--disable-background-networking", "--no-proxy-server",
		"--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE localhost", "--user-data-dir="+t.TempDir(), "--virtual-time-budget=5000", "--dump-dom", url).CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`data-smoke="passed"`)) {
		t.Fatalf("offline preview did not load its local modules and styles: %v\n%s", err, output)
	}
}
