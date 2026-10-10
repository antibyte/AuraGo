package tools

import (
	"encoding/json"
	"net/http"
	"reflect"
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

const dockerInspectMountsFixture = `{"Id":"db","Name":"/db","Mounts":[` +
	`{"Type":"bind","Source":"/srv/secret-dir","Destination":"/data","Mode":"ro","RW":false,"Propagation":"rprivate"},` +
	`{"Type":"volume","Name":"pgdata","Source":"/var/lib/docker/volumes/pgdata/_data","Destination":"/var/lib/postgresql/data","Driver":"local","Mode":"z","RW":true},` +
	`"not-a-map"],"Config":{"Image":"postgres:16","Env":[` +
	`"DB_PASS=hunter2","REDIS_REQUIREPASS=x","MYSQL_ROOT_PASSWD=passwd-secret","ADMIN_PWD=pwd-secret",` +
	`"GPG_PASSPHRASE=phrase-secret","AWS_CREDENTIALS=cred-secret","REDIS_MASTERAUTH=masterauth-secret",` +
	`"PASS=bare-secret","PASSPORT_OFFICE=kept-value","PATH=/usr/bin",` +
	`"PAPERLESS_SECRET_KEY=paperless-value","AUTHENTIK_SECRET_KEY=authentik-value","SECRET_KEY=django-value",` +
	`"ENCRYPTION_KEY=encryption-value","N8N_ENCRYPTION_KEY=n8n-value","APP_KEY=laravel-value",` +
	`"SECRET_KEY_BASE=rails-value","PWD=/app","DB_PWD=x"]}}`

func TestDockerInspectRedactsShortPasswordNamesAndBindSources(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(dockerInspectMountsFixture))
	})
	out := DockerInspectContainer(DockerConfig{Host: host}, "db")

	for _, leaked := range []string{"hunter2", "REDIS_REQUIREPASS=x", "passwd-secret", "pwd-secret", "phrase-secret", "cred-secret", "masterauth-secret", "bare-secret", "/srv/secret-dir", "rprivate",
		"paperless-value", "authentik-value", "django-value", "encryption-value", "n8n-value", "laravel-value", "rails-value", "DB_PWD=x", "not-a-map"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("docker inspect leaked %q: %s", leaked, out)
		}
	}
	for _, masked := range []string{"DB_PASS=", "REDIS_REQUIREPASS=", "REDIS_MASTERAUTH=", "PAPERLESS_SECRET_KEY=", "AUTHENTIK_SECRET_KEY=", "SECRET_KEY=",
		"ENCRYPTION_KEY=", "N8N_ENCRYPTION_KEY=", "APP_KEY=", "SECRET_KEY_BASE=", "DB_PWD="} {
		if !strings.Contains(out, `"`+masked+dockerInspectRedacted+`"`) {
			t.Fatalf("docker inspect did not mask %q: %s", masked, out)
		}
	}
	for _, kept := range []string{"PASSPORT_OFFICE=kept-value", "PATH=/usr/bin", `"PWD=/app"`} {
		if !strings.Contains(out, kept) {
			t.Fatalf("docker inspect dropped non-secret %q: %s", kept, out)
		}
	}

	var resp struct {
		Mounts []any `json:"mounts"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse inspect output: %v", err)
	}
	if len(resp.Mounts) != 2 {
		t.Fatalf("mounts = %v, want the bind and the volume (non-object entries dropped)", resp.Mounts)
	}
	bind, _ := resp.Mounts[0].(map[string]any)
	wantBind := map[string]any{"type": "bind", "source": "secret-dir", "destination": "/data", "mode": "ro", "rw": false}
	if !reflect.DeepEqual(bind, wantBind) {
		t.Fatalf("bind mount = %v, want %v", bind, wantBind)
	}
	volume, _ := resp.Mounts[1].(map[string]any)
	wantVolume := map[string]any{"type": "volume", "name": "pgdata", "source": "/var/lib/docker/volumes/pgdata/_data", "destination": "/var/lib/postgresql/data", "mode": "z", "rw": true}
	if !reflect.DeepEqual(volume, wantVolume) {
		t.Fatalf("volume mount = %v, want %v", volume, wantVolume)
	}
}

// Docker always sends Mounts as an array of objects; anything else is dropped
// rather than passed through unredacted.
func TestDockerInspectMountProjectionFailsClosedOnUnexpectedShapes(t *testing.T) {
	for _, value := range []any{"/srv/secret-dir", map[string]any{"Source": "/srv/secret-dir"}, float64(1), nil} {
		if got := projectDockerInspectMounts(value, false); got != nil {
			t.Errorf("projectDockerInspectMounts(%#v) = %#v, want nil", value, got)
		}
	}
	got := projectDockerInspectMounts([]any{"/srv/secret-dir", map[string]any{"Type": "bind", "Source": "/srv/secret-dir"}}, false)
	want := []any{map[string]any{"type": "bind", "source": "secret-dir"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected mounts = %#v, want %#v", got, want)
	}
}

// Code Studio compares the workspace bind source with its host path; that
// trusted in-process caller gets the full source, nobody else does.
func TestDockerInspectWithMountSourcesKeepsBindPathsForHostChecks(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(dockerInspectMountsFixture))
	})
	out := DockerInspectContainerWithMountSources(DockerConfig{Host: host}, "db")
	if !strings.Contains(out, `"source":"/srv/secret-dir"`) {
		t.Fatalf("host-check inspect lost the bind source: %s", out)
	}
	if strings.Contains(out, "hunter2") || strings.Contains(out, "masterauth-secret") {
		t.Fatalf("host-check inspect leaked environment secrets: %s", out)
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

// Credential words glued to a prefix (WEBPASSWORD, PGPASSWORD, TS_AUTHKEY) are
// masked, and a credential flag inside an ordinary env value
// (REDIS_ARGS=--requirepass x) loses its value; harmless values stay intact.
func TestDockerInspectRedactsGluedCredentialWordsAndEnvFlagValues(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Id":"app","Name":"/app","Config":{"Image":"pihole","Env":[` +
			`"WEBPASSWORD=web-secret","PGPASSWORD=pg-secret","TS_AUTHKEY=tskey-auth-secret",` +
			`"WG_PASSWORD_HASH=hash-secret","APP_APIKEY=apikey-secret","X_API_KEY_FILE=keyfile-secret",` +
			`"DB_PASSWD_FILE=passwd-secret","MY_SECRETS=secrets-secret","GITHUB_TOKENS=tokens-secret",` +
			`"REDIS_ARGS=--requirepass redis-secret --port 6379","VALKEY_EXTRA_FLAGS=--masterauth=auth-secret",` +
			`"EXTRA_FLAGS=--verbose --log-level info","PASSPORT_OFFICE=kept-value","PATH=/usr/bin","PWD=/app"]}}`))
	})
	out := DockerInspectContainer(DockerConfig{Host: host}, "app")
	for _, leaked := range []string{"web-secret", "pg-secret", "tskey-auth-secret", "hash-secret", "apikey-secret", "keyfile-secret",
		"passwd-secret", "secrets-secret", "tokens-secret", "redis-secret", "auth-secret"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("docker inspect leaked %q: %s", leaked, out)
		}
	}
	for _, masked := range []string{"WEBPASSWORD=", "PGPASSWORD=", "TS_AUTHKEY=", "WG_PASSWORD_HASH=", "APP_APIKEY=", "X_API_KEY_FILE=",
		"DB_PASSWD_FILE=", "MY_SECRETS=", "GITHUB_TOKENS="} {
		if !strings.Contains(out, `"`+masked+dockerInspectRedacted+`"`) {
			t.Fatalf("docker inspect did not mask %q: %s", masked, out)
		}
	}
	for _, kept := range []string{`"REDIS_ARGS=--requirepass ` + dockerInspectRedacted + ` --port 6379"`,
		`"VALKEY_EXTRA_FLAGS=--masterauth=` + dockerInspectRedacted + `"`,
		`"EXTRA_FLAGS=--verbose --log-level info"`, `"PASSPORT_OFFICE=kept-value"`, `"PATH=/usr/bin"`, `"PWD=/app"`} {
		if !strings.Contains(out, kept) {
			t.Fatalf("docker inspect output lacks %s: %s", kept, out)
		}
	}
}

