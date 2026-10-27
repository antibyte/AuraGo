package dockerhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestListContainersMapsTheEngineList(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	list, err := ListContainers(context.Background(), fe.client(t))
	if err != nil || len(list) != 3 {
		t.Fatalf("ListContainers = (%d containers, %v)", len(list), err)
	}
	if got := fe.requests(); !slices.Contains(got, "GET /v1.45/containers/json?all=true") {
		t.Fatalf("requests = %v, want the list with all=true", got)
	}

	web := list[0]
	if web.ID != "aaaaaaaaaaaa" || len(web.FullID) != 64 || !slices.Equal(web.Names, []string{"shop-web-1"}) ||
		web.Image != "nginx:1.27" || web.ImageID != "sha256:img-nginx" || web.State != "running" || web.Health != "healthy" || web.Created != 1700000000 {
		t.Fatalf("web = %+v", web)
	}
	if web.ComposeProject != "shop" || web.ComposeService != "web" || web.ComposeWorkingDir != "/opt/stacks/shop" {
		t.Fatalf("web compose fields = %q %q %q", web.ComposeProject, web.ComposeService, web.ComposeWorkingDir)
	}
	if !slices.Equal(web.Networks, []string{"bridge", "shop_default"}) {
		t.Fatalf("web networks = %v, want them sorted", web.Networks)
	}
	wantPorts := []Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "0.0.0.0"}, {PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "::"}, {PrivatePort: 443, Type: "tcp"}}
	if !slices.Equal(web.Ports, wantPorts) {
		t.Fatalf("web ports = %+v, want %+v", web.Ports, wantPorts)
	}
	if len(web.Mounts) != 2 || web.Mounts[0].Source != "html" || web.Mounts[0].Type != "bind" ||
		web.Mounts[1].Name != "shop_data" || web.Mounts[1].Destination != "/data" {
		t.Fatalf("web mounts = %+v; the bind source must keep only its last path element", web.Mounts)
	}
	if strings.Contains(web.Labels["com.example.db_password"], "label-secret") || web.Labels["com.example.db_password"] == "" {
		t.Fatalf("credential label = %q, want it masked", web.Labels["com.example.db_password"])
	}
	if web.Labels["com.docker.compose.service"] != "web" {
		t.Fatalf("compose label lost: %v", web.Labels)
	}

	if list[1].Health != "starting" || list[2].Health != "" || list[2].State != "exited" {
		t.Fatalf("health/state = %q, %q/%q", list[1].Health, list[2].Health, list[2].State)
	}
	// JSON arrays are never null, even for a container without ports, mounts or labels.
	raw, _ := json.Marshal(list[2])
	for _, field := range []string{`"ports":[]`, `"mounts":[]`, `"names":["solo"]`} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("solo JSON = %s, want %s", raw, field)
		}
	}
}

func TestInspectContainerRedactsLikeTheAdminInspect(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	c := fe.client(t)

	raw, err := InspectContainer(context.Background(), c, "solo")
	if err != nil {
		t.Fatalf("InspectContainer: %v", err)
	}
	out := string(raw)
	for _, leaked := range []string{"hunter2", "cmd-secret", "label-secret", "/srv/secret-dir"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("inspect leaked %q: %s", leaked, out)
		}
	}
	for _, kept := range []string{"PATH=/usr/bin", "redis-server", `"com.docker.compose.service":"cache"`, `"source":"secret-dir"`, `"network_mode":"bridge"`} {
		if !strings.Contains(out, kept) {
			t.Fatalf("inspect dropped %q: %s", kept, out)
		}
	}

	if _, err := InspectContainer(context.Background(), c, "../etc"); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("InspectContainer with a path in the id = %v, want ErrInvalidContainerRef", err)
	}
	if _, err := InspectContainer(context.Background(), c, "ghost"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("InspectContainer of an unknown container = %v (status %d), want 404", err, StatusOf(err))
	}
}

