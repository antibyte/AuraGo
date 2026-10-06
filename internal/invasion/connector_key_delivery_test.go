package invasion

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func TestDockerConnectorDeployMarksFailuresBeforeTheConfigUpload(t *testing.T) {
	streamError := "{\"errorDetail\":{\"message\":\"unexpected EOF\"},\"error\":\"unexpected EOF\"}\n"
	upToDate := "{\"status\":\"Status: Image is up to date\"}\n"
	cases := []struct {
		name                string
		pull, image, create http.HandlerFunc
		wantErr             string
	}{
		{name: "pull HTTP status", pull: respond(http.StatusNotFound, `{"message":"manifest unknown"}`),
			wantErr: `failed to pull image: pull failed with HTTP 404: {"message":"manifest unknown"}`},
		{name: "pull stream error without the image", pull: respond(http.StatusOK, streamError), image: respond(http.StatusNotFound, `{"message":"no such image"}`),
			wantErr: "failed to pull image: pull ghcr.io/antibyte/aurago:latest: "},
		{name: "pull stream error with a failing image check", pull: respond(http.StatusOK, streamError), image: respond(http.StatusInternalServerError, "engine busy"),
			wantErr: "failed to pull image: pull ghcr.io/antibyte/aurago:latest: "},
		{name: "container create refused", pull: respond(http.StatusOK, upToDate), create: respond(http.StatusConflict, `{"message":"name in use"}`),
			wantErr: "container creation failed (409)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var archives atomic.Int64
			handlers := map[string]http.HandlerFunc{
				"/images/create": tc.pull,
				"/containers/": func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPut {
						archives.Add(1)
					}
					w.WriteHeader(http.StatusNoContent) // stop, rename and remove of the old egg
				},
			}
			if tc.image != nil {
				handlers["/images/ghcr.io/antibyte/aurago:latest/json"] = tc.image
			}
			if tc.create != nil {
				handlers["/containers/create"] = tc.create
			}
			ts := mockDockerAPI(t, handlers)
			defer ts.Close()

			err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) {
				t.Fatalf("Deploy error = %v, want it to start with %q (hatch_error text must not change)", err, tc.wantErr)
			}
			if !errors.Is(err, ErrEggConfigNotDelivered) {
				t.Fatalf("Deploy error %v is not marked ErrEggConfigNotDelivered although no config was uploaded", err)
			}
			if n := archives.Load(); n != 0 {
				t.Fatalf("Deploy uploaded the config %d times before failing, want 0", n)
			}
		})
	}
}

func TestDockerConnectorDeployMarksAnInvalidNestID(t *testing.T) {
	var requests atomic.Int64
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{"/": func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }})
	defer ts.Close()
	nest := nestForMock(ts)
	nest.ID = "short"
	err := (&DockerConnector{}).Deploy(context.Background(), nest, nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
	if !errors.Is(err, ErrEggConfigNotDelivered) || requests.Load() != 0 {
		t.Fatalf("Deploy = %v with %d requests, want a marked error and no Engine request", err, requests.Load())
	}
}

func TestDockerConnectorDeployDoesNotMarkFailuresFromTheConfigUploadOn(t *testing.T) {
	ok := respond(http.StatusNoContent, "")
	refuse := respond(http.StatusInternalServerError, `{"message":"engine refused"}`)
	dropConnection := func(w http.ResponseWriter, r *http.Request) {
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.Close()
		}
	}
	cases := []struct {
		name           string
		archive, start http.HandlerFunc
		wantErr        string
	}{
		{"config upload refused", refuse, ok, "failed to copy config to container"},
		{"config upload connection lost", dropConnection, ok, "failed to copy config to container"},
		{"start refused", ok, refuse, "container start failed (500)"},
		{"start connection lost", ok, dropConnection, "failed to start container"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := mockDockerAPI(t, map[string]http.HandlerFunc{
				"/images/create":     respond(http.StatusOK, "{\"status\":\"Status: Image is up to date\"}\n"),
				"/containers/create": respond(http.StatusCreated, `{"Id":"abc123"}`),
				"/containers/": func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodPut:
						tc.archive(w, r)
					case strings.HasSuffix(r.URL.Path, "/start"):
						tc.start(w, r)
					default:
						w.WriteHeader(http.StatusNoContent)
					}
				},
			})
			defer ts.Close()
			err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) {
				t.Fatalf("Deploy error = %v, want it to start with %q", err, tc.wantErr)
			}
			if errors.Is(err, ErrEggConfigNotDelivered) {
				t.Fatalf("Deploy marked %v although the Engine may already hold the new configuration", err)
			}
		})
	}
}

type sshDeployStep struct{ cmd, input, remotePath string }

