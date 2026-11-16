package desktop

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ZiroExportImage is published only after both export contract smokes succeed.
const ZiroExportImage = "ghcr.io/antibyte/aurago-kicad-export:kicad-10.0.8-1"

var ziroExportSlot = make(chan struct{}, 1)

// RunZiroExport uses only saved Shared files; an empty project runs the capability probe.
func (s *Service) RunZiroExport(ctx context.Context, docker CodeContainerDocker, jobsRoot, project, operation string, maxBytes int64) ([]byte, string, error) {
	select {
	case ziroExportSlot <- struct{}{}:
		defer func() { <-ziroExportSlot }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	if operation != "netlist" && operation != "manufacturing" && operation != "probe" {
		return nil, "", errors.New("Choose netlist or manufacturing")
	}
	if maxBytes < 1 || maxBytes > 64<<20 {
		return nil, "", errors.New("Invalid export size limit")
	}
	if err := os.MkdirAll(jobsRoot, 0700); err != nil {
		return nil, "", errors.New("Cannot prepare export storage")
	}
	job, err := os.MkdirTemp(jobsRoot, "job-")
	if err != nil {
		return nil, "", errors.New("Cannot prepare export job")
	}
	defer os.RemoveAll(job)
	input := ""
	if operation != "probe" {
		input, err = s.snapshotZiroProject(ctx, project, operation, filepath.Join(job, "source"), maxBytes)
		if err != nil {
			return nil, "", err
		}
	}
	request, _ := json.Marshal(map[string]any{"operation": operation, "input": input, "max_bytes": maxBytes})
	if err := os.WriteFile(filepath.Join(job, "request.json"), request, 0600); err != nil {
		return nil, "", errors.New("Cannot prepare export request")
	}
	if err := docker.EnsureImage(ctx, ZiroExportImage); err != nil {
		return nil, "", errors.New("KiCad export image unavailable; install the versioned export image or update AuraGo")
	}
	name := "aurago-ziro-export-" + strings.ToLower(rand.Text())
	user := "0:0"
	if os.Getuid() >= 0 && os.Getgid() >= 0 {
		user = fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	}
	// Register cleanup before create: a lost create response can still leave a container.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = docker.ContainerAction(cleanup, name, "kill")
		_ = docker.ContainerAction(cleanup, name, "remove")
	}()
	_, err = docker.CreateContainer(ctx, CodeDockerCreateRequest{Name: name, Image: ZiroExportImage,
		Volumes: []string{job + ":/job"}, Env: []string{"HOME=/tmp", "XDG_CONFIG_HOME=/tmp/config"},
		Cmd: []string{"python3", "/opt/aurago/export.py"}, User: user, Restart: "no", NetworkMode: "none",
		ReadonlyRootfs: true, SecurityOpt: []string{"no-new-privileges:true"}, CapDrop: []string{"ALL"},
		Resources: &CodeContainerResources{MemoryMB: 2048, CPUCores: 2, PidsLimit: 128}, Tmpfs: map[string]string{"/tmp": "rw,nosuid,nodev,size=256m"}})
	if err != nil {
		return nil, "", errors.New("Cannot create isolated KiCad export container")
	}
	if err = ctx.Err(); err != nil {
		return nil, "", err
	}
	if err = docker.ContainerAction(ctx, name, "start"); err != nil {
		return nil, "", errors.New("Cannot start KiCad export container")
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err = ctx.Err(); err != nil {
			return nil, "", err
		}
		state, inspectErr := docker.InspectContainer(ctx, name)
		if inspectErr != nil {
			return nil, "", errors.New("Cannot inspect export; job was stopped, do not assume completion")
		}
		if !state.State.Running {
			if state.State.ExitCode != 0 {
				message, _ := readZiroOutput(job, "error.txt", 4096)
				return nil, "", fmt.Errorf("KiCad export failed: %s", strings.TrimSpace(string(message)))
			}
			break
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-ticker.C:
		}
	}
	if operation == "probe" {
		data, err := readZiroOutput(job, "probe.json", 4096)
		var probe struct {
			Version  string `json:"version"`
			Protocol int    `json:"protocol"`
		}
		if err != nil || json.Unmarshal(data, &probe) != nil || probe.Version != "10.0.8" || probe.Protocol != 1 {
			return nil, "", errors.New("Store update required: incompatible KiCad export worker")
		}
		return data, ".json", nil
	}
	extension := ".net"
	if operation == "manufacturing" {
		extension = ".zip"
	}
	data, err := readZiroOutput(job, "result"+extension, maxBytes)
	if err != nil || len(data) == 0 {
		return nil, "", errors.New("KiCad export result is missing, unsafe or exceeds the file limit")
	}
	return data, extension, nil
}

