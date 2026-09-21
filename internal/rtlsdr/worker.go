package rtlsdr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Worker uses an owner-only Unix socket shared with the receive container.
// Native engine ports and rtl_tcp are never exposed to the host or browser.
type Worker struct{ client *http.Client }

func NewWorker(socket string) *Worker {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", socket)
	}, ResponseHeaderTimeout: 45 * time.Second, MaxIdleConnsPerHost: 8}
	return &Worker{client: &http.Client{Transport: transport}}
}
func (w *Worker) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://receiver"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := w.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: transport", ErrUnavailable)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		var detail struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&detail)
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, detail.Error)
	}
	return response, nil
}
func (w *Worker) Info(ctx context.Context) (Receiver, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	r, err := w.request(ctx, "GET", "/state", nil)
	if err != nil {
		return Receiver{}, err
	}
	defer r.Body.Close()
	var v Receiver
	err = json.NewDecoder(io.LimitReader(r.Body, 128<<10)).Decode(&v)
	return v, err
}
func (w *Worker) Tune(ctx context.Context, t Tuning) error {
	r, err := w.request(ctx, "POST", "/tune", t)
	if err == nil {
		r.Body.Close()
	}
	return err
}
func (w *Worker) Stop(ctx context.Context) error {
	r, err := w.request(ctx, "POST", "/stop", nil)
	if err == nil {
		r.Body.Close()
	}
	return err
}
func (w *Worker) Stream(ctx context.Context) (io.ReadCloser, error) {
	r, err := w.request(ctx, "GET", "/audio", nil)
	if err != nil {
		return nil, err
	}
	return r.Body, nil
}
func (w *Worker) Capture(ctx context.Context, seconds int, out io.Writer) (CaptureInfo, error) {
	r, err := w.request(ctx, "GET", "/capture?seconds="+strconv.Itoa(seconds), nil)
	if err != nil {
		return CaptureInfo{}, err
	}
	defer r.Body.Close()
	_, err = io.Copy(out, r.Body)
	duration, _ := strconv.ParseFloat(r.Trailer.Get("X-Capture-Seconds"), 64)
	gaps, _ := strconv.Atoi(r.Trailer.Get("X-Capture-Gaps"))
	if err == nil && r.Trailer.Get("X-Capture-Status") != "complete" {
		err = ErrUnavailable
	}
	return CaptureInfo{Seconds: duration, Gaps: gaps}, err
}
func (w *Worker) Scan(ctx context.Context, progress func([]Station, string)) error {
	r, err := w.request(ctx, "GET", "/scan", nil)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	scanner := bufio.NewScanner(r.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	complete := false
	for scanner.Scan() {
		var v struct {
			Stations []Station `json:"stations"`
			Block    string    `json:"block"`
			Error    string    `json:"error"`
			Complete bool      `json:"complete"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &v); err != nil {
			return err
		}
		if v.Error != "" {
			return ErrUnavailable
		}
		if v.Complete {
			complete = true
		}
		if v.Block != "" {
			progress(v.Stations, v.Block)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !complete {
		return ErrUnavailable
	}
	return nil
}
func (w *Worker) WAV(ctx context.Context, id string, offset float64, seconds int) ([]byte, error) {
	q := url.Values{"id": {id}, "offset": {strconv.FormatFloat(offset, 'f', 3, 64)}, "seconds": {strconv.Itoa(seconds)}}
	r, err := w.request(ctx, "GET", "/wav?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 4<<20))
}

func (w *Worker) AudioSeconds(ctx context.Context, id string) (float64, error) {
	r, err := w.request(ctx, "GET", "/recording-info?id="+url.QueryEscape(id), nil)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	var v struct {
		Seconds float64 `json:"seconds"`
	}
	err = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&v)
	return v.Seconds, err
}
