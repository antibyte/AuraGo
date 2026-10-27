package server

import (
	"net/http"
	"strings"
	"testing"

	"aurago/internal/dockerstacks"
)

// setContainers replaces the container list the fake engine answers with.
func (e *dmFakeEngine) setContainers(raw string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.containers = raw
}

// dmHelperContainers is a list with the three helper roles next to an ordinary container.
func dmHelperContainers(t *testing.T) string {
	t.Helper()
	return dmContainersJSON(t,
		dmContainerJSON('1', "aurago-dm-op-op_0123456789abcdef", "sha256:img1", "exited", "none",
			map[string]string{dockerstacks.LabelRole: dockerstacks.RoleRunner, dockerstacks.LabelOperation: "op_0123456789abcdef"}, nil),
		dmContainerJSON('2', dockerstacks.IOContainerName, "sha256:img1", "created", "none",
			map[string]string{dockerstacks.LabelRole: dockerstacks.RoleIO, dockerstacks.LabelStacksDir: "/opt/stacks"}, nil),
		dmContainerJSON('3', "aurago-dm-ls-0a1b2c3d", "sha256:img1", "exited", "none",
			map[string]string{dockerstacks.LabelRole: dockerstacks.RoleLister}, nil),
		dmContainerJSON('4', "blog", "sha256:img2", "running", "bridge", nil, nil),
	)
}

func TestDockerManagerContainerListFlagsHelpers(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.remote.setContainers(dmHelperContainers(t))
	f.local.setContainers(dmHelperContainers(t))

	for _, host := range []string{id, "local"} {
		code, body := f.do(http.MethodGet, "/hosts/"+host+"/containers", nil)
		if code != http.StatusOK {
			t.Fatalf("%s list = %d %s", host, code, f.lastRaw)
		}
		list := dmArray(t, body, "containers")
		for name, role := range map[string]string{
			"aurago-dm-op-op_0123456789abcdef": dockerstacks.RoleRunner,
			dockerstacks.IOContainerName:       dockerstacks.RoleIO,
			"aurago-dm-ls-0a1b2c3d":            dockerstacks.RoleLister,
		} {
			helper := dmContainerNamed(t, list, name)
			if helper["aurago_helper"] != role || helper["protected_owner"] != "docker_manager" {
				t.Errorf("%s: %s = %v, want aurago_helper %q and protected_owner docker_manager", host, name, helper, role)
			}
		}
		if blog := dmContainerNamed(t, list, "blog"); blog["aurago_helper"] != nil || blog["protected_owner"] != nil {
			t.Errorf("%s: blog = %v, want no helper flag and no owner", host, blog)
		}
	}
}

func TestDockerManagerRemovingAHelperAsksForConfirmation(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.remote.setContainers(dmHelperContainers(t))
	f.local.setContainers(dmHelperContainers(t))
	defer replaceContainerProtection(containerProtection{})() // the local classification itself is not under test here
	base := "/hosts/" + id + "/containers/"

	// By name and by ID prefix, the helper is protected; nothing reaches the engine unconfirmed.
	for _, ref := range []string{dockerstacks.IOContainerName, strings.Repeat("1", 12)} {
		code, body := f.do(http.MethodDelete, base+ref, nil)
		dmExpectError(t, f, code, body, http.StatusConflict, containerCodeConfirmationRequired)
		if body["owner"] != "docker_manager" || !strings.Contains(body["message"].(string), "confirm=protected") {
			t.Fatalf("unconfirmed remove of %s = %s, want owner docker_manager and the repeat hint", ref, f.lastRaw)
		}
	}
	if f.remote.called("DELETE /containers/"+dockerstacks.IOContainerName) || f.remote.called("DELETE /containers/"+strings.Repeat("1", 12)) {
		t.Fatalf("an unconfirmed remove reached the engine: %v", f.remote.calls)
	}
	code, _ := f.do(http.MethodDelete, base+dockerstacks.IOContainerName+"?confirm=protected", nil)
	if code != http.StatusOK || !f.remote.called("DELETE /containers/"+dockerstacks.IOContainerName) {
		t.Fatalf("confirmed remove = %d %s, calls %v", code, f.lastRaw, f.remote.calls)
	}
	if code, _ = f.do(http.MethodDelete, base+"blog", nil); code != http.StatusOK || !f.remote.called("DELETE /containers/blog") {
		t.Fatalf("remove of an ordinary container = %d %s, want it unconfirmed", code, f.lastRaw)
	}

	// The local host asks too, before its own classification runs.
	code, body := f.do(http.MethodDelete, "/hosts/local/containers/aurago-dm-ls-0a1b2c3d", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, containerCodeConfirmationRequired)
	if body["owner"] != "docker_manager" || f.local.called("DELETE /containers/aurago-dm-ls-0a1b2c3d") {
		t.Fatalf("local unconfirmed remove = %s, calls %v", f.lastRaw, f.local.calls)
	}

	// Read-only mode still refuses first.
	f.setDocker(true, true)
	code, body = f.do(http.MethodDelete, base+dockerstacks.IOContainerName+"?confirm=protected", nil)
	dmExpectError(t, f, code, body, http.StatusForbidden, "read_only")
}

func TestDockerManagerHostTestCarriesTheHelperStatus(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")

	// The fake engine has no runner image and an IO container without the Docker Manager's labels.
	code, body := f.do(http.MethodPost, "/hosts/"+id+"/test", nil)
	helpers := dmObject(t, body, "helpers")
	if code != http.StatusOK || helpers["api_version"] != "1.45" || helpers["api_supported"] != true || helpers["runner_image"] != dockerstacks.RunnerImageTag ||
		helpers["runner_image_present"] != false || helpers["io_container"] != "mismatch" || helpers["prepared"] != false {
		t.Fatalf("test = %d %s, want the helper status of an unprepared host", code, f.lastRaw)
	}

	// An unreachable host answers without helpers, and a server without the stacks service never adds them.
	f.remote.srv.Close()
	code, body = f.do(http.MethodPost, "/hosts/"+id+"/test", nil)
	if code != http.StatusOK || body["helpers"] != nil {
		t.Fatalf("unreachable test = %d %s, want no helpers member", code, f.lastRaw)
	}
	stacks := f.s.DockerStacks
	f.s.DockerStacks = nil
	t.Cleanup(stacks.Close)
	code, body = f.do(http.MethodPost, "/hosts/local/test", nil)
	if code != http.StatusOK || body["helpers"] != nil {
		t.Fatalf("test without the stacks service = %d %s, want no helpers member", code, f.lastRaw)
	}
}
