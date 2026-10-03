package tools

import (
	"net/http"
	"strings"
	"testing"
)

func TestDockerInspectRedactsCmdLabelsAndEnvURLCredentials(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/containers/app/json") {
			t.Errorf("unexpected Docker request %s", r.URL.Path)
			return
		}
		_, _ = w.Write([]byte(`{"Id":"app","Name":"/app","Config":{` +
			`"Image":"redis:7",` +
			`"Env":["DATABASE_URL=postgres://svc:env-url-secret@db:5432/app","PATH=/usr/bin"],` +
			`"Cmd":["redis-server","--requirepass","cmd-space-secret","--masterauth=cmd-eq-secret","--port","6379"],` +
			`"Labels":{"com.example.db_password":"label-secret","traefik.http.middlewares.auth.basicauth.users":"admin:$apr1$hash",` +
			`"com.docker.compose.service":"cache","aurago.managed":"go2rtc","com.example.backup-url":"s3://key:label-url-secret@bucket"}}}`))
	})
	out := DockerInspectContainer(DockerConfig{Host: host}, "app")
	for _, leaked := range []string{"env-url-secret", "cmd-space-secret", "cmd-eq-secret", "label-secret", "$apr1$hash", "label-url-secret"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("docker inspect leaked %q: %s", leaked, out)
		}
	}
	for _, kept := range []string{"redis-server", "--requirepass", "--port", "6379", "PATH=/usr/bin", `"com.docker.compose.service":"cache"`, `"aurago.managed":"go2rtc"`} {
		if !strings.Contains(out, kept) {
			t.Fatalf("docker inspect dropped non-secret %q: %s", kept, out)
		}
	}
}

func TestDockerInspectRequiresDockerPermission(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	out := DockerInspectContainer(DockerConfig{Host: "tcp://127.0.0.1:1"}, "app")
	if !strings.Contains(out, "docker is disabled by runtime permissions") {
		t.Fatalf("inspect without docker permission = %s", out)
	}
}
