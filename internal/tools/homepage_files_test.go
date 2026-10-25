package tools

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHomepageFileDockerTransfersPreserveLargeContent(t *testing.T) {
	for _, size := range []int{76434, 256 * 1024, maxHomepageWriteFileSize} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			configureDockerSecurityTestPermissions(t, false)
			prefix, suffix := "<html>\n<!-- INSERT HERE -->\n", "\n</html>"
			original := prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix
			stored := original
			writes := 0
			dockerCfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/containers/"+homepageContainerName+"/archive" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.Method == http.MethodGet {
					if r.URL.Query().Get("path") != "/workspace/news/(group)/[slug]/Über uns.html" {
						t.Errorf("read path = %q", r.URL.Query().Get("path"))
					}
					tw := tar.NewWriter(w)
					if err := tw.WriteHeader(&tar.Header{Name: "Über uns.html", Mode: 0644, Size: int64(len(stored))}); err != nil {
						t.Error(err)
					}
					_, _ = io.WriteString(tw, stored)
					_ = tw.Close()
					return
				}
				if r.Method != http.MethodPut || r.URL.Query().Get("path") != "/workspace/news/(group)/[slug]" || r.Header.Get("Content-Type") != "application/x-tar" {
					t.Errorf("unexpected archive request: %s %s", r.Method, r.URL)
				}
				if r.URL.Query().Get("copyUIDGID") != "true" || r.URL.Query().Get("noOverwriteDirNonDir") != "true" {
					t.Errorf("archive upload must use the container user and preserve directories: %s", r.URL)
				}
				tr := tar.NewReader(r.Body)
				header, err := tr.Next()
				if err != nil || header.Name != "Über uns.html" {
					t.Errorf("archive header = %+v, %v", header, err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				data, err := io.ReadAll(tr)
				if err != nil {
					t.Error(err)
				}
				stored = string(data)
				writes++
			})
			oldExec, oldInternal := homepageDockerExecFunc, homepageDockerExecInternalFunc
			t.Cleanup(func() { homepageDockerExecFunc, homepageDockerExecInternalFunc = oldExec, oldInternal })
			homepageDockerExecInternalFunc = func(DockerConfig, string, string, string, []string) string {
				return `{"status":"ok","exit_code":0,"output":""}`
			}
			homepageDockerExecFunc = func(_ DockerConfig, _, command, _ string) string {
				if command != "mkdir -p '/workspace/news/(group)/[slug]'" {
					t.Errorf("unquoted upload directory: %s", command)
				}
				if len(command) >= 128*1024 {
					return `{"status":"error","error":"Argument list too long"}`
				}
				return `{"status":"ok","exit_code":0,"output":""}`
			}
			dockerAvailabilityMu.Lock()
			dockerAvailabilityResults[dockerCfg.Host] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
			dockerAvailabilityMu.Unlock()
			t.Cleanup(func() { invalidateDockerAvailabilityCache(dockerCfg.Host) })
			cfg := HomepageConfig{DockerHost: dockerCfg.Host}
			result := HomepageWriteFile(cfg, "news/(group)/[slug]/Über uns.html", original, slogDiscard())
			if !strings.Contains(result, `"status":"ok"`) || writes != 1 || stored != original {
				t.Fatalf("large write lost content: result=%s writes=%d bytes=%d want=%d", result, writes, len(stored), len(original))
			}
			var read struct {
				Output string `json:"output"`
			}
			result = HomepageReadFile(cfg, "news/(group)/[slug]/Über uns.html", slogDiscard())
			if err := json.Unmarshal([]byte(result), &read); err != nil || read.Output != original {
				t.Fatalf("large read lost content: bytes=%d want=%d error=%v", len(read.Output), len(original), err)
			}
			result = HomepageEditFile(cfg, "news/(group)/[slug]/Über uns.html", "str_replace", "INSERT HERE", "NEW ARTICLE", "", "", 0, 0, slogDiscard())
			if !strings.Contains(result, `"status":"ok"`) || writes != 2 || stored != strings.Replace(original, "INSERT HERE", "NEW ARTICLE", 1) {
				t.Fatalf("large edit lost content: result=%s writes=%d bytes=%d", result, writes, len(stored))
			}
		})
	}
}

func TestHomepageEditFileLocalUsesWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "news", "pages", "index.html")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("<!-- ARTICLE -->\n<h2>old</h2>"), 0644); err != nil {
		t.Fatal(err)
	}
	result := HomepageEditFile(HomepageConfig{DockerHost: "tcp://127.0.0.1:1", WorkspacePath: workspace}, "news/pages/index.html", "str_replace", "old", "new", "", "", 0, 0, slogDiscard())
	data, err := os.ReadFile(file)
	if !strings.Contains(result, `"status":"success"`) || err != nil || !bytes.Contains(data, []byte("<h2>new</h2>")) {
		t.Fatalf("local edit failed: result=%s data=%q error=%v", result, data, err)
	}
}

