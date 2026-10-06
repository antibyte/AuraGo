package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/tools"
)

// replaceContainerConnIP makes every traced Docker request report ip as the
// remote address of its connection; "" means none was observed.
func replaceContainerConnIP(ip string) func() {
	old := containerTraceConnIP
	containerTraceConnIP = func(ctx context.Context) (context.Context, func() string) {
		return ctx, func() string { return ip }
	}
	return func() { containerTraceConnIP = old }
}

func TestDockerEndpointFromConnection(t *testing.T) {
	for ip, want := range map[string]struct {
		addrs []string
		ok    bool
	}{
		"172.18.0.5": {[]string{"172.18.0.5"}, true},
		"fd00::5":    {[]string{"fd00::5"}, true},
		"127.0.0.1":  {nil, true},
		"::1":        {nil, true},
		"0.0.0.0":    {nil, true},
		"":           {nil, false},
		"not-an-ip":  {nil, false},
	} {
		addrs, ok := dockerEndpointFromConnection(ip)
		if ok != want.ok || !slices.Equal(addrs, want.addrs) {
			t.Fatalf("dockerEndpointFromConnection(%q) = %v, %v; want %v, %v", ip, addrs, ok, want.addrs, want.ok)
		}
	}
}

func TestTraceDockerConnRemoteIPRecordsTheEngineAddress(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("OK")) })
	ctx, connIP := traceDockerConnRemoteIP(context.Background())
	if _, code, err := tools.DockerRequestContext(ctx, tools.DockerConfig{Host: host}, http.MethodGet, "/_ping", ""); err != nil || code != http.StatusOK {
		t.Fatalf("ping = %d, %v", code, err)
	}
	if ip := net.ParseIP(connIP()); ip == nil || !ip.IsLoopback() {
		t.Fatalf("observed connection IP = %q, want the loopback address of the test engine", connIP())
	}
}