func TestContainerActionAndRemove(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	ctx := context.Background()
	for _, action := range []string{"start", "stop", "restart", "pause", "unpause", "kill"} {
		fe.json("POST", "/containers/web/"+action, http.StatusNoContent, "")
	}
	fe.json("POST", "/containers/idle/start", http.StatusNotModified, "")
	fe.json("DELETE", "/containers/web", http.StatusNoContent, "")
	fe.json("DELETE", "/containers/busy", http.StatusConflict, `{"message":"You cannot remove a running container. Stop the container before attempting removal or force remove"}`)

	for _, action := range []string{"start", "stop", "restart", "pause", "unpause", "kill"} {
		if err := ContainerAction(ctx, c, "web", action); err != nil {
			t.Fatalf("ContainerAction %s: %v", action, err)
		}
	}
	requests := strings.Join(fe.requests(), "\n")
	for _, want := range []string{
		"POST /v1.45/containers/web/start", "POST /v1.45/containers/web/stop?t=10", "POST /v1.45/containers/web/restart?t=10",
		"POST /v1.45/containers/web/pause", "POST /v1.45/containers/web/unpause", "POST /v1.45/containers/web/kill",
	} {
		if !strings.Contains(requests+"\n", want+"\n") {
			t.Fatalf("requests =\n%s\nwant %s", requests, want)
		}
	}

	if err := ContainerAction(ctx, c, "idle", "start"); err != nil {
		t.Fatalf("an already started container (304) must count as success: %v", err)
	}
	if err := ContainerAction(ctx, c, "ghost", "start"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("ContainerAction on an unknown container = %v, want 404", err)
	}
	before := len(fe.requests())
	if err := ContainerAction(ctx, c, "web", "explode"); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("unknown action = %v, want ErrInvalidAction", err)
	}
	if err := ContainerAction(ctx, c, "web/../x", "start"); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("unsafe id = %v, want ErrInvalidContainerRef", err)
	}
	if len(fe.requests()) != before {
		t.Fatal("rejected input must not reach the Engine")
	}

	if err := RemoveContainer(ctx, c, "web", false); err != nil {
		t.Fatalf("RemoveContainer: %v", err)
	}
	if err := RemoveContainer(ctx, c, "web", true); err != nil {
		t.Fatalf("RemoveContainer force: %v", err)
	}
	requests = strings.Join(fe.requests(), "\n") + "\n"
	for _, want := range []string{"DELETE /v1.45/containers/web?v=true\n", "DELETE /v1.45/containers/web?v=true&force=true\n"} {
		if !strings.Contains(requests, want) {
			t.Fatalf("requests =\n%s\nwant %q (anonymous volumes go with the container)", requests, want)
		}
	}
	if err := RemoveContainer(ctx, c, "busy", false); StatusOf(err) != http.StatusConflict {
		t.Fatalf("RemoveContainer of a running container = %v, want 409", err)
	}
}

