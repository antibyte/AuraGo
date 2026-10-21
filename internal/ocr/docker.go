package ocr

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

type DockerRuntime struct {
	scope, nonce                               string
	mu                                         sync.Mutex
	policy                                     func() Policy
	self                                       func(context.Context, string) (string, error)
	image                                      string
	engine                                     *dockerutil.Client
	container, network, selfID, baseURL, token string
	client                                     *http.Client
}

func NewDockerRuntime(policy func() Policy, self func(context.Context, string) (string, error), image string) *DockerRuntime {
	return &DockerRuntime{policy: policy, self: self, image: image, nonce: id(), client: &http.Client{
		Timeout: FileTimeout + 10*time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{Proxy: nil},
	}}
}

func (d *DockerRuntime) Start(ctx context.Context, install bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	p := d.policy()
	if !p.Enabled {
		return ErrDisabled
	}
	if !p.DockerEnabled || p.DockerReadOnly {
		return errors.New("Docker control is disabled or read-only")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return errors.New("unsupported OCR platform")
	}
	image := d.image
	if image == "" {
		image = ReleaseImage
	}
	if image == "" {
		return fmt.Errorf("%w: release image is not published", ErrNotReady)
	}
	if !(strings.HasPrefix(image, "sha256:") || strings.Contains(image, "@sha256:")) {
		return ErrInvalid
	}
	if err := d.stopLocked(ctx); err != nil {
		return err
	}
	d.engine = dockerutil.NewClient(p.DockerHost, 20*time.Second)
	if err := d.prepareInstance(ctx, p); err != nil {
		return err
	}
	var metadata struct{ ID, OS, Architecture string }
	code, err := d.engine.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(image)+"/json", nil, &metadata)
	// An offline OCI import may expose only its immutable platform config ID.
	if err != nil && code == http.StatusNotFound && d.image == "" {
		code, err = d.engine.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(releaseConfigIDs[runtime.GOARCH])+"/json", nil, &metadata)
	}
	if err != nil && code == http.StatusNotFound && install {
		request, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(image)), nil)
		if reqErr != nil {
			return reqErr
		}
		response, pullErr := d.engine.HTTPClientWithTimeout(10 * time.Minute).Do(request)
		if pullErr != nil {
			return pullErr
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return errors.New("OCR image pull failed")
		}
		pullErr = dockerutil.DrainJSONMessages(response.Body)
		response.Body.Close()
		if pullErr != nil {
			return pullErr
		}
		_, err = d.engine.DoJSON(ctx, http.MethodGet, "images/"+url.PathEscape(image)+"/json", nil, &metadata)
	}
	if err != nil {
		return fmt.Errorf("%w: OCR image unavailable", ErrNotReady)
	}
	if metadata.OS != "linux" || metadata.Architecture != runtime.GOARCH {
		return fmt.Errorf("%w: OCR image platform %s/%s", ErrInvalid, metadata.OS, metadata.Architecture)
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	d.token = hex.EncodeToString(token[:])
	security.RegisterSensitive(d.token)
	selfID := d.selfID
	labels := map[string]string{"aurago.managed": OwnerLabel, "aurago.component": "indra-ocr", "aurago.ocr.instance": d.scope}
	var network struct {
		ID string `json:"Id"`
	}
	_, err = d.engine.DoJSON(ctx, http.MethodPost, "networks/create", map[string]any{
		"Name": ContainerName + "-" + id(), "Internal": p.InDocker, "Labels": labels,
		"Options": map[string]string{"com.docker.network.bridge.enable_ip_masquerade": "false"}}, &network)
	d.network = network.ID
	if err != nil || network.ID == "" {
		return errors.New("OCR private network unavailable")
	}
	if selfID != "" {
		_, err = d.engine.DoJSON(ctx, http.MethodPost, "networks/"+d.network+"/connect", map[string]any{"Container": selfID}, nil)
		if err != nil {
			return err
		}
	}
	host := map[string]any{"NetworkMode": d.network, "ReadonlyRootfs": true, "Memory": int64(2 << 30), "MemorySwap": int64(2 << 30),
		"NanoCpus": int64(4_000_000_000), "PidsLimit": int64(64), "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"},
		"Tmpfs": map[string]string{"/tmp": fmt.Sprintf("rw,noexec,nosuid,size=%d,mode=1777", WorkerTempBytes)}, "Init": true}
	if selfID == "" {
		host["PortBindings"] = map[string]any{"8766/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": ""}}}
	}
	var container struct {
		ID string `json:"Id"`
	}
	_, err = d.engine.DoJSON(ctx, http.MethodPost, "containers/create", map[string]any{
		"Image": metadata.ID, "User": "1001:1001", "Labels": labels, "Env": []string{"OCR_SERVICE_TOKEN=" + d.token, "HF_HUB_OFFLINE=1"},
		"ExposedPorts": map[string]any{"8766/tcp": map[string]any{}}, "HostConfig": host,
	}, &container)
	d.container = container.ID
	if err != nil || container.ID == "" {
		return errors.New("OCR container creation failed")
	}
	if _, err = d.engine.DoJSON(ctx, http.MethodPost, "containers/"+d.container+"/start", nil, nil); err != nil {
		return err
	}
	var info struct {
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string
			}
			Networks map[string]struct{ IPAddress string }
		}
	}
	if _, err = d.engine.DoJSON(ctx, http.MethodGet, "containers/"+d.container+"/json", nil, &info); err != nil {
		return err
	}
	if selfID == "" {
		ports := info.NetworkSettings.Ports["8766/tcp"]
		if len(ports) != 1 || ports[0].HostIP != "127.0.0.1" || ports[0].HostPort == "" {
			return fmt.Errorf("%w: OCR loopback port unavailable (%+v)", ErrInvalid, ports)
		}
		d.baseURL = "http://127.0.0.1:" + ports[0].HostPort
	} else {
		for _, network := range info.NetworkSettings.Networks {
			if network.IPAddress != "" {
				d.baseURL = "http://" + network.IPAddress + ":8766"
			}
		}
		if d.baseURL == "" {
			return ErrInvalid
		}
	}
	readyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		request, _ := http.NewRequestWithContext(readyCtx, http.MethodGet, d.baseURL+"/healthz", nil)
		request.Header.Set("Authorization", "Bearer "+d.token)
		response, err := d.client.Do(request)
		if err == nil {
			var health struct {
				Version, ModelRevision string
				Ready                  bool
			}
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&health)
			response.Body.Close()
			if response.StatusCode == http.StatusOK && decodeErr == nil && health.Ready && health.Version == Version && health.ModelRevision == ModelRevision {
				return nil
			}
		}
		select {
		case <-readyCtx.Done():
			return readyCtx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (d *DockerRuntime) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.engine == nil {
		p := d.policy()
		if !p.DockerEnabled || p.DockerReadOnly {
			return nil
		}
		d.engine = dockerutil.NewClient(p.DockerHost, 20*time.Second)
		if err := d.prepareInstance(ctx, p); err != nil {
			return err
		}
	}
	return d.stopLocked(ctx)
}

