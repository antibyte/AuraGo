package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
	"aurago/internal/tools"
)

// liveCYDBuilder returns an adapter on the real engine (DOCKER_HOST or the
// platform default) with unique container and volume names, removed after
// the test.
func liveCYDBuilder(t *testing.T, image string, idle time.Duration) *cydDockerBuilder {
	t.Helper()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	var suffix [4]byte
	_, _ = rand.Read(suffix[:])
	id := hex.EncodeToString(suffix[:])
	b := newCYDDockerBuilder(tools.DockerConfig{Host: os.Getenv("DOCKER_HOST")},
		cydBuilderSettings{Image: image, MemoryMB: 2048, CPUCores: 2, IdleStop: idle},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.containerName = "aurago-cyd-builder-it-" + id
	b.cacheVolume = "aurago-cyd-cache-it-" + id
	b.workVolume = "aurago-cyd-work-it-" + id
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		b.mu.Lock()
		b.closed = true
		if b.idleTimer != nil {
			b.idleTimer.Stop()
		}
		b.mu.Unlock()
		_, _, _ = b.request(ctx, http.MethodDelete, "/containers/"+url.PathEscape(b.containerName)+"?force=true", nil)
		for _, v := range []string{b.cacheVolume, b.workVolume} {
			_, _, _ = b.request(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(v)+"?force=true", nil)
		}
	})
	return b
}

func liveContainerRunning(t *testing.T, b *cydDockerBuilder) bool {
	t.Helper()
	info, err := b.inspectContainer(context.Background())
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	return info != nil && info.State.Running
}