// TestContainerEndpointLookupFailureUsesTheConnectionAddress: when docker.host
// does not resolve but Docker answers, the container at the address of
// AuraGo's own Docker connection is the endpoint, and every other container is
// classified normally instead of "unverified" (M7).
func TestContainerEndpointLookupFailureUsesTheConnectionAddress(t *testing.T) {
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/web","Config":{"Labels":{}},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/proxy/json"):
			_, _ = w.Write([]byte(`{"Id":"bbbbbbbbbbbb2222","Name":"/aurago_docker_proxy","Config":{"Labels":{}},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.5"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			_, _ = w.Write([]byte("OK"))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[
				{"Id":"cccccccccccc3333","Names":["/web"],"Image":"nginx","State":"running","Status":"Up","Labels":{},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2"}}}},
				{"Id":"bbbbbbbbbbbb2222","Names":["/aurago_docker_proxy"],"Image":"tecnativa/docker-socket-proxy","State":"running","Status":"Up","Labels":{},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.5"}}}}
			]`))
		default:
			http.NotFound(w, r)
		}
	})
	t.Cleanup(replaceContainerSelfHostname("aurago-host"))
	t.Cleanup(replaceContainerSelfProcFiles(nil))
	t.Cleanup(replaceContainerEndpointFailure())
	t.Cleanup(replaceContainerConnIP("172.18.0.5"))
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()

	if got, want := classifyContainerForAction(ctx, s, cfg, "web"), (containerProtection{}); got != want {
		t.Fatalf("web with failed lookup = %+v, want %+v (no blanket confirmation)", got, want)
	}
	if got, want := classifyContainerForAction(ctx, s, cfg, "proxy"), (containerProtection{DockerEndpoint: true}); got != want {
		t.Fatalf("proxy with failed lookup = %+v, want %+v", got, want)
	}
	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/proxy/update", nil))
	if body := decodeContainerResponse(t, rec); rec.Code != http.StatusConflict || body["code"] != containerCodeSelfUpdateUnsupported {
		t.Fatalf("proxy update = %d %v, want 409 %s", rec.Code, body, containerCodeSelfUpdateUnsupported)
	}

	rec = httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	var list struct {
		Containers []map[string]interface{} `json:"containers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Containers) != 2 {
		t.Fatalf("list = %d %s (%v)", rec.Code, rec.Body.String(), err)
	}
	for _, c := range list.Containers {
		if want := c["id"] == "bbbbbbbbbbbb"; (c["docker_endpoint"] == true) != want {
			t.Fatalf("list entry %v: docker_endpoint must mark only the connected proxy", c)
		}
	}
}

// TestContainerEndpointFallbackNeverLoosensAnUnprovenTarget: the connection
// address replaces the failed lookup only when a listed container has it. A
// loopback or host address names no container (a proxy behind a published
// port stays unknown), another replica of the connected container's compose
// service may serve the name too, and a failed list proves nothing: those
// targets stay unverified, exactly as without the fallback. Other flags stay.
func TestContainerEndpointFallbackNeverLoosensAnUnprovenTarget(t *testing.T) {
	var listFails atomic.Bool
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/web/json"):
			_, _ = w.Write([]byte(`{"Id":"cccccccccccc3333","Name":"/web","Config":{"Labels":{}},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/cams/json"):
			_, _ = w.Write([]byte(`{"Id":"aaaaaaaaaaaa1111","Name":"/cams","Config":{"Labels":{"aurago.managed":"go2rtc"}},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.3"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/containers/replica/json"):
			_, _ = w.Write([]byte(`{"Id":"dddddddddddd4444","Name":"/aurago-docker-proxy-2","Config":{"Labels":{"com.docker.compose.project":"aurago","com.docker.compose.service":"docker-proxy"}},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.6"}}}}`))
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			_, _ = w.Write([]byte("OK"))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			if listFails.Load() {
				// 403, not 5xx: tools retries a failed GET on 5xx with sleeps.
				http.Error(w, `{"message":"list refused"}`, http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`[
				{"Id":"cccccccccccc3333","Names":["/web"],"Image":"nginx","State":"running","Status":"Up","Labels":{},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2"}}}},
				{"Id":"aaaaaaaaaaaa1111","Names":["/cams"],"Image":"go2rtc","State":"running","Status":"Up","Labels":{"aurago.managed":"go2rtc"},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.3"}}}},
				{"Id":"bbbbbbbbbbbb2222","Names":["/aurago-docker-proxy-1"],"Image":"tecnativa/docker-socket-proxy","State":"running","Status":"Up","Labels":{"com.docker.compose.project":"aurago","com.docker.compose.service":"docker-proxy"},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.5"}}}},
				{"Id":"dddddddddddd4444","Names":["/aurago-docker-proxy-2"],"Image":"tecnativa/docker-socket-proxy","State":"running","Status":"Up","Labels":{"com.docker.compose.project":"aurago","com.docker.compose.service":"docker-proxy"},"NetworkSettings":{"Networks":{"aurago_docker-control":{"IPAddress":"172.18.0.6"}}}}
			]`))
		default:
			http.NotFound(w, r)
		}
	})
	t.Cleanup(replaceContainerSelfHostname("aurago-host"))
	t.Cleanup(replaceContainerSelfProcFiles(nil))
	t.Cleanup(replaceContainerEndpointFailure())
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()
	classify := func(ip, target string) containerProtection {
		t.Helper()
		restore := replaceContainerConnIP(ip)
		defer restore()
		return classifyContainerForAction(ctx, s, cfg, target)
	}
	unverified := containerProtection{Unverified: true}

	for _, ip := range []string{"", "127.0.0.1", "::1", "0.0.0.0", "192.168.1.10"} {
		if got := classify(ip, "web"); got != unverified {
			t.Fatalf("connection %q: web = %+v, want %+v (the address names no container)", ip, got, unverified)
		}
		if got, want := classify(ip, "cams"), (containerProtection{Owner: "go2rtc", Unverified: true}); got != want {
			t.Fatalf("connection %q: cams = %+v, want %+v", ip, got, want)
		}
	}
	// Connected to replica 1: replica 2 of the same compose service may serve
	// the name too; other containers are classified normally.
	if got := classify("172.18.0.5", "replica"); got != unverified {
		t.Fatalf("other replica = %+v, want %+v", got, unverified)
	}
	if got, want := classify("172.18.0.5", "web"), (containerProtection{}); got != want {
		t.Fatalf("web beside the connected proxy = %+v, want %+v", got, want)
	}
	if got, want := classify("172.18.0.5", "cams"), (containerProtection{Owner: "go2rtc"}); got != want {
		t.Fatalf("cams beside the connected proxy = %+v, want %+v (the owner stays)", got, want)
	}
	if got, want := classify("172.18.0.6", "replica"), (containerProtection{DockerEndpoint: true}); got != want {
		t.Fatalf("connected replica = %+v, want %+v", got, want)
	}
	// Without the list nothing proves which container is connected.
	listFails.Store(true)
	if got := classify("172.18.0.5", "web"); got != unverified {
		t.Fatalf("web with a failed list = %+v, want %+v", got, unverified)
	}
	listFails.Store(false)

	// The list marks only a container that has the connection address.
	for ip, want := range map[string]string{"127.0.0.1": "", "192.168.1.10": "", "172.18.0.5": "bbbbbbbbbbbb"} {
		restore := replaceContainerConnIP(ip)
		rec := httptest.NewRecorder()
		handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
		restore()
		var list struct {
			Containers []map[string]interface{} `json:"containers"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Containers) != 4 {
			t.Fatalf("list via %q = %d %s (%v)", ip, rec.Code, rec.Body.String(), err)
		}
		for _, c := range list.Containers {
			if (c["docker_endpoint"] == true) != (c["id"] == want) {
				t.Fatalf("list via %q: entry %v, want only %q marked", ip, c, want)
			}
		}
	}
}