func TestHomepageEditFileRejectsUnsafeInputWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		content  string
		typeflag byte
		broken   bool
		marker   string
	}{
		{name: "duplicate marker", content: "<!-- NEWS -->\nold\n<!-- NEWS -->", marker: "<!-- NEWS -->"},
		{name: "truncated archive", content: "old", broken: true, marker: "old"},
		{name: "symbolic link", typeflag: tar.TypeSymlink, marker: "old"},
		{name: "oversized file", content: strings.Repeat("x", maxHomepageWriteFileSize+1), marker: "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configureDockerSecurityTestPermissions(t, false)
			var archive bytes.Buffer
			tw := tar.NewWriter(&archive)
			if err := tw.WriteHeader(&tar.Header{Name: "index.html", Mode: 0644, Size: int64(len(tc.content)), Typeflag: tc.typeflag, Linkname: "other.html"}); err != nil {
				t.Fatal(err)
			}
			_, _ = io.WriteString(tw, tc.content)
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			data := archive.Bytes()
			if tc.broken {
				data = data[:513]
			}
			writes := 0
			dockerCfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/archive") {
					_, _ = w.Write(data)
					return
				}
				if r.Method == http.MethodPut {
					writes++
				}
				w.WriteHeader(http.StatusBadRequest)
			})
			dockerAvailabilityMu.Lock()
			dockerAvailabilityResults[dockerCfg.Host] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
			dockerAvailabilityMu.Unlock()
			t.Cleanup(func() { invalidateDockerAvailabilityCache(dockerCfg.Host) })
			result := HomepageEditFile(HomepageConfig{DockerHost: dockerCfg.Host}, "news/index.html", "insert_after", "", "", tc.marker, "new article", 0, 0, slogDiscard())
			if !strings.Contains(result, `"status":"error"`) || writes != 0 {
				t.Fatalf("unsafe edit accepted or wrote a file: %s writes=%d", result, writes)
			}
		})
	}
}

func TestHomepageWriteFileRepairsProjectBeforeWriting(t *testing.T) {
	oldInternalExec := homepageDockerExecInternalFunc
	oldExec := homepageDockerExecFunc
	defer func() {
		homepageDockerExecInternalFunc = oldInternalExec
		homepageDockerExecFunc = oldExec
	}()

	projectChecks := 0
	repairedProject := false
	writes := 0
	dockerHost := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		if !repairedProject || r.Method != http.MethodPut || r.URL.Query().Get("path") != "/workspace/ki-news/src" {
			t.Errorf("archive upload before permission repair or to wrong directory: %s %s", r.Method, r.URL)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		writes++
	}).Host
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	defer invalidateDockerAvailabilityCache(dockerHost)

	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		switch {
		case strings.Contains(cmd, "'/workspace/ki-news'/.aurago-project-write-test"):
			projectChecks++
			if projectChecks == 1 {
				return `{"status":"error","exit_code":1,"output":"touch: Permission denied"}`
			}
			return `{"status":"ok","exit_code":0,"output":""}`
		case strings.Contains(cmd, "id -u"):
			return `{"status":"ok","exit_code":0,"output":"1001:1001\n"}`
		case user == "0:0" && strings.Contains(cmd, "chown -R 1001:1001 '/workspace/ki-news'"):
			repairedProject = true
			return `{"status":"ok","exit_code":0,"output":""}`
		default:
			t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
			return `{"status":"error","exit_code":1}`
		}
	}

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		if !repairedProject {
			return `{"status":"error","exit_code":1,"output":"/bin/sh: 1: cannot create /workspace/ki-news/src/main.ts: Permission denied"}`
		}
		if strings.Contains(command, `src\main.ts`) {
			t.Fatalf("write command used Windows separators in container path: %s", command)
		}
		return `{"status":"ok","exit_code":0,"output":""}`
	}

	got := HomepageWriteFile(HomepageConfig{DockerHost: dockerHost}, "ki-news/src/main.ts", "console.log('ok')", slogDiscard())
	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected write_file to succeed after repair, got: %s", got)
	}
	if projectChecks < 2 || !repairedProject || writes != 1 {
		t.Fatalf("expected project writability repair and upload, checks=%d repaired=%v writes=%d", projectChecks, repairedProject, writes)
	}
}

func TestHomepageWriteFileReportsDockerOutputOnFailure(t *testing.T) {
	oldInternalExec := homepageDockerExecInternalFunc
	oldExec := homepageDockerExecFunc
	defer func() {
		homepageDockerExecInternalFunc = oldInternalExec
		homepageDockerExecFunc = oldExec
	}()

	dockerHost := "unit-test-write-failure-message"
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{available: true, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
	defer invalidateDockerAvailabilityCache(dockerHost)

	homepageDockerExecInternalFunc = func(cfg DockerConfig, containerID, cmd, user string, env []string) string {
		if strings.Contains(cmd, "'/workspace/ki-news'/.aurago-project-write-test") {
			return `{"status":"ok","exit_code":0,"output":""}`
		}
		t.Fatalf("unexpected docker exec call user=%q cmd=%q", user, cmd)
		return `{"status":"error","exit_code":1}`
	}
	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"status":"error","exit_code":1,"output":"/bin/sh: 1: cannot create /workspace/ki-news/src/main.ts: Permission denied"}`
	}

	got := HomepageWriteFile(HomepageConfig{DockerHost: dockerHost}, "ki-news/src/main.ts", "console.log('ok')", slogDiscard())
	if !strings.Contains(got, "Permission denied") {
		t.Fatalf("expected write_file error to include Docker output, got: %s", got)
	}
}

func TestHomepageWriteFileRejectsEmptyContentWithoutTruncating(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "ki-news", "src", "App.tsx")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("export default function App() { return 'ok' }\n"), 0644); err != nil {
		t.Fatalf("WriteFile seed: %v", err)
	}

	got := HomepageWriteFile(HomepageConfig{WorkspacePath: workspace}, "ki-news/src/App.tsx", "", slogDiscard())
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "content is empty") {
		t.Fatalf("expected empty-content rejection, got: %s", got)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "export default function App() { return 'ok' }\n" {
		t.Fatalf("file was modified despite rejected empty write: %q", string(data))
	}
}
