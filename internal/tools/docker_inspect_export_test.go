package tools

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The Docker Manager trims remote inspect documents with ProjectDockerInspect;
// it must give the admin inspect's answer byte for byte (minus "status").
func TestProjectDockerInspectMatchesTheAdminInspect(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(dockerInspectMountsFixture))
	})
	admin := DockerInspectContainer(DockerConfig{Host: host}, "db")

	projected, err := ProjectDockerInspect([]byte(dockerInspectMountsFixture))
	if err != nil {
		t.Fatalf("ProjectDockerInspect: %v", err)
	}
	if _, present := projected["status"]; present {
		t.Fatal("the projection must not carry a status key")
	}
	projected["status"] = "ok"
	got, _ := json.Marshal(projected)
	if string(got) != admin {
		t.Fatalf("projection differs from the admin inspect\n got: %s\nwant: %s", got, admin)
	}
	for _, leaked := range []string{"hunter2", "masterauth-secret", "/srv/secret-dir"} {
		if strings.Contains(string(got), leaked) {
			t.Fatalf("projection leaked %q: %s", leaked, got)
		}
	}
	if _, err := ProjectDockerInspect([]byte("not json")); err == nil {
		t.Fatal("ProjectDockerInspect accepted a document that is not JSON")
	}
}

func TestRedactDockerLabelsMasksCredentialLabels(t *testing.T) {
	in := map[string]string{
		"com.example.db_password":    "label-secret",
		"com.example.backup-url":     "s3://key:label-url-secret@bucket",
		"com.docker.compose.service": "cache",
		"aurago.managed":             "go2rtc",
	}
	out := RedactDockerLabels(in)
	for _, leaked := range []string{"label-secret", "label-url-secret"} {
		for key, value := range out {
			if strings.Contains(value, leaked) {
				t.Fatalf("label %s leaked %q: %q", key, leaked, value)
			}
		}
	}
	if out["com.docker.compose.service"] != "cache" || out["aurago.managed"] != "go2rtc" {
		t.Fatalf("ownership labels must stay visible: %v", out)
	}
	if in["com.example.db_password"] != "label-secret" {
		t.Fatal("RedactDockerLabels must not modify its argument")
	}
	if got := RedactDockerLabels(nil); len(got) != 0 {
		t.Fatalf("RedactDockerLabels(nil) = %v, want empty", got)
	}
}

// RedactDockerMountSource must stay in step with the inspect's mount projection.
func TestRedactDockerMountSourceMatchesTheInspectProjection(t *testing.T) {
	for _, mount := range []map[string]interface{}{
		{"Type": "bind", "Source": "/srv/secret-dir"},
		{"Type": "bind", "Source": `C:\data\secret-dir`},
		{"Type": "volume", "Name": "pgdata", "Source": "/var/lib/docker/volumes/pgdata/_data"},
	} {
		projected, _ := projectDockerInspectMounts([]interface{}{mount}, false).([]interface{})
		want, _ := projected[0].(map[string]interface{})["source"].(string)
		if got := RedactDockerMountSource(mount["Type"].(string), mount["Source"].(string)); got != want {
			t.Fatalf("RedactDockerMountSource(%v) = %q, inspect projection gives %q", mount, got, want)
		}
	}
	if got := RedactDockerMountSource("bind", ""); got != "" {
		t.Fatalf("empty source = %q, want empty", got)
	}
}