func sshDeployStepName(s sshDeployStep) string {
	switch {
	case strings.HasSuffix(s.remotePath, "/aurago"):
		return "binary"
	case strings.HasSuffix(s.remotePath, "/resources.dat"):
		return "resources"
	case strings.Contains(s.cmd, "cp -a"):
		return "backup"
	case strings.HasPrefix(s.cmd, "mkdir -p"):
		return "mkdir"
	case strings.HasPrefix(s.cmd, "chmod +x"):
		return "chmod"
	case strings.Contains(s.cmd, "tar -xzf"):
		return "unpack"
	case s.cmd == "bash -s" && strings.Contains(s.input, "/config.yaml'"):
		return "config"
	case s.cmd == "bash -s" && strings.Contains(s.input, "/data/vault.enc'"):
		return "vault"
	case s.cmd == "bash -s" && strings.Contains(s.input, "/.env'"):
		return "env"
	case strings.Contains(s.cmd, "nohup ./aurago"):
		return "start"
	}
	return "unknown: " + s.cmd
}

// fakeSSHDeploy replaces the SSH connector's remote calls, records every step
// and fails the steps failAt selects.
func fakeSSHDeploy(t *testing.T, failAt func(name string) bool) *[]string {
	t.Helper()
	var steps []string
	priorCommand, priorTransfer := sshRemoteCommand, sshTransferFile
	t.Cleanup(func() { sshRemoteCommand, sshTransferFile = priorCommand, priorTransfer })
	sshRemoteCommand = func(ctx context.Context, host string, port int, user string, secret []byte, cmd string, input ...io.Reader) (string, error) {
		step := sshDeployStep{cmd: cmd}
		if len(input) > 0 && input[0] != nil {
			data, _ := io.ReadAll(input[0])
			step.input = string(data)
		}
		name := sshDeployStepName(step)
		steps = append(steps, name)
		if failAt(name) {
			return "", errors.New("simulated SSH failure")
		}
		return "ok\n", nil
	}
	sshTransferFile = func(ctx context.Context, host string, port int, user string, secret []byte, localPath, remotePath, direction string, allowedRoot ...string) error {
		name := sshDeployStepName(sshDeployStep{remotePath: remotePath})
		steps = append(steps, name)
		if failAt(name) {
			return errors.New("simulated SFTP failure")
		}
		return nil
	}
	return &steps
}

func sshDeployTestNest() NestRecord {
	return NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: "10.0.0.5", Port: 22, Username: "deploy", DeployMethod: "ssh"}
}

func sshDeployTestPayload() EggDeployPayload {
	return EggDeployPayload{
		BinaryPath: "aurago", ConfigYAML: []byte("egg_mode:\n  shared_key: hatch-key\n"),
		ResourcesPkg: "resources.dat", IncludeVault: true, VaultData: []byte("vault"),
		MasterKey: strings.Repeat("ab", 32),
	}
}

func TestSSHConnectorDeployStepOrderIsUnchanged(t *testing.T) {
	steps := fakeSSHDeploy(t, func(string) bool { return false })
	if err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), []byte("secret"), sshDeployTestPayload()); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	want := "backup,mkdir,binary,chmod,config,resources,unpack,vault,env,start"
	if got := strings.Join(*steps, ","); got != want {
		t.Fatalf("steps = %s, want %s", got, want)
	}
}

func TestSSHConnectorDeployMarksOnlyFailuresBeforeTheConfigWrite(t *testing.T) {
	cases := []struct {
		step, wantErr string
		marked        bool
	}{
		{"backup", "failed to backup existing deployment", true},
		{"mkdir", "failed to create directories", true},
		{"binary", "failed to transfer binary", true},
		{"chmod", "failed to chmod binary", true},
		{"config", "failed to write config", false},
		{"resources", "failed to transfer resources", false},
		{"unpack", "failed to unpack resources", false},
		{"vault", "failed to write vault", false},
		{"env", "failed to write .env", false},
		{"start", "failed to start egg process", false},
	}
	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			steps := fakeSSHDeploy(t, func(name string) bool { return name == tc.step })
			err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), []byte("secret"), sshDeployTestPayload())
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) {
				t.Fatalf("Deploy error = %v, want it to start with %q", err, tc.wantErr)
			}
			if got := errors.Is(err, ErrEggConfigNotDelivered); got != tc.marked {
				t.Fatalf("marked = %v, want %v for a failure at %s", got, tc.marked, tc.step)
			}
			if tc.marked && strings.Contains(strings.Join(*steps, ","), "config") {
				t.Fatalf("a marked failure happened after the config write: %v", *steps)
			}
		})
	}
}

