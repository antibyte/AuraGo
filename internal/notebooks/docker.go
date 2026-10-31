package notebooks

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/kaggle"
)

const RuntimeImage = "ghcr.io/antibyte/aurago-kaggle-notebook:protocol-1-20261026"
const containerPrefix = "aurago-kaggle-"

type Docker struct {
	client   *dockerutil.Client
	image    string
	instance string
}

func NewDocker(host, storageRoot string) *Docker {
	id := sha256.Sum256([]byte(storageRoot))
	return &Docker{client: dockerutil.NewClient(host, 30*time.Second), image: RuntimeImage, instance: hex.EncodeToString(id[:16])}
}
func (d *Docker) EnsureImage(ctx context.Context) error {
	status, err := d.client.DoJSON(ctx, "GET", "/images/"+url.PathEscape(d.image)+"/json", nil, &struct{}{})
	if err == nil && status == 200 {
		return nil
	}
	if status != 404 {
		return errors.New("Docker image inspection failed")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", dockerutil.Endpoint("/images/create?fromImage="+url.QueryEscape(d.image)), nil)
	if err != nil {
		return err
	}
	resp, err := d.client.StreamingHTTPClient().Do(req)
	if err != nil {
		return errors.New("notebook image download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("notebook image is not available in the registry")
	}
	return dockerutil.DrainJSONMessages(resp.Body)
}
func (d *Docker) Create(ctx context.Context, id string, c config.KaggleLocalConfig, internet bool) (string, error) {
	network := "none"
	if internet {
		network = "bridge"
	}
	body := map[string]any{"Image": d.image, "User": "1000:100", "WorkingDir": "/workspace", "Env": []string{"HOME=/tmp/home", "JUPYTER_RUNTIME_DIR=/tmp/runtime", "IPYTHONDIR=/tmp/ipython"}, "Labels": map[string]string{"org.aurago.managed": "kaggle-notebook", "org.aurago.session": id, "org.aurago.instance": d.instance}, "HostConfig": map[string]any{"NetworkMode": network, "ReadonlyRootfs": true, "Memory": int64(c.MemoryMB) << 20, "NanoCpus": int64(c.CPUCores) * 1000000000, "PidsLimit": 256, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "Tmpfs": map[string]string{"/tmp": "rw,nosuid,nodev,size=268435456,mode=1777", "/workspace": fmt.Sprintf("rw,nosuid,nodev,size=%d,uid=1000,gid=100,mode=0700", int64(c.MemoryMB)<<19)}}}
	var created struct {
		ID string `json:"Id"`
	}
	status, err := d.client.DoJSON(ctx, "POST", "/containers/create?name="+containerPrefix+id, body, &created)
	if err != nil || status != 201 || created.ID == "" {
		return "", errors.New("notebook container creation failed")
	}
	status, err = d.client.DoJSON(ctx, "POST", "/containers/"+created.ID+"/start", nil, nil)
	if err != nil || status != 204 {
		d.Remove(context.WithoutCancel(ctx), created.ID)
		return "", errors.New("notebook container start failed")
	}
	return created.ID, nil
}
func (d *Docker) Remove(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	status, err := d.client.DoJSON(ctx, "DELETE", "/containers/"+url.PathEscape(id)+"?force=1&v=1", nil, nil)
	if err != nil || status != 204 && status != 404 {
		return errors.New("notebook container cleanup failed")
	}
	return nil
}
func (d *Docker) Cleanup(ctx context.Context) error {
	encoded, _ := json.Marshal(map[string][]string{"label": {"org.aurago.managed=kaggle-notebook", "org.aurago.instance=" + d.instance}})
	filter := url.QueryEscape(string(encoded))
	var rows []struct {
		ID string `json:"Id"`
	}
	status, err := d.client.DoJSON(ctx, "GET", "/containers/json?all=1&filters="+filter, nil, &rows)
	if err != nil || status != 200 {
		return errors.New("notebook runtime recovery failed")
	}
	for _, r := range rows {
		if err = d.Remove(ctx, r.ID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Docker) CopyIn(ctx context.Context, id string, storage *kaggle.Service, project string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	if len(files) > 1000 {
		return errors.New("too many selected files")
	}
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		tw := tar.NewWriter(writer)
		var copyErr error
		defer func() {
			if copyErr == nil {
				copyErr = tw.Close()
			}
			writer.CloseWithError(copyErr)
			done <- copyErr
		}()
		for _, name := range files {
			f, e := storage.OpenFile(project, name)
			if e != nil {
				copyErr = e
				return
			}
			i, e := f.Stat()
			if e != nil {
				f.Close()
				copyErr = e
				return
			}
			if i.Size() > int64(storage.Snapshot().Config.MaxTransferMB)<<20 {
				f.Close()
				copyErr = errors.New("selected file exceeds transfer limit")
				return
			}
			e = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Uid: 1000, Gid: 100, Size: i.Size(), Typeflag: tar.TypeReg})
			if e == nil {
				_, e = io.Copy(tw, f)
			}
			f.Close()
			if e != nil {
				copyErr = e
				return
			}
		}
	}()
	req, _ := http.NewRequestWithContext(ctx, "PUT", dockerutil.Endpoint("/containers/"+id+"/archive?path=/workspace"), reader)
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := d.client.StreamingHTTPClient().Do(req)
	reader.Close()
	copyErr := <-done
	if err != nil {
		return errors.New("project transfer to kernel failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("kernel rejected project files")
	}
	return copyErr
}

type bridgeStream struct {
	conn      net.Conn
	reader    *bufio.Reader
	remaining uint32
	channel   byte
	stop      func() bool
}

func (s *bridgeStream) Write(p []byte) (int, error) { return s.conn.Write(p) }
func (s *bridgeStream) Close() error {
	if s.stop != nil {
		s.stop()
	}
	return s.conn.Close()
}
func (s *bridgeStream) Read(p []byte) (int, error) {
	for {
		if s.remaining == 0 {
			var header [8]byte
			if _, err := io.ReadFull(s.reader, header[:]); err != nil {
				return 0, err
			}
			s.channel = header[0]
			s.remaining = binary.BigEndian.Uint32(header[4:])
			if s.remaining > 2<<20 {
				return 0, errors.New("oversized Docker stream frame")
			}
		}
		if s.remaining == 0 {
			continue
		}
		n := len(p)
		if uint32(n) > s.remaining {
			n = int(s.remaining)
		}
		count, err := s.reader.Read(p[:n])
		s.remaining -= uint32(count)
		if s.channel == 1 {
			return count, err
		}
		if err != nil {
			return 0, err
		}
	}
}
func (d *Docker) Attach(ctx context.Context, id string) (io.ReadWriteCloser, error) {
	var exec struct {
		ID string `json:"Id"`
	}
	status, err := d.client.DoJSON(ctx, "POST", "/containers/"+id+"/exec", map[string]any{"AttachStdin": true, "AttachStdout": true, "AttachStderr": true, "Tty": false, "User": "1000:100", "Cmd": []string{"python", "-u", "/opt/aurago/bridge.py"}}, &exec)
	if err != nil || status != 201 {
		return nil, errors.New("notebook bridge creation failed")
	}
	version, err := d.client.APIVersion(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := d.client.Dial(ctx)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	success := false
	defer func() {
		if !success {
			stop()
			conn.Close()
		}
	}()
	body, _ := json.Marshal(map[string]bool{"Detach": false, "Tty": false})
	req, _ := http.NewRequest("POST", "http://docker/v"+strings.TrimPrefix(version, "v")+"/exec/"+exec.ID+"/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err = req.Write(conn); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 101 && response.StatusCode != 200 {
		return nil, errors.New("notebook bridge attach failed")
	}
	conn.SetDeadline(time.Time{})
	success = true
	return &bridgeStream{conn: conn, reader: reader, stop: stop}, nil
}
