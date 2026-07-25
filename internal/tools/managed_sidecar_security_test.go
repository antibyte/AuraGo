package tools

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHardenManagedSidecarHostConfig(t *testing.T) {
	hostConfig := hardenManagedSidecarHostConfig(map[string]interface{}{"Memory": int64(1024)})

	if got := hostConfig["SecurityOpt"]; !reflect.DeepEqual(got, []string{"no-new-privileges:true"}) {
		t.Fatalf("SecurityOpt = %#v", got)
	}
	if got := hostConfig["CapDrop"]; !reflect.DeepEqual(got, []string{"ALL"}) {
		t.Fatalf("CapDrop = %#v", got)
	}
	if got := hostConfig["Memory"]; got != int64(1024) {
		t.Fatalf("Memory = %#v, want preserved value", got)
	}
}

func TestManagedContainerUserSpecForGOOS(t *testing.T) {
	if got := managedContainerUserSpecForGOOS("linux", 1234, 5678); got != "1234:5678" {
		t.Fatalf("linux user spec = %q", got)
	}
	if got := managedContainerUserSpecForGOOS("windows", 1234, 5678); got != "" {
		t.Fatalf("windows user spec = %q, want image default", got)
	}
	if got := managedContainerUserSpecForGOOS("linux", -1, 5678); got != "" {
		t.Fatalf("invalid user spec = %q", got)
	}
}

func TestBuildSupertonicCreatePayloadUsesNonRootCacheContract(t *testing.T) {
	payload := buildSupertonicCreatePayload("supertonic:test", "supertonic-3", "17788", "/srv/aurago/supertonic", "1234:5678")

	if got := payload["User"]; got != "1234:5678" {
		t.Fatalf("User = %#v", got)
	}
	if got := payload["Env"]; !reflect.DeepEqual(got, []string{"HOME=/home/supertonic"}) {
		t.Fatalf("Env = %#v", got)
	}
	hostConfig, ok := payload["HostConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("HostConfig = %T", payload["HostConfig"])
	}
	if got := hostConfig["SecurityOpt"]; !reflect.DeepEqual(got, []string{"no-new-privileges:true"}) {
		t.Fatalf("SecurityOpt = %#v", got)
	}
	if got := hostConfig["CapDrop"]; !reflect.DeepEqual(got, []string{"ALL"}) {
		t.Fatalf("CapDrop = %#v", got)
	}
	binds, ok := hostConfig["Binds"].([]string)
	if !ok || len(binds) != 1 || !strings.HasSuffix(binds[0], ":/home/supertonic/.cache") {
		t.Fatalf("Binds = %#v", hostConfig["Binds"])
	}
}

func TestManagedBrowserAndAnsibleHostConfigsAreHardened(t *testing.T) {
	browser := browserAutomationManagedHostConfig(BrowserAutomationSidecarConfig{
		WorkspaceDir: "/srv/aurago/workspace",
		DownloadDir:  "/srv/aurago/downloads",
	})
	ansible := ansibleManagedHostConfig([]string{"/srv/aurago/playbooks:/playbooks"})

	for name, hostConfig := range map[string]map[string]interface{}{
		"browser": browser,
		"ansible": ansible,
	} {
		if got := hostConfig["SecurityOpt"]; !reflect.DeepEqual(got, []string{"no-new-privileges:true"}) {
			t.Fatalf("%s SecurityOpt = %#v", name, got)
		}
		if got := hostConfig["CapDrop"]; !reflect.DeepEqual(got, []string{"ALL"}) {
			t.Fatalf("%s CapDrop = %#v", name, got)
		}
	}
}

func TestSupertonicContainerNeedsRecreate(t *testing.T) {
	newInspect := func() map[string]interface{} {
		return map[string]interface{}{
			"Config": map[string]interface{}{"User": "1234:5678"},
			"HostConfig": map[string]interface{}{
				"SecurityOpt": []interface{}{"no-new-privileges:true"},
				"CapDrop":     []interface{}{"ALL"},
				"Binds":       []interface{}{"/srv/aurago/supertonic:/home/supertonic/.cache"},
			},
		}
	}

	if supertonicContainerNeedsRecreate(newInspect()) {
		t.Fatal("hardened Supertonic container should be reusable")
	}

	for name, mutate := range map[string]func(map[string]interface{}){
		"root user": func(info map[string]interface{}) {
			info["Config"].(map[string]interface{})["User"] = ""
		},
		"old cache": func(info map[string]interface{}) {
			info["HostConfig"].(map[string]interface{})["Binds"] = []interface{}{"/srv/aurago/supertonic:/root/.cache"}
		},
		"missing security opt": func(info map[string]interface{}) {
			delete(info["HostConfig"].(map[string]interface{}), "SecurityOpt")
		},
		"missing cap drop": func(info map[string]interface{}) {
			delete(info["HostConfig"].(map[string]interface{}), "CapDrop")
		},
	} {
		t.Run(name, func(t *testing.T) {
			info := newInspect()
			mutate(info)
			if !supertonicContainerNeedsRecreate(info) {
				t.Fatal("legacy Supertonic container should be recreated")
			}
		})
	}
}

func TestSupertonicDockerfileRunsAsNonRootWithWritableHome(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "deploy", "docker", "Dockerfile.supertonic"))
	if err != nil {
		t.Fatalf("read Supertonic Dockerfile: %v", err)
	}
	dockerfile := string(content)
	for _, want := range []string{
		"useradd --uid 1001 --gid 1001",
		"ENV HOME=/home/supertonic",
		"mkdir -p /home/supertonic/.cache",
		"chown -R supertonic:supertonic /home/supertonic",
		"USER supertonic",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Supertonic Dockerfile missing %q", want)
		}
	}
}