// TestCYDBuilderStubMechanicsLive drives the adapter against a real engine
// with the stub image of testdata/cyd_builder_stub: upload, read-only root,
// no network, uid 1000, streamed log, artefacts, exit codes, cancel, idle
// stop and addr2line. Opt-in: CYD_BUILDER_STUB_IMAGE=<local stub tag>.
func TestCYDBuilderStubMechanicsLive(t *testing.T) {
	image := os.Getenv("CYD_BUILDER_STUB_IMAGE")
	if image == "" {
		t.Skip("set CYD_BUILDER_STUB_IMAGE to the locally built stub image (see testdata/cyd_builder_stub/Dockerfile)")
	}
	b := liveCYDBuilder(t, image, 3*time.Second)
	ctx := context.Background()
	if st := b.Status(ctx); !st.DockerAvailable || !st.Present {
		t.Fatalf("status = %+v; build the stub image first", st)
	}

	var mu sync.Mutex
	var firstLine time.Time
	var lines []string
	out, err := b.Build(ctx, cydcreator.BuildRequest{BuildID: "ccb_live0001", Env: "esp32-2432s028r", CacheKey: "ccp_live-esp32-2432s028r",
		Tree: stubTree(0, 5), Timeout: time.Minute}, func(line string) {
		mu.Lock()
		defer mu.Unlock()
		if firstLine.IsZero() {
			firstLine = time.Now()
		}
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	log := strings.Join(lines, "\n")
	for _, want := range []string{"uid=1000 gid=1000", "rootfs read-only", "net: lo", "cache ok", "line 4", "warning: stub warning"} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
	if out.ExitCode != 0 || len(out.Files) != 3 || !out.Result.OK {
		t.Fatalf("output = exit %d, %d files, result %+v", out.ExitCode, len(out.Files), out.Result)
	}
	for name, data := range out.Files {
		if want := out.Result.Files[name].SHA256; want == "" || want != sha256HexForTest(data) {
			t.Errorf("%s hash %s does not match result.json %q", name, sha256HexForTest(data), want)
		}
	}

	for _, exit := range []int{1, 3} {
		out, err := b.Build(ctx, cydcreator.BuildRequest{BuildID: fmt.Sprintf("ccb_live000%d", exit+1), Env: "esp32-2432s028r",
			CacheKey: "k", Tree: stubTree(exit, 0)}, nil)
		if err != nil || out.ExitCode != exit || len(out.Files) != 0 {
			t.Fatalf("exit %d: %+v, %v", exit, out, err)
		}
		if exit == 3 && out.Result.Exit != 3 {
			t.Fatalf("exit 3 result = %+v", out.Result)
		}
	}

	frames, err := b.Addr2Line(ctx, cydcreator.Addr2LineRequest{BuildID: "ccb_live0001", Chip: "esp32", ELF: []byte("ELF"), Addresses: []string{"0x400d1234"}})
	if err != nil || len(frames) != 1 || frames[0].Function != "stub_esp32()" {
		t.Fatalf("addr2line = %+v, %v", frames, err)
	}

	slow := stubTree(0, 1)
	slow["stub.json"] = []byte(`{"exit":0,"lines":1,"sleep":60}`)
	cctx, cancel := context.WithCancel(ctx)
	started := make(chan struct{})
	var once sync.Once
	errCh := make(chan error, 1)
	go func() {
		_, err := b.Build(cctx, cydcreator.BuildRequest{BuildID: "ccb_live0009", Env: "esp32-2432s028r", CacheKey: "k", Tree: slow},
			func(string) { once.Do(func() { close(started) }) })
		errCh <- err
	}()
	select {
	case <-started:
	case err := <-errCh:
		cancel()
		t.Fatalf("slow build ended before its first log line: %v", err)
	}
	cancelAt := time.Now()
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled build = %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancel did not end the build")
	}
	if took := time.Since(cancelAt); took > 20*time.Second {
		t.Fatalf("cancel took %v", took)
	}
	if info, err := b.inspectContainer(context.Background()); err != nil || info != nil {
		t.Fatalf("cancel must stop and remove the container (the Engine cannot kill a single exec): %+v, %v", info, err)
	}

	if out, err := b.Build(ctx, cydcreator.BuildRequest{BuildID: "ccb_live0010", Env: "esp32-2432s028r", CacheKey: "k", Tree: stubTree(0, 0)}, nil); err != nil || out.ExitCode != 0 {
		t.Fatalf("build after cancel = %+v, %v", out, err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for liveContainerRunning(t, b) {
		if time.Now().After(deadline) {
			t.Fatal("the idle container was not stopped")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// TestCYDBuilderIntegration compiles a generated project for every board
// profile in the real toolchain image and decodes one address. Opt-in:
// CYD_BUILDER_IT=1; CYD_BUILDER_IT_IMAGE overrides the image (default: the
// cyd_creator.builder_image default). The image is pulled when missing.
func TestCYDBuilderIntegration(t *testing.T) {
	if os.Getenv("CYD_BUILDER_IT") != "1" {
		t.Skip("set CYD_BUILDER_IT=1 to compile every board profile in the toolchain image")
	}
	image := os.Getenv("CYD_BUILDER_IT_IMAGE")
	if image == "" {
		image = config.DefaultCYDCreatorBuilderImage
	}
	b := liveCYDBuilder(t, image, 0)
	ctx := context.Background()
	if st := b.Status(ctx); !st.DockerAvailable {
		t.Fatalf("Docker unavailable: %+v", st)
	} else if !st.Present {
		if err := b.Pull(ctx); err != nil {
			t.Fatalf("pull %s: %v", image, err)
		}
	}
	boards, err := cydcreator.Boards()
	if err != nil {
		t.Fatal(err)
	}
	for i, board := range boards {
		t.Run(board.ID, func(t *testing.T) {
			p, err := cydcreator.NewProject("IT "+board.ID, board.ID, "clear", "en", "", &cydcreator.SeqIDs{})
			if err != nil {
				t.Fatal(err)
			}
			gen, err := cydcreator.Generate(cydcreator.GenerateInput{Project: p, ProjectID: "ccp_it", AppFiles: cydcreator.ScaffoldAppFiles(p)})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			bctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
			defer cancel()
			buildID := fmt.Sprintf("ccb_it%04d", i)
			out, err := b.Build(bctx, cydcreator.BuildRequest{BuildID: buildID, Env: board.PIOEnv(), CacheKey: "ccp_it-" + board.ID,
				Tree: gen.Files, Timeout: 20 * time.Minute}, func(line string) { t.Log(line) })
			if err != nil || out.ExitCode != 0 {
				t.Fatalf("build: exit %d, %v", out.ExitCode, err)
			}
			for _, name := range []string{cydcreator.ArtifactMergedBin, cydcreator.ArtifactFirmwareBin, cydcreator.ArtifactFirmwareELF} {
				if len(out.Files[name]) == 0 || out.Result.Files[name].SHA256 != sha256HexForTest(out.Files[name]) {
					t.Fatalf("%s missing or not matching result.json", name)
				}
			}
			// cyd-build copies the converted fonts and fonts.json to out/fonts/ (kept for the ZIP export).
			if out.Result.Fonts["fonts.json"].SHA256 != sha256HexForTest(gen.Files["src/generated/fonts.json"]) || len(out.Result.Fonts) < 2 {
				t.Fatalf("result.json fonts = %+v", out.Result.Fonts)
			}
			for name, want := range out.Result.Fonts {
				if data := out.Files[cydcreator.ArtifactFontsDir+"/"+name]; len(data) == 0 || want.SHA256 != sha256HexForTest(data) {
					t.Fatalf("font %s missing or not matching result.json", name)
				}
			}
			addr := "0x400d0000"
			if board.Chip == cydcreator.ChipESP32S3 {
				addr = "0x42000000"
			}
			if frames, err := b.Addr2Line(bctx, cydcreator.Addr2LineRequest{BuildID: buildID, Chip: board.Chip,
				ELF: out.Files[cydcreator.ArtifactFirmwareELF], Addresses: []string{addr}}); err != nil || len(frames) != 1 {
				t.Fatalf("addr2line: %+v, %v", frames, err)
			}
		})
	}
}
