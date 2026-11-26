package kaggle

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestKaggleCleanOutputDropsKernelOutputURLs(t *testing.T) {
	in := `{"files":[{"fileName":"a.gguf","url":"https://storage.example/a.gguf?X-Goog-Signature=abc"}],"log":""}`
	got := string(cleanOutput(json.RawMessage(in)))
	want := `{"files":[{"fileName":"a.gguf"}],"log":""}`
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	for _, bad := range []string{"url", "https://", "storage.example", "X-Goog-Signature"} {
		if strings.Contains(got, bad) {
			t.Fatalf("result contains %q: %s", bad, got)
		}
	}
}

func TestKaggleCleanOutputURLRules(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"file entry token in path", `{"files":[{"fileName":"m.gguf","url":"https://www.kaggleusercontent.com/kf/1/eyJhbGciOiJkaXIifQ..x/m.gguf"}]}`, `{"files":[{"fileName":"m.gguf"}]}`},
		{"file entry unknown host", `{"files":[{"fileName":"b.bin","url":"https://files.example/opaque/b.bin"}]}`, `{"files":[{"fileName":"b.bin"}]}`},
		{"dataset file list", `{"datasetFiles":[{"name":"train.csv","totalBytes":12,"url":"https://files.example/t/train.csv"}],"nextPageToken":"next"}`, `{"datasetFiles":[{"name":"train.csv","totalBytes":12}],"nextPageToken":"next"}`},
		{"fileName outside list", `{"fileName":"sub.csv","ref":7,"url":"https://www.kaggle.com/submissions/7/7.raw"}`, `{"fileName":"sub.csv","ref":7}`},
		{"kaggle page link stays", `{"ref":"/code/owner/book","url":"https://www.kaggle.com/code/owner/book","versionNumber":3}`, `{"ref":"/code/owner/book","url":"https://www.kaggle.com/code/owner/book","versionNumber":3}`},
		{"third-party link stays", `{"externalBaseModelUrl":"https://huggingface.co/org/model"}`, `{"externalBaseModelUrl":"https://huggingface.co/org/model"}`},
		{"signed url any key", `{"model":{"archive":"https://storage.googleapis.com/b/o?X-Goog-Algorithm=GOOG4&X-Goog-Signature=abc","title":"m"}}`, `{"model":{"title":"m"}}`},
		{"kaggleusercontent host", `{"outputLink":"https://www.kaggleusercontent.com/kf/1/tok/x.csv","status":"ok"}`, `{"status":"ok"}`},
		{"user info", `{"source":"https://user:pass@host.example/x","title":"t"}`, `{"title":"t"}`},
		{"credential url in text", `{"files":[],"log":"saved to https://storage.example/a.gguf?X-Goog-Signature=abc done"}`, `{"files":[],"log":"saved to [download URL removed] done"}`},
		{"trailing punctuation", `{"log":"see https://storage.example/a?Signature=abc&GoogleAccessId=x."}`, `{"log":"see [download URL removed]."}`},
		{"list of strings", `{"links":["https://storage.example/a?Signature=abc&GoogleAccessId=x","https://www.kaggle.com/datasets/o/d"]}`, `{"links":["[download URL removed]","https://www.kaggle.com/datasets/o/d"]}`},
		{"harmless link in text", `{"description":"docs at https://www.kaggle.com/docs/api?tab=1 and more"}`, `{"description":"docs at https://www.kaggle.com/docs/api?tab=1 and more"}`},
		{"int64 identity", `{"files":[{"fileName":"a","url":"https://x.example/a"}],"id":9007199254740993}`, `{"files":[{"fileName":"a"}],"id":9007199254740993}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(cleanOutput(json.RawMessage(tc.in)))
			if got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestKaggleExecuteListKernelSessionOutputHidesURLs(t *testing.T) {
	s := newTestService(t)
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":[{"fileName":"a.gguf","url":"https://storage.example/a.gguf?X-Goog-Signature=abc"},{"fileName":"b.gguf","url":"https://storage.example/b.gguf?X-Goog-Signature=def"}],"log":"training done"}`))
	})
	s.clientFactory = func(Credentials, time.Duration) (*Client, error) { return c, nil }
	res, err := s.Execute(context.Background(), Request{Domain: "notebooks", Operation: "list_kernel_session_output", Parameters: []byte(`{"userName":"owner","kernelSlug":"book"}`)}, s.current().Access)
	if err != nil {
		t.Fatal(err)
	}
	if res.Job != nil {
		t.Fatalf("read returned a job: %+v", res.Job)
	}
	var out struct {
		Files []map[string]any `json:"files"`
		Log   string           `json:"log"`
	}
	if err := json.Unmarshal(res.Data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 2 {
		t.Fatalf("files = %v", out.Files)
	}
	for i, f := range out.Files {
		if name, _ := f["fileName"].(string); name == "" {
			t.Fatalf("file %d lost its name: %v", i, f)
		}
		if _, has := f["url"]; has {
			t.Fatalf("file %d kept url: %v", i, f)
		}
	}
	if out.Log != "training done" {
		t.Fatalf("log = %q", out.Log)
	}
	data := string(res.Data)
	for _, bad := range []string{"X-Goog-Signature", "storage.example"} {
		if strings.Contains(data, bad) {
			t.Fatalf("data contains %q: %s", bad, data)
		}
	}
}