// The whole value of an ordinary env key still goes through the scrubber, so
// its patterns that span whitespace (a bearer header, key = value,
// key: value) keep masking; the command-line pass then also masks the
// argument of a credential flag.
func TestDockerInspectEnvValuesKeepWhitespaceSpanningScrubbing(t *testing.T) {
	encoded, _ := json.Marshal(redactDockerInspectEnv([]interface{}{
		"CURL_ARGS=-H Authorization: Bearer abcdefghijklmnopqrstuvwxyz012345",
		"APP_SETTINGS=password = hunter2hunter2",
		"EXTRA_SETTINGS=token: abcdefghijklmnop",
		"VALKEY_ARGS=--requirepass x",
		"JAVA_OPTS=-Xmx1g  -Dlog.level=info",
	}))
	out := string(encoded)
	for _, leaked := range []string{"abcdefghijklmnopqrstuvwxyz012345", "hunter2hunter2", "abcdefghijklmnop"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("env value scrubbing leaked %q: %s", leaked, out)
		}
	}
	for _, key := range []string{`"CURL_ARGS=`, `"APP_SETTINGS=`, `"EXTRA_SETTINGS=`} {
		if !strings.Contains(out, key) {
			t.Fatalf("env output lost the key %s: %s", key, out)
		}
	}
	if !strings.Contains(out, `"VALKEY_ARGS=--requirepass `+dockerInspectRedacted+`"`) {
		t.Fatalf("credential flag argument not masked: %s", out)
	}
	// A value with nothing to mask keeps its original spacing.
	if !strings.Contains(out, `"JAVA_OPTS=-Xmx1g  -Dlog.level=info"`) {
		t.Fatalf("harmless env value was rewritten: %s", out)
	}
}

func TestOpenSCADProbeInspectExposesHardening(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/containers/aurago-openscad/json") {
			t.Errorf("unexpected Docker request %s", r.URL.Path)
			return
		}
		_, _ = w.Write([]byte(`{"Id":"probe","Name":"/aurago-openscad","State":{"Running":true},` +
			`"HostConfig":{"ReadonlyRootfs":true,"Tmpfs":{"/tmp":"rw,nosuid,size=256m"}},` +
			`"Config":{"Env":["HOME=/tmp","PATH=/usr/bin"]}}`))
	})
	out := DockerInspectContainerWithMountSources(DockerConfig{Host: host}, "aurago-openscad")
	for _, want := range []string{`"readonly_rootfs":true`, `"/tmp":"rw,nosuid,size=256m"`, `"HOME=/tmp"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("inspect output missing %s: %s", want, out)
		}
	}
}
