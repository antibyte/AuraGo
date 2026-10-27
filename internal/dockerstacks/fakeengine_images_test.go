package dockerstacks

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// The image, volume and system df routes of the fake Engine (see fakeengine_test.go).

// ---- images ------------------------------------------------------------------

func (fe *fakeEngine) handleImageInspect(w http.ResponseWriter, r *http.Request) {
	ref, _ := url.PathUnescape(r.PathValue("ref"))
	fe.mu.Lock()
	defer fe.mu.Unlock()
	id, ok := fe.images[ref]
	if !ok {
		fakeMessage(w, http.StatusNotFound, "No such image: %s", ref)
		return
	}
	fakeJSON(w, http.StatusOK, map[string]string{"Id": id, "Os": fe.os, "Architecture": fe.arch})
}

func (fe *fakeEngine) handleImagePull(w http.ResponseWriter, r *http.Request) {
	from, tag := r.URL.Query().Get("fromImage"), r.URL.Query().Get("tag")
	fe.mu.Lock()
	fail := fe.pullFails
	if fail == "" {
		ref := from + ":" + tag
		if strings.HasPrefix(tag, "sha256:") {
			ref = from + "@" + tag
		}
		fe.pulled = append(fe.pulled, ref)
		fe.images[ref] = "sha256:pulledimage"
	}
	fe.mu.Unlock()
	if fail != "" {
		fakeMessage(w, http.StatusInternalServerError, "%s", fail)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	for _, line := range []string{
		`{"status":"Pulling from library/` + from + `","id":"` + tag + `"}`,
		`{"status":"Pulling fs layer","progressDetail":{},"id":"layer1"}`,
		`{"status":"Downloading","progressDetail":{"current":1000,"total":4000},"progress":"[=>  ]","id":"layer1"}`,
		`{"status":"Downloading","progressDetail":{"current":4000,"total":4000},"progress":"[===>]","id":"layer1"}`,
		`{"status":"Pull complete","progressDetail":{},"id":"layer1"}`,
		`{"status":"Status: Downloaded newer image for ` + from + `@` + tag + `"}`,
	} {
		_, _ = io.WriteString(w, line+"\n")
	}
}

func (fe *fakeEngine) handleImageSave(w http.ResponseWriter, r *http.Request) {
	ref, _ := url.PathUnescape(r.PathValue("ref"))
	fe.mu.Lock()
	_, ok := fe.images[ref]
	if ok {
		fe.savedRefs = append(fe.savedRefs, ref)
	}
	fe.mu.Unlock()
	if !ok {
		fakeMessage(w, http.StatusNotFound, "No such image: %s", ref)
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	_, _ = io.WriteString(w, "FAKE-IMAGE-TAR:"+ref)
}

func (fe *fakeEngine) handleImageLoad(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	fe.mu.Lock()
	fe.loadedBody = body
	reply := fe.loadReply
	fe.images["sha256:loadedimage"] = "sha256:loadedimage"
	fe.mu.Unlock()
	if ct := r.Header.Get("Content-Type"); ct != "application/x-tar" {
		fe.t.Errorf("image load Content-Type = %q, want application/x-tar", ct)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, reply)
}

func (fe *fakeEngine) handleImageTag(w http.ResponseWriter, r *http.Request) {
	ref, _ := url.PathUnescape(r.PathValue("ref"))
	fe.mu.Lock()
	defer fe.mu.Unlock()
	id, ok := fe.images[ref]
	if !ok {
		fakeMessage(w, http.StatusNotFound, "No such image: %s", ref)
		return
	}
	fe.images[r.URL.Query().Get("repo")+":"+r.URL.Query().Get("tag")] = id
	w.WriteHeader(http.StatusCreated)
}

// ---- volumes -----------------------------------------------------------------

func (fe *fakeEngine) handleVolumes(w http.ResponseWriter, r *http.Request) {
	project := ""
	if raw := r.URL.Query().Get("filters"); raw != "" {
		var filters map[string][]string
		if err := json.Unmarshal([]byte(raw), &filters); err != nil {
			fakeMessage(w, http.StatusBadRequest, "bad filters: %v", err)
			return
		}
		for _, label := range filters["label"] {
			if v, ok := strings.CutPrefix(label, "com.docker.compose.project="); ok {
				project = v
			}
		}
	}
	fe.mu.Lock()
	defer fe.mu.Unlock()
	list := []map[string]any{}
	for _, v := range fe.volumes {
		if project != "" && v.Project != project {
			continue
		}
		list = append(list, map[string]any{"Name": v.Name, "Driver": v.Driver})
	}
	fakeJSON(w, http.StatusOK, map[string]any{"Volumes": list, "Warnings": nil})
}

func (fe *fakeEngine) handleDF(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	status := fe.dfStatus
	fe.mu.Unlock()
	if status != 0 {
		fakeMessage(w, status, "df failed")
		return
	}
	if r.URL.Query().Get("type") != "volume" {
		fakeMessage(w, http.StatusBadRequest, "the fake serves only /system/df?type=volume")
		return
	}
	fe.mu.Lock()
	defer fe.mu.Unlock()
	list := []map[string]any{}
	for _, v := range fe.volumes {
		list = append(list, map[string]any{"Name": v.Name, "UsageData": map[string]any{"Size": v.Size, "RefCount": 0}})
	}
	fakeJSON(w, http.StatusOK, map[string]any{"Volumes": list})
}