func TestSSHConnectorDeployMarksRejectedInputWithoutRemoteCalls(t *testing.T) {
	steps := fakeSSHDeploy(t, func(string) bool { return false })
	badKey := sshDeployTestPayload()
	badKey.MasterKey = "nope"
	if err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), nil, badKey); !errors.Is(err, ErrEggConfigNotDelivered) ||
		err.Error() != "deployment requires a 32-byte hexadecimal master key" {
		t.Fatalf("invalid master key: %v", err)
	}
	badID := sshDeployTestNest()
	badID.ID = "short"
	if err := (&SSHConnector{}).Deploy(context.Background(), badID, nil, sshDeployTestPayload()); !errors.Is(err, ErrEggConfigNotDelivered) {
		t.Fatalf("invalid nest ID: %v", err)
	}
	if len(*steps) != 0 {
		t.Fatalf("rejected input reached the nest: %v", *steps)
	}
}

// sshDeployCommands records every remote command of an SSH deploy.
func sshDeployCommands(t *testing.T, payload EggDeployPayload) []string {
	t.Helper()
	var cmds []string
	priorCommand, priorTransfer := sshRemoteCommand, sshTransferFile
	t.Cleanup(func() { sshRemoteCommand, sshTransferFile = priorCommand, priorTransfer })
	sshRemoteCommand = func(ctx context.Context, host string, port int, user string, secret []byte, cmd string, input ...io.Reader) (string, error) {
		cmds = append(cmds, cmd)
		return "ok\n", nil
	}
	sshTransferFile = func(ctx context.Context, host string, port int, user string, secret []byte, localPath, remotePath, direction string, allowedRoot ...string) error {
		return nil
	}
	if err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), []byte("secret"), payload); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	return cmds
}

func TestSSHConnectorDeployStopsTheRunningEggProcessBeforeTheNewOneStarts(t *testing.T) {
	cmds := sshDeployCommands(t, sshDeployTestPayload())
	start := cmds[len(cmds)-1]
	stop := strings.Index(start, "kill -TERM")
	launch := strings.Index(start, "nohup ./aurago")
	if stop < 0 || launch < 0 || stop > launch {
		t.Fatalf("start command = %q, want the running egg stopped (SIGTERM) before nohup starts the new one", start)
	}
	// Only processes of this nest's binary: exe is <base dir>/aurago, also
	// after the upload replaced the file ("(deleted)"). Never a pattern the
	// command line of the remote shell itself contains.
	for _, want := range []string{`dir=$HOME/'.aurago-egg-12345678'`, `pgrep -u "$(id -u)" -x aurago`, `"$dir/aurago"|"$dir/aurago (deleted)")`, "kill -KILL"} {
		if !strings.Contains(start, want) {
			t.Fatalf("start command lacks %q: %s", want, start)
		}
	}
	if strings.Contains(start, "pkill -f") || strings.Contains(start, "pgrep -f") {
		t.Fatalf("start command matches command lines, which includes its own shell: %s", start)
	}
	// Only nohup is backgrounded, detached from the SSH session's streams, so
	// the command returns instead of holding the session until the deploy
	// context expires.
	if !strings.HasSuffix(start, "{ nohup ./aurago > log/egg.log 2>&1 < /dev/null & echo $!; }") {
		t.Fatalf("start command = %q, want only nohup backgrounded with stdin from /dev/null", start)
	}
}

func TestSSHConnectorDeployRestartsAPermanentEggService(t *testing.T) {
	payload := sshDeployTestPayload()
	payload.Permanent = true
	cmds := sshDeployCommands(t, payload)
	start := cmds[len(cmds)-1]
	if !strings.Contains(start, "systemctl --user restart aurago-egg-12345678") || strings.Contains(start, "systemctl --user start ") {
		t.Fatalf("service start = %q, want systemctl --user restart so a running egg service picks up the new deploy", start)
	}
	for _, want := range []string{"systemctl --user daemon-reload", "systemctl --user enable aurago-egg-12345678"} {
		if !strings.Contains(start, want) {
			t.Fatalf("service start lacks %q: %s", want, start)
		}
	}
}

func TestSSHConnectorDeployUploadsToHomeRelativeSFTPPaths(t *testing.T) {
	var remotePaths []string
	priorCommand, priorTransfer := sshRemoteCommand, sshTransferFile
	t.Cleanup(func() { sshRemoteCommand, sshTransferFile = priorCommand, priorTransfer })
	sshRemoteCommand = func(ctx context.Context, host string, port int, user string, secret []byte, cmd string, input ...io.Reader) (string, error) {
		return "ok\n", nil
	}
	sshTransferFile = func(ctx context.Context, host string, port int, user string, secret []byte, localPath, remotePath, direction string, allowedRoot ...string) error {
		remotePaths = append(remotePaths, remotePath)
		return nil
	}
	if err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), []byte("secret"), sshDeployTestPayload()); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	// SFTP does not expand "~"; OpenSSH's sftp-server resolves relative paths
	// against the login's home directory, which the shell steps use as $HOME.
	want := []string{".aurago-egg-12345678/aurago", ".aurago-egg-12345678/resources.dat"}
	if strings.Join(remotePaths, ",") != strings.Join(want, ",") {
		t.Fatalf("SFTP remote paths = %v, want %v", remotePaths, want)
	}
}
