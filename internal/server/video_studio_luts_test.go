package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/desktop"
	"aurago/internal/uid"
	"aurago/internal/videostudio"
)

// videoStudioTestCube is an identity .cube table with size³ entries, red fastest, and a title.
func videoStudioTestCube(size int) []byte {
	var text bytes.Buffer
	fmt.Fprintf(&text, "TITLE \"test\"\nLUT_3D_SIZE %d\n", size)
	for b := 0; b < size; b++ {
		for g := 0; g < size; g++ {
			for r := 0; r < size; r++ {
				fmt.Fprintf(&text, "%.6f %.6f %.6f\n", float64(r)/float64(size-1), float64(g)/float64(size-1), float64(b)/float64(size-1))
			}
		}
	}
	return text.Bytes()
}

// videoStudioUploadLUT posts one file as a multipart import and returns the raw answer.
func videoStudioUploadLUT(s *Server, token, projectID, name, key string, data []byte) *httptest.ResponseRecorder {
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, _ := writer.CreateFormFile("file", name)
	_, _ = part.Write(data)
	_ = writer.Close()
	return videoStudioAPIRequest(s, token, http.MethodPost, "/projects/"+projectID+"/media", &form, map[string]string{"Content-Type": writer.FormDataContentType(), "Idempotency-Key": key})
}

func videoStudioExpectError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status || !strings.Contains(response.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("answered %d %s, want %d %s", response.Code, response.Body.String(), status, code)
	}
}

// A .cube import becomes a lut asset {kind, lut_size, size_bytes} (index §4.20). Its probe job parses the stored copy
// without FFmpeg (the configured binary is missing here). It gets no poster and no proxy.
func TestVideoStudioLUTUploadRecordsAsset(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = "video-studio-test-missing-ffmpeg"
	project, _ := videoStudioAPICreate(t, s, writeToken)
	cube := videoStudioTestCube(17)
	accepted := videoStudioUploadLUT(s, writeToken, project.ID, "Teal Look.CUBE", "lut-upload", cube)
	var body struct {
		Job videoStudioJob `json:"job"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &body); err != nil || accepted.Code != http.StatusAccepted || body.Job.ID == "" {
		t.Fatalf("upload: %d %s (%v)", accepted.Code, accepted.Body.String(), err)
	}
	job := videoStudioDeriveWait(t, s, readToken, body.Job.ID)
	asset := job.Result.Asset
	if asset == nil || asset.Kind != videostudio.AssetLUT || asset.LUTSize != 17 || asset.SizeBytes != int64(len(cube)) ||
		asset.Width != 0 || asset.Height != 0 || asset.DurationFrames != 0 || asset.HasPoster || asset.Name != "Teal_Look.CUBE" {
		t.Fatalf("probed asset = %+v", asset)
	}
	media := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID+"/media", nil, nil)
	if text := media.Body.String(); media.Code != http.StatusOK || !strings.Contains(text, `"kind":"lut"`) || !strings.Contains(text, `"lut_size":17`) || !strings.Contains(text, `"state":"ready"`) {
		t.Fatalf("media list: %d %s", media.Code, text)
	}
	read := videoStudioAPIRequest(s, readToken, http.MethodGet, "/projects/"+project.ID, nil, nil)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"lut_size":17`) {
		t.Fatalf("the project does not list the LUT: %d %s", read.Code, read.Body.String())
	}
	preview := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/jobs", strings.NewReader(`{"kind":"preview","asset_id":"`+asset.ID+`"}`), map[string]string{"Idempotency-Key": "lut-preview"})
	videoStudioExpectError(t, preview, http.StatusBadRequest, "preview_requires_video")
}