// Reap only this installation's parser resources after an unclean server exit.
func (d *DockerRuntime) prepareInstance(ctx context.Context, p Policy) error {
	key := p.InstanceKey
	if key == "" {
		key = d.nonce
	}
	d.selfID = ""
	if p.InDocker {
		if d.self == nil {
			return ErrInvalid
		}
		selfID, err := d.self(ctx, p.DockerHost)
		if err != nil || selfID == "" {
			return errors.New("AuraGo container identity unavailable")
		}
		var origin struct{ Name string }
		if _, err = d.engine.DoJSON(ctx, "GET", "containers/"+url.PathEscape(selfID)+"/json", nil, &origin); err != nil || origin.Name == "" {
			return errors.New("AuraGo container identity unavailable")
		}
		d.selfID = selfID
		key += "|" + origin.Name
	}
	d.scope = fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	filter, _ := json.Marshal(map[string][]string{"label": {"aurago.managed=" + OwnerLabel, "aurago.component=indra-ocr", "aurago.ocr.instance=" + d.scope}})
	query := "filters=" + url.QueryEscape(string(filter))
	var containers []struct {
		ID     string
		Labels map[string]string
	}
	if _, err := d.engine.DoJSON(ctx, "GET", "containers/json?all=1&"+query, nil, &containers); err != nil {
		return err
	}
	for _, item := range containers {
		if item.ID == "" || item.Labels["aurago.managed"] != OwnerLabel || item.Labels["aurago.component"] != "indra-ocr" || item.Labels["aurago.ocr.instance"] != d.scope {
			return ErrInvalid
		}
		if _, err := d.engine.DoJSON(ctx, "DELETE", "containers/"+url.PathEscape(item.ID)+"?force=true", nil, nil); err != nil {
			return err
		}
	}
	var networks []struct {
		ID     string
		Labels map[string]string
	}
	if _, err := d.engine.DoJSON(ctx, "GET", "networks?"+query, nil, &networks); err != nil {
		return err
	}
	for _, item := range networks {
		if item.ID == "" || item.Labels["aurago.managed"] != OwnerLabel || item.Labels["aurago.component"] != "indra-ocr" || item.Labels["aurago.ocr.instance"] != d.scope {
			return ErrInvalid
		}
		if d.selfID != "" {
			_, _ = d.engine.DoJSON(ctx, "POST", "networks/"+url.PathEscape(item.ID)+"/disconnect", map[string]any{"Container": d.selfID, "Force": true}, nil)
		}
		if _, err := d.engine.DoJSON(ctx, "DELETE", "networks/"+url.PathEscape(item.ID), nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func (d *DockerRuntime) stopLocked(ctx context.Context) error {
	d.baseURL = ""
	if d.engine == nil {
		return nil
	}
	if d.container != "" {
		code, err := d.engine.DoJSON(ctx, http.MethodDelete, "containers/"+d.container+"?force=true", nil, nil)
		if err != nil && code != http.StatusNotFound {
			return err
		}
		d.container = ""
	}
	if d.network != "" {
		if d.selfID != "" {
			_, _ = d.engine.DoJSON(ctx, http.MethodPost, "networks/"+d.network+"/disconnect", map[string]any{"Container": d.selfID, "Force": true}, nil)
		}
		code, err := d.engine.DoJSON(ctx, http.MethodDelete, "networks/"+d.network, nil, nil)
		if err != nil && code != http.StatusNotFound {
			return err
		}
		d.network = ""
	}
	d.token = ""
	d.engine.CloseIdleConnections()
	return nil
}
func (d *DockerRuntime) Convert(ctx context.Context, path, mime, mode string, emit func(Page) error) error {
	d.mu.Lock()
	baseURL, token := d.baseURL, d.token
	d.mu.Unlock()
	if baseURL == "" {
		return ErrNotReady
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	key := id()
	defer func() {
		cancelCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(cancelCtx, http.MethodDelete, baseURL+"/v1/ocr/"+key, nil)
		if err != nil {
			return
		}
		request.Header.Set("Authorization", "Bearer "+token)
		if response, err := d.client.Do(request); err == nil {
			response.Body.Close()
		}
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/ocr?mode="+url.QueryEscape(mode)+"&id="+key, file)
	if err != nil {
		return err
	}
	request.ContentLength = info.Size()
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", mime)
	response, err := d.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: OCR transport unavailable", ErrNotReady)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusServiceUnavailable {
			return ErrNotReady
		}
		return errors.New("OCR runtime rejected input")
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, MaxResultBytes+1))
	scanner.Buffer(make([]byte, 64<<10), int(MaxResultBytes))
	var read int64
	completed := false
	for scanner.Scan() {
		read += int64(len(scanner.Bytes())) + 1
		if read > MaxResultBytes {
			return ErrLimit
		}
		var event struct {
			Type string `json:"type"`
			Page Page   `json:"page"`
			Code string `json:"code"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("%w: invalid OCR event: %v", ErrInvalid, err)
		}
		switch event.Type {
		case "page":
			if completed {
				return ErrInvalid
			}
			if err := emit(event.Page); err != nil {
				return err
			}
		case "done":
			if completed {
				return ErrInvalid
			}
			completed = true
		case "error":
			return errors.New("OCR page processing failed")
		default:
			return ErrInvalid
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !completed {
		return errors.New("OCR response incomplete")
	}
	return nil
}
