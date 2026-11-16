package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/dockerutil"
)

type ziroDocker struct {
	codeStudioDockerAdapter
	client                              *dockerutil.Client
	jobs                                string
	mountSource, mountTarget, mountType string
}

func (s *Server) newZiroDocker(ctx context.Context, cfg desktop.Config, jobs string) (*ziroDocker, error) {
	d := &ziroDocker{codeStudioDockerAdapter: newCodeStudioDockerAdapter(cfg, s.Logger), jobs: jobs, client: dockerutil.NewClient(cfg.DockerHost, 30*time.Second)}
	if !s.ConfigSnapshot().Runtime.IsDocker {
		return d, nil
	}
	id, err := resolveDecisionsDockerSelf(ctx, d.cfg, readContainerSelfSignals(true))
	if err != nil {
		d.client.CloseIdleConnections()
		return nil, errors.New("Cannot prove AuraGo's own container for the export snapshot")
	}
	var own struct {
		ID     string `json:"Id"`
		Mounts []struct {
			Type, Name, Source, Destination string
			RW                              bool
		}
	}
	_, err = d.client.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(id)+"/json", nil, &own)
	if err != nil || own.ID != id {
		d.client.CloseIdleConnections()
		return nil, errors.New("Cannot resolve export storage mount")
	}
	for _, m := range own.Mounts {
		target := path.Clean(m.Destination)
		if !m.RW || (m.Type != "bind" && m.Type != "volume") || (jobs != target && !strings.HasPrefix(jobs, target+"/")) || len(target) <= len(d.mountTarget) {
			continue
		}
		d.mountTarget, d.mountSource, d.mountType = target, m.Source, m.Type
		if m.Type == "volume" {
			d.mountSource = m.Name
		}
	}
	if d.mountSource == "" {
		d.client.CloseIdleConnections()
		return nil, errors.New("Export storage must be inside a writable AuraGo bind mount or named volume")
	}
	return d, nil
}

func (d *ziroDocker) CreateContainer(ctx context.Context, req desktop.CodeDockerCreateRequest) (string, error) {
	if req.Image != desktop.ZiroExportImage || !strings.HasPrefix(req.Name, "aurago-ziro-export-") || len(req.Volumes) != 1 {
		return "", errors.New("Invalid owned export container")
	}
	job := strings.TrimSuffix(req.Volumes[0], ":/job")
	rel, err := filepath.Rel(d.jobs, job)
	if err != nil || !filepath.IsLocal(rel) || strings.ContainsAny(rel, "/\\") || rel == "." {
		return "", errors.New("Invalid export job directory")
	}
	mount := map[string]any{"Type": "bind", "Source": job, "Target": "/job"}
	if d.mountType != "" {
		subpath := strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(job), d.mountTarget), "/")
		if d.mountType == "volume" {
			version, err := d.client.APIVersion(ctx)
			minor, parseErr := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(version, "v"), "1."))
			if err != nil || parseErr != nil || minor < 45 {
				return "", errors.New("Named-volume export snapshots require Docker API 1.45 or newer")
			}
			mount = map[string]any{"Type": "volume", "Source": d.mountSource, "Target": "/job", "VolumeOptions": map[string]any{"Subpath": subpath, "NoCopy": true}}
		} else {
			mount["Source"] = path.Join(d.mountSource, subpath)
		}
	}
	payload := map[string]any{"Image": desktop.ZiroExportImage, "Cmd": req.Cmd, "Env": req.Env, "User": req.User, "HostConfig": map[string]any{
		"Mounts": []any{mount}, "NetworkMode": "none", "ReadonlyRootfs": true, "SecurityOpt": []string{"no-new-privileges:true"}, "CapDrop": []string{"ALL"},
		"Tmpfs": req.Tmpfs, "Memory": int64(2048) << 20, "MemorySwap": int64(2048) << 20, "NanoCpus": int64(2e9), "PidsLimit": 128, "RestartPolicy": map[string]any{"Name": "no"}}}
	var created struct {
		ID string `json:"Id"`
	}
	_, err = d.client.DoJSON(ctx, http.MethodPost, "containers/create?name="+url.QueryEscape(req.Name), payload, &created)
	return created.ID, err
}

func (d *ziroDocker) InspectContainer(ctx context.Context, name string) (desktop.CodeDockerInspect, error) {
	var raw json.RawMessage
	_, err := d.client.DoJSON(ctx, http.MethodGet, "containers/"+url.PathEscape(name)+"/json", nil, &raw)
	var result desktop.CodeDockerInspect
	if err == nil {
		err = json.Unmarshal(raw, &result)
	}
	return result, err
}

func (d *ziroDocker) ContainerAction(ctx context.Context, name, action string) error {
	if !strings.HasPrefix(name, "aurago-ziro-export-") {
		return errors.New("Invalid export container")
	}
	method, suffix := http.MethodPost, "/"+action
	if action == "remove" {
		method, suffix = http.MethodDelete, "?force=true&v=true"
	} else if action != "kill" && action != "start" {
		return errors.New("Invalid export lifecycle action")
	}
	_, err := d.client.DoJSON(ctx, method, "containers/"+url.PathEscape(name)+suffix, nil, nil)
	return err
}