func readZiroOutput(job, name string, limit int64) ([]byte, error) {
	f, err := openFileNoFollow(filepath.Join(job, name), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("Invalid export output")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(data)) > limit {
		return nil, errors.New("Export output exceeds limit")
	}
	return data, err
}

func (s *Service) snapshotZiroProject(ctx context.Context, project, operation, destination string, maxBytes int64) (string, error) {
	if !strings.HasPrefix(project, "Shared/") || path.Clean(project) != project || strings.ContainsAny(project, "\\:\x00") || !filepath.IsLocal(project) || path.Dir(project) == "Shared" {
		return "", errors.New("Choose a saved KiCad project in its own directory below Shared")
	}
	input := path.Base(project)
	ext := ".kicad_sch"
	if operation == "manufacturing" {
		ext = ".kicad_pcb"
	}
	if path.Ext(input) == ".kicad_pro" {
		input = strings.TrimSuffix(input, ".kicad_pro") + ext
	}
	if path.Ext(input) != ext {
		return "", fmt.Errorf("This export requires a saved %s file", ext)
	}
	desktopMutationMu.RLock()
	defer desktopMutationMu.RUnlock()
	root, err := os.OpenRoot(s.Config().WorkspaceDir)
	if err != nil {
		return "", errors.New("Cannot open Shared workspace")
	}
	defer root.Close()
	source, dir, err := openDesktopSearchDirectory(root, filepath.FromSlash(path.Dir(project)))
	if err != nil {
		return "", errors.New("Project directory is missing or contains links")
	}
	defer source.Close()
	dir.Close()
	if err = os.Mkdir(destination, 0700); err != nil {
		return "", errors.New("Cannot stage project")
	}
	count, total := 0, int64(0)
	err = fs.WalkDir(source.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("Cannot read project directory")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 2048 || strings.Count(name, "/") > 32 {
			return errors.New("Project snapshot exceeds 2048 entries or 32 directory levels")
		}
		if strings.ContainsAny(name, "\\:\x00") {
			return errors.New("Invalid project filename")
		}
		info, err := source.Lstat(name)
		if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
			return errors.New("Project snapshot refuses links and special files")
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(destination, filepath.FromSlash(name)), 0700)
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".kicad_pro", ".kicad_sch", ".kicad_pcb", ".kicad_sym", ".kicad_mod", ".kicad_wks", ".lib", ".dcm", ".mod":
		default:
			if path.Base(name) != "sym-lib-table" && path.Base(name) != "fp-lib-table" {
				return nil
			}
		}
		f, err := source.Open(name)
		if err != nil {
			return errors.New("Cannot read saved project file")
		}
		defer f.Close()
		opened, err := f.Stat()
		if err != nil || !os.SameFile(info, opened) {
			return errors.New("Project changed while creating snapshot")
		}
		data, err := io.ReadAll(io.LimitReader(desktopContextReader{ctx, f}, maxBytes-total+1))
		total += int64(len(data))
		if err != nil || total > maxBytes {
			return errors.New("Project snapshot exceeds the file limit")
		}
		after, err := f.Stat()
		if err != nil || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
			return errors.New("Project changed while creating snapshot")
		}
		return os.WriteFile(filepath.Join(destination, filepath.FromSlash(name)), data, 0600)
	})
	if err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Join(destination, input))
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Saved %s file is missing; save the project in Ziro first", ext)
	}
	return input, nil
}