// Each refused .cube answers 400 invalid_lut with the parser's line, and nothing reaches the project: no record and no
// file.
func TestVideoStudioLUTUploadRejectsInvalid(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	for i, c := range []struct {
		name string
		data []byte
		line int
	}{
		{"a NaN value", []byte("LUT_3D_SIZE 2\n0 0 0\n0 0 NaN\n"), 3},
		{"a 1D table", []byte("TITLE \"x\"\nLUT_1D_SIZE 4\n"), 2},
		{"binary data", []byte{0xff, 0xfe, 0x00, 0x41}, 1},
		{"too few data lines", []byte("LUT_3D_SIZE 2\n0 0 0\n"), 3},
	} {
		response := videoStudioUploadLUT(s, writeToken, project.ID, "bad.cube", fmt.Sprintf("bad-%d", i), c.data)
		var refused struct {
			Code string `json:"code"`
			Line int    `json:"line"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &refused); err != nil || response.Code != http.StatusBadRequest || refused.Code != "invalid_lut" || refused.Line != c.line {
			t.Fatalf("%s: %d %s, want 400 invalid_lut on line %d", c.name, response.Code, response.Body.String(), c.line)
		}
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	if records := manager.mediaForProject(project.ID); len(records) != 0 {
		t.Fatalf("refused imports left records: %+v", records)
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if entries, err := svc.ListFiles(context.Background(), videoStudioProjectPath(project.ID)+"/media"); err == nil && len(entries) != 0 {
		t.Fatalf("refused imports left files: %+v", entries)
	}
}

// The size limit holds for both import paths:
//   - a 16 MiB + 1 byte multipart part is refused after reading at most that much;
//   - a Desktop source_path is refused from its stat size, before reading.
//
// The count limit holds under the project lock and includes .cube imports whose probe is still pending.
func TestVideoStudioLUTUploadLimits(t *testing.T) {
	s, readToken, writeToken := videoStudioAPIFixture(t)
	project, _ := videoStudioAPICreate(t, s, writeToken)
	ctx := context.Background()
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte{'#'}, videostudio.MaxLUTBytes+1)
	t.Run("multipart above 16 MiB", func(t *testing.T) {
		videoStudioExpectError(t, videoStudioUploadLUT(s, writeToken, project.ID, "big.cube", "lut-big", big), http.StatusRequestEntityTooLarge, "lut_size_limit")
	})
	t.Run("Desktop source_path above 16 MiB", func(t *testing.T) {
		if err := svc.WriteFileBytes(ctx, "Documents/big.cube", big, desktop.SourceUser); err != nil {
			t.Fatal(err)
		}
		response := videoStudioAPIRequest(s, writeToken, http.MethodPost, "/projects/"+project.ID+"/media", strings.NewReader(`{"source_path":"Documents/big.cube"}`), map[string]string{"Idempotency-Key": "lut-big-desktop"})
		videoStudioExpectError(t, response, http.StatusRequestEntityTooLarge, "lut_size_limit")
	})
	if records := manager.mediaForProject(project.ID); len(records) != 0 {
		t.Fatalf("refused imports left records: %+v", records)
	}
	t.Run("the 33rd LUT", func(t *testing.T) {
		cube := videoStudioTestCube(2)
		for i := 0; i < videostudio.MaxLUTAssetsPerProject-1; i++ {
			videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: uid.New(), Name: fmt.Sprintf("look%02d.cube", i), Kind: videostudio.AssetLUT, LUTSize: 2}, cube)
		}
		pending := videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: uid.New(), Name: "pending.cube"}, cube)
		videoStudioExpectError(t, videoStudioUploadLUT(s, writeToken, project.ID, "one-more.cube", "lut-33a", cube), http.StatusConflict, "lut_count_limit")
		if err := manager.removeMedia(project.ID, pending.Asset.ID); err != nil {
			t.Fatal(err)
		}
		accepted := videoStudioUploadLUT(s, writeToken, project.ID, "last.cube", "lut-32", cube)
		var body struct {
			Job videoStudioJob `json:"job"`
		}
		if err := json.Unmarshal(accepted.Body.Bytes(), &body); err != nil || accepted.Code != http.StatusAccepted {
			t.Fatalf("the 32nd LUT: %d %s (%v)", accepted.Code, accepted.Body.String(), err)
		}
		videoStudioDeriveWait(t, s, readToken, body.Job.ID)
		videoStudioExpectError(t, videoStudioUploadLUT(s, writeToken, project.ID, "too-many.cube", "lut-33b", cube), http.StatusConflict, "lut_count_limit")
	})
}

// GET …/luts/builtin/{name}.cube serves the Go-generated bytes of registry looks only, with the access every other
// Video Studio read needs and the headers of index §4.20.
func TestVideoStudioBuiltinLUTEndpoint(t *testing.T) {
	s, readToken, _ := videoStudioAPIFixture(t)
	looks := videostudio.EffectRegistry().Looks
	if len(looks) != 8 {
		t.Fatalf("%d registry looks, want 8", len(looks))
	}
	for _, look := range looks {
		response := videoStudioAPIRequest(s, readToken, http.MethodGet, "/luts/builtin/"+look.ID+".cube", nil, nil)
		want, ok := videostudio.BuiltinLookCube(look.ID)
		if response.Code != http.StatusOK || !ok || !bytes.Equal(response.Body.Bytes(), want) {
			t.Fatalf("look %s: %d, %d bytes (want %d)", look.ID, response.Code, response.Body.Len(), len(want))
		}
		header := response.Header()
		if header.Get("Content-Type") != "text/plain; charset=utf-8" || header.Get("Cache-Control") != "private, max-age=86400" || header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("look %s headers: %v", look.ID, header)
		}
	}
	if head := videoStudioAPIRequest(s, readToken, http.MethodHead, "/luts/builtin/warm.cube", nil, nil); head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD: %d with %d bytes", head.Code, head.Body.Len())
	}
	for _, name := range []string{"sepia.cube", "warm", "WARM.cube", "warm.cube.cube", "builtin:warm.cube", ".cube"} {
		videoStudioExpectError(t, videoStudioAPIRequest(s, readToken, http.MethodGet, "/luts/builtin/"+name, nil, nil), http.StatusNotFound, "lut_not_found")
	}
	for _, path := range []string{"/luts/builtin/..%2Fwarm.cube", "/luts/builtin/warm.cube/extra", "/luts/builtin", "/luts/warm.cube"} {
		if response := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "LUT_3D_SIZE") {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
	}
	if post := videoStudioAPIRequest(s, readToken, http.MethodPost, "/luts/builtin/warm.cube", nil, nil); post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", post.Code)
	}
	anonymous := httptest.NewRecorder()
	handleVideoStudio(s).ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, videoStudioAPIBase+"/luts/builtin/warm.cube", nil))
	if anonymous.Code != http.StatusUnauthorized && anonymous.Code != http.StatusForbidden {
		t.Fatalf("without a token: %d %s", anonymous.Code, anonymous.Body.String())
	}
	s.Cfg.VideoStudio.Enabled = false
	videoStudioExpectError(t, videoStudioAPIRequest(s, readToken, http.MethodGet, "/luts/builtin/warm.cube", nil, nil), http.StatusServiceUnavailable, "video_studio_disabled")
}

// A LUT asset stays while an effect of the saved project references it, even an effect that is switched off.
func TestVideoStudioLUTMediaInUse(t *testing.T) {
	s, _, writeToken := videoStudioAPIFixture(t)
	project, etag := videoStudioAPICreate(t, s, writeToken)
	const imageID, lutID = "123e4567-e89b-12d3-a456-426614176301", "123e4567-e89b-12d3-a456-426614176302"
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: imageID, Name: "still.png", Kind: videostudio.AssetImage, Width: 2, Height: 2}, []byte("still"))
	videoStudioSeedMedia(t, s, project.ID, videostudio.Asset{ID: lutID, Name: "look.cube", Kind: videostudio.AssetLUT, LUTSize: 2}, videoStudioTestCube(2))
	project.Project.Assets = []videostudio.Asset{{ID: imageID, Name: "still.png"}, {ID: lutID, Name: "look.cube"}}
	project.Project.Tracks = []videostudio.Track{{ID: "v1", Kind: videostudio.TrackVideo, Name: "Video 1", Clips: []videostudio.Clip{{
		ID: "c1", AssetID: imageID, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: videostudio.FitContain,
		Effects: []videostudio.EffectInstance{{ID: "lut", On: false, Params: map[string]any{"intensity": 1.0, "lut": lutID}}},
	}}}}
	save := func(etag string) string {
		t.Helper()
		body, err := json.Marshal(project.Project)
		if err != nil {
			t.Fatal(err)
		}
		saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, "/projects/"+project.ID, bytes.NewReader(body), map[string]string{"If-Match": etag})
		if saved.Code != http.StatusOK {
			t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
		}
		return saved.Header().Get("ETag")
	}
	etag = save(etag)
	path := "/projects/" + project.ID + "/media/" + lutID
	videoStudioExpectError(t, videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil), http.StatusConflict, "media_in_use")
	// The reference counts on its own, also when the asset list no longer names the LUT.
	if !videoStudioProjectUsesAsset(videostudio.Project{Tracks: project.Project.Tracks}, lutID) {
		t.Fatal("an effect reference does not count as use")
	}
	project.Project.Assets = project.Project.Assets[:1]
	project.Project.Tracks[0].Clips[0].Effects = nil
	save(etag)
	if removed := videoStudioAPIRequest(s, writeToken, http.MethodDelete, path, nil, nil); removed.Code != http.StatusOK {
		t.Fatalf("removal after the effect is gone: %d %s", removed.Code, removed.Body.String())
	}
}

// A render stages the LUT assets of on lut entries, but only on enabled clips of visible visual tracks. It is the same
// rule that collectRenderSources (videostudio) uses to load them.
func TestVideoStudioRenderAssetIDsIncludeLUTs(t *testing.T) {
	lut := func(on bool, ref string) videostudio.EffectInstance {
		return videostudio.EffectInstance{ID: "lut", On: on, Params: map[string]any{"intensity": 1.0, "lut": ref}}
	}
	project := videostudio.Project{Tracks: []videostudio.Track{
		{ID: "v", Kind: videostudio.TrackVideo, Clips: []videostudio.Clip{
			{ID: "a", AssetID: "img", Effects: []videostudio.EffectInstance{lut(true, "used"), lut(false, "off"), lut(true, "builtin:warm"), lut(true, "")}},
			{ID: "b", AssetID: "img2", Disabled: true, Effects: []videostudio.EffectInstance{lut(true, "disabled")}},
		}},
		{ID: "o", Kind: videostudio.TrackOverlay, Hidden: true, Clips: []videostudio.Clip{{ID: "c", AssetID: "logo", Effects: []videostudio.EffectInstance{lut(true, "hidden")}}}},
		{ID: "x", Kind: videostudio.TrackOverlay, Clips: []videostudio.Clip{{ID: "d", AssetID: "logo", Effects: []videostudio.EffectInstance{{ID: "color", On: true, Params: map[string]any{"lut": "not-a-lut-effect"}}}}}},
	}}
	active := videoStudioRenderAssetIDs(project)
	for _, id := range []string{"img", "used", "logo"} {
		if _, ok := active[id]; !ok {
			t.Errorf("%s is not staged", id)
		}
	}
	for _, id := range []string{"off", "disabled", "hidden", "img2", "warm", "builtin:warm", "not-a-lut-effect", ""} {
		if _, ok := active[id]; ok {
			t.Errorf("%s is staged", id)
		}
	}
	all, rendered := videoStudioEffectLUTRefs(project)
	if len(all) != 4 || len(rendered) != 1 {
		t.Fatalf("references %v, rendered %v", all, rendered)
	}
	for _, id := range []string{"used", "off", "disabled", "hidden"} {
		if _, ok := all[id]; !ok {
			t.Errorf("reference %s is missing", id)
		}
	}
}

// A long .cube name is shortened in its stem, so it keeps its extension and the import still treats it as a LUT
// (index §4.20).
func TestVideoStudioLUTFilenameKeepsExtension(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{"Teal Look.cube", "Teal_Look.cube"},
		{strings.Repeat("a", 200) + ".cube", strings.Repeat("a", 115) + ".cube"},
		{strings.Repeat("b", 130) + ".CUBE", strings.Repeat("b", 115) + ".CUBE"},
		{strings.Repeat("c", 130), strings.Repeat("c", 120)},
	} {
		got := safeVideoStudioFilename(c.raw)
		if got != c.want || videostudio.IsLUTFileName(got) != videostudio.IsLUTFileName(c.raw) {
			t.Errorf("safeVideoStudioFilename(%d bytes) = %q (%d bytes), want %q", len(c.raw), got, len(got), c.want)
		}
	}
}

// videoStudioEndlessReader yields '#' bytes forever and counts what was taken.
type videoStudioEndlessReader struct{ taken int64 }

func (r *videoStudioEndlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '#'
	}
	r.taken += int64(len(p))
	return len(p), nil
}

// readVideoStudioLUTUpload takes at most MaxLUTBytes+1 bytes from an endless source, and reads nothing at all when
// the stat size of a Desktop source_path is already above the limit.
func TestVideoStudioLUTReadIsBounded(t *testing.T) {
	endless := &videoStudioEndlessReader{}
	response := httptest.NewRecorder()
	if _, ok := readVideoStudioLUTUpload(response, endless, -1); ok {
		t.Fatal("an endless source was accepted")
	}
	videoStudioExpectError(t, response, http.StatusRequestEntityTooLarge, "lut_size_limit")
	if endless.taken < videostudio.MaxLUTBytes+1 || endless.taken > videostudio.MaxLUTBytes+1+(1<<20) {
		t.Fatalf("took %d bytes, want MaxLUTBytes+1 (plus the reader's chunk)", endless.taken)
	}
	untouched := &videoStudioEndlessReader{}
	response = httptest.NewRecorder()
	if _, ok := readVideoStudioLUTUpload(response, untouched, videostudio.MaxLUTBytes+1); ok || untouched.taken != 0 {
		t.Fatalf("a source_path above the limit was read: ok=%v, %d bytes", ok, untouched.taken)
	}
	videoStudioExpectError(t, response, http.StatusRequestEntityTooLarge, "lut_size_limit")
}
