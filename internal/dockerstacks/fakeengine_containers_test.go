package dockerstacks

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"sort"
	"strings"
	"time"
)

// The container and archive routes of the fake Engine (see fakeengine_test.go).

// ---- containers --------------------------------------------------------------

func (fc *fakeContainer) listStatus() string {
	if fc.status != "" {
		return fc.status
	}
	switch fc.state {
	case "running":
		return "Up 1 minute"
	case "exited":
		return fmt.Sprintf("Exited (%d) 1 minute ago", fc.exitCode)
	}
	return "Created"
}

func (fe *fakeEngine) handleList(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	out := []map[string]any{}
	for _, id := range fe.order {
		fc := fe.containers[id]
		if fc == nil {
			continue
		}
		ports := []map[string]any{}
		for _, p := range fc.ports {
			entry := map[string]any{"PrivatePort": p.Private, "Type": p.Proto}
			if p.Public > 0 {
				entry["PublicPort"] = p.Public
			}
			if p.IP != "" {
				entry["IP"] = p.IP
			}
			ports = append(ports, entry)
		}
		out = append(out, map[string]any{
			"Id": fc.id, "Names": []string{"/" + fc.name}, "Image": fc.image, "ImageID": "sha256:" + fc.image,
			"Created": fc.created, "State": fc.state, "Status": fc.listStatusWithHealth(),
			"Ports": ports, "Labels": fc.labels,
			"NetworkSettings": map[string]any{"Networks": map[string]any{}}, "Mounts": []any{},
		})
	}
	fakeJSON(w, http.StatusOK, out)
}

func (fc *fakeContainer) listStatusWithHealth() string {
	status := fc.listStatus()
	switch fc.health {
	case "healthy":
		return status + " (healthy)"
	case "unhealthy":
		return status + " (unhealthy)"
	case "starting":
		return status + " (health: starting)"
	}
	return status
}

func (fe *fakeEngine) handleCreate(w http.ResponseWriter, r *http.Request) {
	var spec map[string]any
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		fakeMessage(w, http.StatusBadRequest, "bad body: %v", err)
		return
	}
	name := r.URL.Query().Get("name")
	fe.mu.Lock()
	defer fe.mu.Unlock()
	for _, fc := range fe.containers {
		if fc.name == name {
			fakeMessage(w, http.StatusConflict, "Conflict. The container name \"/%s\" is already in use by container %q.", name, fc.id)
			return
		}
	}
	image, _ := spec["Image"].(string)
	if _, ok := fe.images[image]; !ok {
		fakeMessage(w, http.StatusNotFound, "No such image: %s", image)
		return
	}
	fc := fe.newContainerLocked(name, spec)
	fakeJSON(w, http.StatusCreated, map[string]any{"Id": fc.id, "Warnings": []string{}})
}

func (fe *fakeEngine) handleStart(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	fc := fe.lookupLocked(r.PathValue("id"))
	if fc == nil {
		fe.mu.Unlock()
		fakeMessage(w, http.StatusNotFound, "No such container: %s", r.PathValue("id"))
		return
	}
	role := fc.labels[LabelRole]
	if role == RoleLister && fe.startFails != "" {
		msg := fe.startFails
		fe.mu.Unlock()
		fakeMessage(w, http.StatusInternalServerError, "%s", msg)
		return
	}
	fc.state = "running"
	fe.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
	go fe.behave(fc, role)
}

// behave plays the role of the helper container after its start.
func (fe *fakeEngine) behave(fc *fakeContainer, role string) {
	switch role {
	case RoleLister:
		fe.mu.Lock()
		dir, _ := fc.spec["Cmd"].([]any)[0].(string)
		var paths []string
		for p := range fe.files {
			rel, ok := strings.CutPrefix(p, dir+"/")
			if !ok || strings.Count(rel, "/") != 1 {
				continue
			}
			file := path.Base(rel)
			if file == EnvFileName || composeFileRank(file) >= 0 {
				paths = append(paths, p)
			}
		}
		listerExit, listerErr, gate := fe.listerExit, fe.listerErr, fe.listerGate
		fe.mu.Unlock()
		sort.Strings(paths)
		if len(paths) > 0 {
			fe.emit(fc, 1, strings.Join(paths, "\n")+"\n")
		}
		if listerErr != "" {
			fe.emit(fc, 2, listerErr)
		}
		if gate != nil {
			select {
			case <-gate:
			case <-fc.done: // killed
			}
		}
		fe.finish(fc, listerExit)
	case RoleRunner:
		fe.mu.Lock()
		lines, exit, gate, errText := fe.runnerLines, fe.runnerExit, fe.runnerGate, fe.runnerErr
		fe.mu.Unlock()
		for _, line := range lines {
			fe.emit(fc, 2, line+"\n") // compose writes its progress to stderr
		}
		if errText != "" {
			fe.emit(fc, 2, errText)
		}
		if gate != nil {
			<-gate
		}
		fe.finish(fc, exit)
	}
}

func (fe *fakeEngine) handleKill(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	fc := fe.lookupLocked(r.PathValue("id"))
	fe.mu.Unlock()
	if fc == nil {
		fakeMessage(w, http.StatusNotFound, "No such container: %s", r.PathValue("id"))
		return
	}
	if !fe.isRunning(fc) {
		fakeMessage(w, http.StatusConflict, "Container %s is not running", fc.name)
		return
	}
	fe.finish(fc, 137)
	w.WriteHeader(http.StatusNoContent)
}

