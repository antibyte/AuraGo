package rustfs

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"aurago/internal/config"
)

func installLiveRustFSAliasCollision(t *testing.T, m *Manager, cfg *config.Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m.mu.Lock()
	s := m.state
	m.mu.Unlock()
	p, err := m.provePlacement(ctx, cfg, s)
	if err != nil {
		t.Fatal("alias fixture placement failed")
	}
	name := m.names.container + "-dns-collision"
	spec := map[string]any{
		"Image": Image, "Entrypoint": []string{"/bin/sleep"}, "Cmd": []string{"600"},
		"Labels":     m.labels("dns-collision", s),
		"HostConfig": map[string]any{"NetworkMode": p.network},
		"NetworkingConfig": map[string]any{"EndpointsConfig": map[string]any{
			p.network: map[string]any{"Aliases": []string{m.names.container}},
		}},
	}
	var created struct {
		ID string `json:"Id"`
	}
	if _, err = m.docker.DoJSON(ctx, http.MethodPost, "containers/create?name="+url.QueryEscape(name), spec, &created); err != nil || created.ID == "" {
		t.Fatal("alias fixture create failed")
	}
	t.Cleanup(func() {
		cleanup, finish := context.WithTimeout(context.Background(), 30*time.Second)
		defer finish()
		var info containerInfo
		if _, err := m.docker.DoJSON(cleanup, http.MethodGet, "containers/"+created.ID+"/json", nil, &info); err != nil || info.ID != created.ID || !owns(info.Config.Labels, s, "dns-collision") {
			t.Error("alias fixture cleanup ownership failed")
			return
		}
		if _, err := m.docker.DoJSON(cleanup, http.MethodDelete, "containers/"+created.ID+"?force=true&v=true", nil, nil); err != nil {
			t.Error("alias fixture cleanup failed")
		}
	})
	if _, err = m.docker.DoJSON(ctx, http.MethodPost, "containers/"+created.ID+"/start", nil, nil); err != nil {
		t.Fatal("alias fixture start failed")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, m.names.container)
	if err != nil {
		t.Fatal("alias fixture DNS lookup failed")
	}
	unique := map[string]bool{}
	for _, address := range addresses {
		unique[address.IP.String()] = true
	}
	if len(unique) < 2 {
		t.Fatal("alias fixture did not create ambiguous DNS")
	}
	endpoint, err := url.Parse(s.Endpoint)
	if err != nil || net.ParseIP(endpoint.Hostname()) == nil || !unique[endpoint.Hostname()] {
		t.Fatal("managed endpoint is not a pinned container address")
	}
	owned, found, err := m.inspectContainer(ctx, s)
	if err != nil || !found {
		t.Fatal("owned runtime inspection failed")
	}
	actual, err := m.observedEndpoint(cfg, s, owned, p)
	if err != nil || actual != s.Endpoint {
		t.Fatal("managed endpoint does not match the owned runtime")
	}
	t.Log("Duplicate Docker DNS alias present; S3 and IAM use the inspected runtime address")
}
