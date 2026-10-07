package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"aurago/internal/desktopstore"
	"aurago/internal/tools"
)

func (s *Server) previewRequested(r *http.Request) bool {
	_, enabled := s.previewSettings()
	return enabled || r.URL.Query().Get("isolated") == "1"
}

func serveIsolatedVMPreviewLaunch(s *Server, w http.ResponseWriter, r *http.Request, machineID string, port int, suffix string) bool {
	if !s.previewRequested(r) {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		jsonError(w, "Use the isolated preview host for app requests", http.StatusConflict)
		return true
	}
	query := r.URL.Query()
	query.Del("isolated")
	start := (&url.URL{Path: "/" + strings.TrimLeft(suffix, "/"), RawQuery: query.Encode()}).RequestURI()
	launch, err := s.issuePreviewLaunch(r, previewResource{kind: "vm", id: machineID, port: strconv.Itoa(port)}, start)
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, launch, http.StatusSeeOther)
	return true
}

func (s *Server) previewTarget(ctx context.Context, resource previewResource) (*url.URL, error) {
	if resource.kind == "vm" {
		client, err := virtualComputersClient(s)
		if err != nil {
			return nil, err
		}
		port, err := strconv.Atoi(resource.port)
		if err != nil {
			return nil, err
		}
		target, err := client.PreviewTargetURL(resource.id, port, "")
		// The pinned boringd web route is unauthenticated and passes Authorization
		// to the guest. Sending its management bearer here would disclose it.
		return target, err
	}
	if resource.kind != "store" {
		return nil, errors.New("unknown preview resource")
	}
	store, err := s.getDesktopStoreService(ctx)
	if err != nil {
		return nil, err
	}
	// OpenURL selects only persisted ports of a running managed installation.
	raw, app, err := store.OpenURL(ctx, resource.id, "127.0.0.1", false, "", resource.port)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	s.CfgMu.RLock()
	containerized, dockerHost := s.Cfg.Runtime.IsDocker, s.Cfg.Docker.Host
	s.CfgMu.RUnlock()
	if containerized {
		// Reuse a network already shared by AuraGo and the app. Never attach a
		// running app to another network or publish an additional port implicitly.
		name, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		own, err := previewContainerNetworks(ctx, dockerHost, name)
		if err != nil {
			return nil, err
		}
		appNetworks, err := previewContainerNetworks(ctx, dockerHost, app.ContainerName)
		if err != nil {
			return nil, err
		}
		containerPort := app.ContainerPort
		for _, port := range app.Ports {
			if strconv.Itoa(port.HostPort) == target.Port() {
				containerPort = port.ContainerPort
				break
			}
		}
		for network := range own {
			if ip := appNetworks[network]; net.ParseIP(ip) != nil {
				target.Host = net.JoinHostPort(ip, strconv.Itoa(containerPort))
				return target, nil
			}
		}
		return nil, errors.New("AuraGo and the app have no reachable shared Docker network")
	}
	if remote, err := url.Parse(dockerHost); err == nil && remote.Hostname() != "" && remote.Scheme != "unix" && remote.Scheme != "npipe" {
		// A published loopback port is deliberately unreachable on a remote host.
		remoteIP := net.ParseIP(remote.Hostname())
		if !strings.EqualFold(remote.Hostname(), "localhost") && (remoteIP == nil || !remoteIP.IsLoopback()) {
			if app.BindMode != desktopstore.BindModeLAN {
				return nil, errors.New("remote Docker preview requires a reachable published binding")
			}
			target.Host = net.JoinHostPort(remote.Hostname(), target.Port())
			return target, nil
		}
	}
	return target, nil
}

func previewContainerNetworks(ctx context.Context, dockerHost, container string) (map[string]string, error) {
	body, code, err := tools.DockerRequestContext(ctx, tools.DockerConfig{Host: dockerHost}, http.MethodGet, "/containers/"+url.PathEscape(container)+"/json", "")
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, errors.New("unable to inspect managed preview network")
	}
	var info struct {
		NetworkSettings struct {
			Networks map[string]struct{ IPAddress string }
		}
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	networks := make(map[string]string, len(info.NetworkSettings.Networks))
	for name, network := range info.NetworkSettings.Networks {
		networks[name] = network.IPAddress
	}
	return networks, nil
}