func (fe *fakeEngine) handleInspect(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fc := fe.lookupLocked(r.PathValue("id"))
	if fc == nil {
		fakeMessage(w, http.StatusNotFound, "No such container: %s", r.PathValue("id"))
		return
	}
	var binds []string
	if hc, ok := fc.spec["HostConfig"].(map[string]any); ok {
		if raw, ok := hc["Binds"].([]any); ok {
			for _, b := range raw {
				binds = append(binds, b.(string))
			}
		}
	}
	fakeJSON(w, http.StatusOK, map[string]any{
		"Id": fc.id, "Name": "/" + fc.name,
		"Config":     map[string]any{"Image": fc.image, "Labels": fc.labels, "Tty": fc.tty},
		"HostConfig": map[string]any{"Binds": binds},
		"State": map[string]any{"Status": fc.state, "Running": fc.state == "running", "ExitCode": fc.exitCode,
			"OOMKilled": false, "Error": ""},
	})
}

func (fe *fakeEngine) handleRemove(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fc := fe.lookupLocked(r.PathValue("id"))
	if fc == nil {
		fakeMessage(w, http.StatusNotFound, "No such container: %s", r.PathValue("id"))
		return
	}
	if fc.state == "running" && r.URL.Query().Get("force") != "true" {
		fakeMessage(w, http.StatusConflict, "cannot remove a running container")
		return
	}
	delete(fe.containers, fc.id)
	fe.order = slices.DeleteFunc(fe.order, func(id string) bool { return id == fc.id })
	w.WriteHeader(http.StatusNoContent)
}

func (fe *fakeEngine) handleLogs(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	fc := fe.lookupLocked(r.PathValue("id"))
	fe.mu.Unlock()
	if fc == nil {
		fakeMessage(w, http.StatusNotFound, "No such container: %s", r.PathValue("id"))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	offset := 0
	for {
		fe.mu.Lock()
		chunk := slices.Clone(fc.logs[offset:])
		running := fc.state == "running"
		fe.mu.Unlock()
		if len(chunk) > 0 {
			_, _ = w.Write(chunk)
			offset += len(chunk)
			if flusher != nil {
				flusher.Flush()
			}
			continue
		}
		if !running {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// ---- archive -----------------------------------------------------------------

func (fe *fakeEngine) pathExistsLocked(p string) (isFile, isDir bool) {
	if _, ok := fe.files[p]; ok {
		return true, false
	}
	if fe.dirs[p] {
		return false, true
	}
	for f := range fe.files {
		if strings.HasPrefix(f, p+"/") {
			return false, true
		}
	}
	return false, false
}

func (fe *fakeEngine) archiveContainerLocked(w http.ResponseWriter, ref string) bool {
	if fe.lookupLocked(ref) == nil {
		fakeMessage(w, http.StatusNotFound, "No such container: %s", ref)
		return false
	}
	return true
}

func (fe *fakeEngine) handleArchiveGet(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	fe.mu.Lock()
	if !fe.archiveContainerLocked(w, r.PathValue("id")) {
		fe.mu.Unlock()
		return
	}
	isFile, isDir := fe.pathExistsLocked(p)
	content := slices.Clone(fe.files[p])
	fe.mu.Unlock()
	if !isFile && !isDir {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fakeMessage(w, http.StatusNotFound, "Could not find the file %s in container %s", p, r.PathValue("id"))
		return
	}
	w.Header().Set("X-Docker-Container-Path-Stat", base64.StdEncoding.EncodeToString([]byte(`{"name":"`+path.Base(p)+`"}`)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if isFile {
		_ = tw.WriteHeader(&tar.Header{Name: path.Base(p), Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(content)
	}
	_ = tw.Close()
	w.Header().Set("Content-Type", "application/x-tar")
	_, _ = w.Write(buf.Bytes())
}

func (fe *fakeEngine) handleArchivePut(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	fe.mu.Lock()
	if !fe.archiveContainerLocked(w, r.PathValue("id")) {
		fe.mu.Unlock()
		return
	}
	_, isDir := fe.pathExistsLocked(p)
	fe.putQueries = append(fe.putQueries, r.URL.RawQuery)
	fe.mu.Unlock()
	if !isDir {
		fakeMessage(w, http.StatusNotFound, "Could not find the file %s in container %s", p, r.PathValue("id"))
		return
	}
	tr := tar.NewReader(r.Body)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fakeMessage(w, http.StatusBadRequest, "bad tar: %v", err)
			return
		}
		target := path.Join(p, hdr.Name)
		fe.mu.Lock()
		switch hdr.Typeflag {
		case tar.TypeDir:
			fe.dirs[target] = true
			fe.modes[target+"/"] = hdr.Mode
		default:
			body, _ := io.ReadAll(tr)
			fe.files[target] = body
			fe.modes[target] = hdr.Mode
			if hdr.Uid != 0 || hdr.Gid != 0 {
				fe.t.Errorf("entry %s has uid/gid %d/%d, want 0/0", hdr.Name, hdr.Uid, hdr.Gid)
			}
		}
		fe.mu.Unlock()
	}
	w.WriteHeader(http.StatusOK)
}
