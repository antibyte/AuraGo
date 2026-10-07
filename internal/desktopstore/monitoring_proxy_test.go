package desktopstore

import (
	"reflect"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestCompanionSecretsAreScrubbedBeforeDockerCalls(t *testing.T) {
	const stored = "fixture-companion-vault-value-20261005"
	const inherited = "fixture-companion-inherited-value-20261005"
	s := &Service{cfg: Config{Secrets: &fakeSecretStore{data: map[string]string{"companion_token": stored}}}}
	values := s.companionTemplateSecrets([]string{"TOKEN=${SECRET:companion_token}"}, map[string]string{"other": inherited})
	if values["companion_token"] != stored || values["other"] != inherited {
		t.Fatal("companion credentials changed")
	}
	if scrubbed := security.Scrub("Docker failed: " + stored + " " + inherited); strings.Contains(scrubbed, stored) || strings.Contains(scrubbed, inherited) {
		t.Fatal("companion credentials were not registered for log scrubbing")
	}
}

func monitoringProxyEnv() []string {
	return []string{
		"AUTH=0",
		"CONTAINERS=1",
		"EVENTS=1",
		"INFO=1",
		"PING=1",
		"POST=0",
		"SECRETS=0",
		"VERSION=1",
	}
}

func TestMonitoringProxyCatalogUsesExplicitReadOnlyProfile(t *testing.T) {
	catalog := DefaultCatalog()
	for _, appID := range []string{"dozzle", "beszel"} {
		entry, ok := catalogEntryByID(catalog, appID)
		if !ok {
			t.Fatalf("%s missing from catalog", appID)
		}
		var proxy *CompanionTemplate
		for i := range entry.Companions {
			if entry.Companions[i].ID == "socket-proxy" {
				proxy = &entry.Companions[i]
				break
			}
		}
		if proxy == nil {
			t.Fatalf("%s has no socket proxy", appID)
		}
		if !reflect.DeepEqual(proxy.Env, monitoringProxyEnv()) {
			t.Fatalf("%s proxy env = %#v, want exact GET-only profile %#v", appID, proxy.Env, monitoringProxyEnv())
		}
		if len(proxy.HostBinds) != 1 || proxy.HostBinds[0].HostPath != "/var/run/docker.sock" || proxy.HostBinds[0].ContainerPath != "/var/run/docker.sock" || !proxy.HostBinds[0].ReadOnly {
			t.Fatalf("%s proxy socket bind = %#v", appID, proxy.HostBinds)
		}
	}

	dozzle, _ := catalogEntryByID(catalog, "dozzle")
	if len(dozzle.HostBinds) != 0 || !containsString(dozzle.Env, "DOZZLE_REMOTE_HOST=tcp://aurago-store-dozzle-socket-proxy:2375") {
		t.Fatalf("Dozzle must connect to its private proxy without mounting the socket: binds=%#v env=%#v", dozzle.HostBinds, dozzle.Env)
	}
	if dozzle.Companions[0].NetworkMode != "aurago-store-dozzle-net" || len(dozzle.Companions[0].Ports) != 0 {
		t.Fatalf("Dozzle proxy must remain private and unpublished: %#v", dozzle.Companions[0])
	}

	beszel, _ := catalogEntryByID(catalog, "beszel")
	proxy := beszel.Companions[0]
	if len(proxy.Ports) != 1 || proxy.Ports[0].HostIP != "127.0.0.1" || proxy.Ports[0].ContainerPort != 2375 {
		t.Fatalf("Beszel proxy must publish only its API port on loopback: %#v", proxy.Ports)
	}
	if beszel.Companions[1].NetworkMode != "host" || len(beszel.Companions[1].HostBinds) != 0 || !containsString(beszel.Companions[1].Env, "DOCKER_HOST=tcp://127.0.0.1:${COMPANION_PORT_SOCKET_PROXY_DOCKER_API}") {
		t.Fatalf("Beszel agent must preserve host metrics and use the loopback proxy: %#v", beszel.Companions[1])
	}
}

func catalogEntryByID(catalog []CatalogEntry, id string) (CatalogEntry, bool) {
	for _, entry := range catalog {
		if entry.ID == id {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}
