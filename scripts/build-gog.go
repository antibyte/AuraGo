//go:build ignore

// Build the pinned, patched optional helper before building AuraGo releases.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const commit = "3b5122f4c81c5df6df38ee48e01327f1034364ee"
const version = "0.43.0-aurago1"

type artifact struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archive_sha256"`
	BinarySHA256  string `json:"binary_sha256"`
}

func fatal(e error) {
	if e != nil {
		panic(e)
	}
}
func run(dir string, env []string, name string, args ...string) []byte {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = os.Stderr
	fatal(c.Run())
	return bytes.TrimSpace(out.Bytes())
}
func sum(data []byte) string { v := sha256.Sum256(data); return hex.EncodeToString(v[:]) }
func main() {
	defer func() {
		if v := recover(); v != nil {
			fmt.Fprintln(os.Stderr, v)
			os.Exit(1)
		}
	}()
	build()
}
func build() {
	source := flag.String("source", "disposable/_gog/build-source", "clean checkout/cache directory")
	out := flag.String("out", "deploy/gog", "package destination")
	flag.Parse()
	root, e := os.Getwd()
	fatal(e)
	src, e := filepath.Abs(*source)
	fatal(e)
	if _, e = os.Stat(filepath.Join(src, ".git")); os.IsNotExist(e) {
		fatal(os.MkdirAll(filepath.Dir(src), 0700))
		run(root, nil, "git", "clone", "--depth", "1", "--branch", "v0.43.0", "https://github.com/openclaw/gogcli.git", src)
	}
	if string(run(src, nil, "git", "rev-parse", "HEAD")) != commit {
		fatal(fmt.Errorf("unexpected gog source commit"))
	}
	patch := filepath.Join(root, "internal", "gog", "build", "single-attempt.patch")
	if len(run(src, nil, "git", "status", "--porcelain")) != 0 {
		fatal(fmt.Errorf("gog build source must be clean"))
	}
	run(src, nil, "git", "apply", patch)
	defer func() {
		run(src, nil, "git", "apply", "--reverse", patch)
		_ = os.Remove(filepath.Join(src, "internal", "googleapi", "aurago_single_attempt_test.go"))
		_ = os.Remove(filepath.Join(src, "gog"))
		_ = os.Remove(filepath.Join(src, "gog.exe"))
	}()
	test, e := os.ReadFile(filepath.Join(root, "internal", "gog", "build", "single_attempt_test.go.txt"))
	fatal(e)
	fatal(os.WriteFile(filepath.Join(src, "internal", "googleapi", "aurago_single_attempt_test.go"), test, 0600))
	env := []string{"GOTOOLCHAIN=go1.27.2", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=", "GOEXPERIMENT=", "GOAMD64=v1", "GOARM64=v8.0"}
	fmt.Println("Verifying single-attempt transport and SDK uploads")
	run(src, env, "go", "test", "./internal/googleapi", "-run", "TestAuraGo", "-count=1")
	fatal(os.MkdirAll(*out, 0755))
	var artifacts []artifact
	for _, target := range []string{"linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64", "darwin/amd64", "darwin/arm64"} {
		parts := strings.Split(target, "/")
		name := "gog"
		if parts[0] == "windows" {
			name += ".exe"
		}
		binary := filepath.Join(src, name)
		fmt.Println("Building gog", target)
		buildEnv := append(append([]string{}, env...), "GOOS="+parts[0], "GOARCH="+parts[1])
		run(src, buildEnv, "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid= -X github.com/openclaw/gogcli/internal/cmd.version="+version+" -X github.com/openclaw/gogcli/internal/cmd.commit="+commit+" -X github.com/openclaw/gogcli/internal/cmd.date=2026-10-01T00:00:00Z", "-o", binary, "./cmd/gog")
		data, e := os.ReadFile(binary)
		fatal(e)
		fatal(os.Remove(binary))
		var archive bytes.Buffer
		z := zip.NewWriter(&archive)
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0700)
		h.Modified = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		w, e := z.CreateHeader(h)
		fatal(e)
		_, e = w.Write(data)
		fatal(e)
		license, e := os.ReadFile(filepath.Join(root, "internal", "gog", "build", "LICENSE.gog"))
		fatal(e)
		license = bytes.ReplaceAll(license, []byte("\r\n"), []byte("\n"))
		lh := &zip.FileHeader{Name: "LICENSE.gog", Method: zip.Deflate, Modified: h.Modified}
		lh.SetMode(0644)
		lw, e := z.CreateHeader(lh)
		fatal(e)
		_, e = lw.Write(license)
		fatal(e)
		fatal(z.Close())
		filename := "gog-" + version + "-" + parts[0] + "-" + parts[1] + ".zip"
		fatal(os.WriteFile(filepath.Join(*out, filename), archive.Bytes(), 0644))
		artifacts = append(artifacts, artifact{parts[0], parts[1], filename, sum(archive.Bytes()), sum(data)})
	}
	manifest, e := json.MarshalIndent(artifacts, "", "  ")
	fatal(e)
	manifest = append(manifest, '\n')
	fatal(os.WriteFile(filepath.Join(root, "internal", "gog", "build", "artifacts.json"), manifest, 0644))
	fatal(os.WriteFile(filepath.Join(*out, "artifacts.json"), manifest, 0644))
	fmt.Println("Verified packages and embedded manifest written")
}