func TestContainerLogsDemultiplexesAndClampsTail(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	ctx := context.Background()
	fe.json("GET", "/containers/web/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	var frames bytes.Buffer
	frames.Write(muxFrame(1, "2024-05-01T10:00:00.000000001Z out line\n"))
	frames.Write(muxFrame(2, "2024-05-01T10:00:01.000000001Z err line\n"))
	frames.Write(muxFrame(1, "2024-05-01T10:00:02.000000001Z last line\n"))
	fe.on("GET", "/containers/web/logs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(frames.Bytes())
	})

	got, err := ContainerLogs(ctx, c, "web", 0)
	if err != nil {
		t.Fatalf("ContainerLogs: %v", err)
	}
	want := "2024-05-01T10:00:00.000000001Z out line\n2024-05-01T10:00:01.000000001Z err line\n2024-05-01T10:00:02.000000001Z last line\n"
	if got != want {
		t.Fatalf("logs = %q, want the frames without their 8-byte headers %q", got, want)
	}
	if _, err := ContainerLogs(ctx, c, "web", 99999); err != nil {
		t.Fatal(err)
	}
	if _, err := ContainerLogs(ctx, c, "web", 7); err != nil {
		t.Fatal(err)
	}
	requests := strings.Join(fe.requests(), "\n")
	for _, want := range []string{
		"GET /v1.45/containers/web/logs?stdout=true&stderr=true&timestamps=true&tail=200",
		"GET /v1.45/containers/web/logs?stdout=true&stderr=true&timestamps=true&tail=5000",
		"GET /v1.45/containers/web/logs?stdout=true&stderr=true&timestamps=true&tail=7",
	} {
		if !strings.Contains(requests+"\n", want+"\n") {
			t.Fatalf("requests =\n%s\nwant %s", requests, want)
		}
	}

	if _, err := ContainerLogs(ctx, c, "ghost", 10); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("logs of an unknown container = %v, want 404", err)
	}
	if _, err := ContainerLogs(ctx, c, "web;rm", 10); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("logs with an unsafe id = %v, want ErrInvalidContainerRef", err)
	}
}

func TestContainerLogsOfATTYContainerArePlainText(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	fe.json("GET", "/containers/shell/json", http.StatusOK, `{"Config":{"Tty":true}}`)
	// A TTY stream is plain text, even when its first bytes look like a frame header.
	tty := "\x01\x00\x00\x00\x00\x00\x00\x05 plain\r\nsecond line\r\n"
	fe.on("GET", "/containers/shell/logs", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tty) })
	got, err := ContainerLogs(context.Background(), c, "shell", 50)
	if err != nil || got != tty {
		t.Fatalf("TTY logs = (%q, %v), want the raw stream", got, err)
	}

	// A proxy may already have demultiplexed a non-TTY stream: keep it verbatim.
	fe.json("GET", "/containers/proxied/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	fe.on("GET", "/containers/proxied/logs", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "already plain text\n") })
	if got, err := ContainerLogs(context.Background(), c, "proxied", 50); err != nil || got != "already plain text\n" {
		t.Fatalf("proxied logs = (%q, %v), want them unchanged", got, err)
	}
}

