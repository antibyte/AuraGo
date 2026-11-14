package invasion

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDockerEggRecoveryOnlyBeforePossibleTransfer(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		before, secure, renamed bool
		restored                bool
	}{
		{"safe-predecessor", true, true, true, true}, {"legacy-predecessor", true, false, true, false},
		{"no-acknowledged-predecessor", true, true, false, false}, {"possible-transfer", false, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recoveryReads, restores, uploads := 0, 0, 0
			server := mockDockerAPI(t, map[string]http.HandlerFunc{
				"/images/create": respond(http.StatusOK, "{\"status\":\"ready\"}\n"),
				"/containers/create": func(w http.ResponseWriter, r *http.Request) {
					if tc.before {
						w.WriteHeader(http.StatusConflict)
					} else {
						w.WriteHeader(http.StatusCreated)
					}
				},
				"/containers/": func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/json") {
						recoveryReads++
						if tc.secure {
							_, _ = io.WriteString(w, `{"Config":{"Labels":{"aurago.egg-security":"1"}}}`)
						} else {
							_, _ = io.WriteString(w, `{"Config":{"Labels":{}}}`)
						}
						return
					}
					if strings.Contains(r.URL.Path, "/archive") {
						uploads++
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if strings.HasSuffix(r.URL.Path, "-prev/rename") {
						restores++
						w.WriteHeader(http.StatusNoContent)
						return
					}
					if strings.HasSuffix(r.URL.Path, "/rename") && !tc.renamed {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.WriteHeader(http.StatusNoContent)
				},
			})
			defer server.Close()
			err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(server), nil, EggDeployPayload{ConfigYAML: []byte("directories:\n  data_dir: ./data\n")})
			if err == nil || errors.Is(err, ErrEggConfigNotDelivered) != tc.before || errors.Is(err, ErrEggPredecessorRestored) != tc.restored {
				t.Fatalf("recovery phase = %v", err)
			}
			if tc.restored && restores != 1 {
				t.Fatal("secure predecessor was not restored")
			}
			if !tc.secure && restores != 0 {
				t.Fatal("legacy predecessor started")
			}
			if (!tc.before || !tc.renamed) && recoveryReads != 0 {
				t.Fatal("unproven recovery attempted")
			}
			if tc.before && uploads != 0 || !tc.before && uploads != 1 {
				t.Fatalf("transfer phase counts = %d", uploads)
			}
		})
	}
}

func TestSSHEggRecoveryOnlyBeforePossibleTransfer(t *testing.T) {
	for _, tc := range []struct {
		name             string
		before, backedUp bool
	}{
		{"before-transfer", true, true},
		{"possible-transfer", false, true},
		{"no-acknowledged-predecessor", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.before
			previousCommand, previousTransfer := sshRemoteCommand, sshTransferFile
			t.Cleanup(func() { sshRemoteCommand, sshTransferFile = previousCommand, previousTransfer })
			restored, checked := false, false
			sshRemoteCommand = func(ctx context.Context, host string, port int, user string, secret []byte, cmd string, input ...io.Reader) (string, error) {
				var body string
				if len(input) > 0 {
					data, _ := io.ReadAll(input[0])
					body = string(data)
				}
				if strings.Contains(body, "Existing Egg .env") && !strings.Contains(body, "AURAGO_DEPLOY_DATA") {
					return "recoverable\n", nil
				}
				if strings.Contains(cmd, "cp -a ") {
					if tc.backedUp {
						return "backed_up\n", nil
					}
					return "", nil
				}
				if strings.HasPrefix(cmd, "test -d ") {
					checked = true
					return "ok", nil
				}
				if strings.HasPrefix(cmd, "cat ") {
					return "1\n", nil
				}
				if strings.Contains(cmd, " && mv ") {
					restored = true
				}
				if !before && strings.Contains(body, "/config.yaml'") {
					return "", errors.New("lost write acknowledgement")
				}
				return "ok", nil
			}
			sshTransferFile = func(ctx context.Context, host string, port int, user string, secret []byte, local, remote, direction string, root ...string) error {
				if before && strings.HasSuffix(remote, "/aurago") {
					return errors.New("binary transfer failed")
				}
				return nil
			}
			err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), nil, sshDeployTestPayload())
			wantRestore := before && tc.backedUp
			if errors.Is(err, ErrEggConfigNotDelivered) != before || errors.Is(err, ErrEggPredecessorRestored) != wantRestore || restored != wantRestore || checked != wantRestore {
				t.Fatalf("unsafe SSH recovery phase: restored=%v checked=%v error=%v", restored, checked, err)
			}
		})
	}
}
