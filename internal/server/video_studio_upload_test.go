package server

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"
)

func TestVideoStudioUploadOutlivesOrdinaryReadTimeout(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	s, _, token := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, token)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "slow.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(part, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewUnstartedServer(handleVideoStudio(s))
	origin.Config.ReadTimeout = 100 * time.Millisecond
	origin.Config.WriteTimeout = 100 * time.Millisecond
	origin.Start()
	defer origin.Close()
	reader, pipe := io.Pipe()
	defer reader.Close()
	done := make(chan error, 1)
	go func() {
		data := body.Bytes()
		_, writeErr := pipe.Write(data[:len(data)/2])
		if writeErr == nil {
			time.Sleep(250 * time.Millisecond)
			_, writeErr = pipe.Write(data[len(data)/2:])
		}
		_ = pipe.CloseWithError(writeErr)
		done <- writeErr
	}()
	req, err := http.NewRequest(http.MethodPost, origin.URL+videoStudioAPIBase+"/projects/"+project.ID+"/media", reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Idempotency-Key", "slow-upload")
	client := origin.Client()
	client.Timeout = 5 * time.Second
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	result, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("slow upload: %d %s", response.StatusCode, result)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