func TestContainerLogsKeepTheEndOfAnOversizedStream(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	fe.json("GET", "/containers/chatty/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	const lines = 60000 // about 1.6 MB of 28 byte lines
	fe.on("GET", "/containers/chatty/logs", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < lines; i++ {
			_, _ = w.Write(muxFrame(1, fmt.Sprintf("2024-05-01T10:00:00Z line %07d\n", i)))
		}
	})
	got, err := ContainerLogs(context.Background(), c, "chatty", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > logResultMax+len(logOmittedNote) || len(got) < logResultMax/2 {
		t.Fatalf("result is %d bytes, want it capped near 1 MiB", len(got))
	}
	if !strings.HasPrefix(got, logOmittedNote+"2024-05-01T10:00:00Z line ") {
		t.Fatalf("result starts with %q, want the omitted note and then a whole line", got[:60])
	}
	if !strings.HasSuffix(got, fmt.Sprintf("line %07d\n", lines-1)) {
		t.Fatalf("result ends with %q, want the last line kept", got[len(got)-40:])
	}
}

func TestListImagesVolumesNetworksAndInfo(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	c := fe.client(t)
	ctx := context.Background()

	images, err := ListImages(ctx, c)
	if err != nil || len(images) != 4 {
		t.Fatalf("ListImages = (%d, %v)", len(images), err)
	}
	if images[0].ID != "sha256:img-nginx" || !slices.Equal(images[0].Tags, []string{"nginx:1.27", "nginx:latest"}) || images[0].Size != 187000000 ||
		images[0].Containers != -1 || images[0].Dangling {
		t.Fatalf("nginx image = %+v", images[0])
	}
	if old := images[3]; !old.Dangling || len(old.Tags) != 0 || old.Tags == nil {
		t.Fatalf("the <none>:<none> image = %+v, want dangling with an empty tag list", old)
	}

	volumes, err := ListVolumes(ctx, c)
	if err != nil || len(volumes) != 3 || volumes[0].Name != "shop_data" || volumes[0].Driver != "local" ||
		volumes[0].Labels["com.docker.compose.project"] != "shop" || volumes[0].UsedBy == nil {
		t.Fatalf("ListVolumes = (%+v, %v)", volumes, err)
	}

	networks, err := ListNetworks(ctx, c)
	if err != nil || len(networks) != 4 {
		t.Fatalf("ListNetworks = (%d, %v)", len(networks), err)
	}
	if networks[0].Name != "bridge" || !networks[0].Builtin || !slices.Equal(networks[0].Subnets, []string{"172.17.0.0/16"}) {
		t.Fatalf("bridge network = %+v", networks[0])
	}
	if networks[2].Subnets == nil || len(networks[2].Subnets) != 0 {
		t.Fatalf("a network without IPAM config must have an empty subnet list, got %#v", networks[2].Subnets)
	}
	if shop := networks[3]; shop.Builtin || !slices.Equal(shop.Subnets, []string{"172.20.0.0/16", "fd00:1::/64"}) {
		t.Fatalf("shop network = %+v", shop)
	}

	info, err := Info(ctx, c)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	want := EngineInfo{Name: "nas", ServerVersion: "27.3.1", APIVersion: "1.45", OS: "Debian GNU/Linux 12 (bookworm)", OSType: "linux",
		Kernel: "6.1.0-26-amd64", Arch: "x86_64", DockerRootDir: "/var/lib/docker", NCPU: 4, MemTotal: 8589934592,
		Containers: 3, ContainersRunning: 2, ContainersStopped: 1, Images: 4}
	if info != want {
		t.Fatalf("Info = %+v, want %+v", info, want)
	}
	raw, _ := json.Marshal(info)
	for _, key := range []string{`"server_version"`, `"api_version"`, `"os_type"`, `"docker_root_dir"`, `"ncpu"`, `"mem_total"`, `"containers_running"`, `"containers_paused"`, `"containers_stopped"`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("EngineInfo JSON = %s, want key %s", raw, key)
		}
	}
}

func TestAnnotateCountsUsage(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	c := fe.client(t)
	ctx := context.Background()
	containers, _ := ListContainers(ctx, c)
	images, _ := ListImages(ctx, c)
	volumes, _ := ListVolumes(ctx, c)
	networks, _ := ListNetworks(ctx, c)

	Annotate(containers, images, volumes, networks)

	imageCounts := map[string]int{}
	for _, img := range images {
		imageCounts[img.ID] = img.Containers
	}
	if imageCounts["sha256:img-nginx"] != 1 || imageCounts["sha256:img-redis"] != 1 || imageCounts["sha256:img-old"] != 0 {
		t.Fatalf("image counts = %v", imageCounts)
	}
	usedBy := map[string][]string{}
	for _, v := range volumes {
		usedBy[v.Name] = v.UsedBy
	}
	if !slices.Equal(usedBy["shop_data"], []string{"shop-web-1"}) || !slices.Equal(usedBy["shop_pg"], []string{"shop-db-1"}) || len(usedBy["orphan"]) != 0 || usedBy["orphan"] == nil {
		t.Fatalf("volume users = %v", usedBy)
	}
	netCounts := map[string]int{}
	for _, n := range networks {
		netCounts[n.Name] = n.Containers
	}
	if netCounts["bridge"] != 2 || netCounts["shop_default"] != 2 || netCounts["host"] != 0 {
		t.Fatalf("network counts = %v", netCounts)
	}

	// Without a container list the counts stay unknown.
	unknown, _ := ListImages(ctx, c)
	Annotate(nil, unknown, nil, nil)
	if unknown[0].Containers != -1 {
		t.Fatalf("image count without containers = %d, want -1", unknown[0].Containers)
	}
}
