package notebooks

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/kaggle"
)

const preservedNotebook = `{"nbformat":4,"nbformat_minor":5,"metadata":{"unknown":{"nested":[1,2]}},"extra":"preserve","cells":[{"id":"first","cell_type":"code","source":["answer = 40"],"metadata":{"vendor":true},"execution_count":null,"outputs":[]},{"id":"second","cell_type":"code","source":"print(answer + 2)","metadata":{},"execution_count":null,"outputs":[]},{"id":"md","cell_type":"markdown","source":"![x](attachment:x.png)","metadata":{},"attachments":{"x.png":{"image/png":"eA=="}}}]}`

func documentFixture(t *testing.T) (*kaggle.Service, *Documents, kaggle.Project, *sync.Mutex, *config.KaggleConfig) {
	t.Helper()
	cfg := config.DefaultKaggleConfig()
	cfg.Enabled = true
	cfg.ReadOnly = false
	cfg.Local.Enabled = true
	cfg.Local.AllowExecute = true
	var mu sync.Mutex
	s, err := kaggle.NewService(context.Background(), t.TempDir(), func() kaggle.Snapshot {
		mu.Lock()
		defer mu.Unlock()
		return kaggle.Snapshot{Config: cfg, Access: kaggle.Access{Network: true, Write: true, Python: true, Docker: true, Admin: true}}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	p, err := s.CreateProject(context.Background(), "Notebook tests")
	if err != nil {
		t.Fatal(err)
	}
	return s, NewDocuments(s), p, &mu, &cfg
}
func TestNotebookRoundtripUnknownMetadataAttachmentsAndConflict(t *testing.T) {
	_, d, p, _, _ := documentFixture(t)
	doc, err := d.Save(context.Background(), p.ID, "book.ipynb", "*", []byte(preservedNotebook))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := d.Load(p.ID, "book.ipynb")
	if err != nil || string(loaded.Content) != preservedNotebook || loaded.Revision != doc.Revision {
		t.Fatalf("lossy roundtrip: %s %v", loaded.Content, err)
	}
	changed := strings.Replace(preservedNotebook, "answer = 40", "answer = 41", 1)
	newDoc, err := d.Save(context.Background(), p.ID, "book.ipynb", doc.Revision, []byte(changed))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Save(context.Background(), p.ID, "book.ipynb", doc.Revision, doc.Content); !errors.Is(err, ErrConflict) {
		t.Fatal("lost update allowed")
	}
	if _, err = d.Save(context.Background(), p.ID, "book.ipynb", "*", doc.Content); !errors.Is(err, ErrConflict) {
		t.Fatal("create replaced existing file")
	}
	source, err := CellSource(newDoc, "first")
	if err != nil || source != "answer = 41" {
		t.Fatalf("source=%q %v", source, err)
	}
}
func TestNotebookValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"nbformat":3,"cells":[]}`, `{"nbformat":4,"cells":[{"id":"x"},{"id":"x"}]}`} {
		if Validate([]byte(raw)) == nil {
			t.Fatal("accepted invalid notebook")
		}
	}
}

func TestNotebookDockerStateInterruptRestartAndRevocation(t *testing.T) {
	if os.Getenv("AURAGO_KAGGLE_DOCKER_SMOKE") != "1" {
		t.Skip("set AURAGO_KAGGLE_DOCKER_SMOKE=1 for the managed Docker smoke")
	}
	s, d, p, mu, cfg := documentFixture(t)
	doc, err := d.Save(context.Background(), p.ID, "book.ipynb", "*", []byte(preservedNotebook))
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewDocker("", s.Root)
	if image := os.Getenv("AURAGO_KAGGLE_TEST_IMAGE"); image != "" {
		runtime.image = image
	}
	k := NewKernels(context.Background(), s, d, runtime)
	defer k.Close()
	session, err := k.Start(p.ID, "book.ipynb", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	wait := func(id, state string) {
		t.Helper()
		deadline := time.Now().Add(50 * time.Second)
		for time.Now().Before(deadline) {
			for _, v := range k.List() {
				if v.ID == id {
					if v.Status == state {
						return
					}
					if v.Status == "stopped" && state != "stopped" {
						t.Fatalf("kernel stopped: %+v", v)
					}
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("kernel did not reach %s: %+v", state, k.List())
	}
	wait(session.ID, "idle")
	first, err := k.Execute(session.ID, doc.Revision, "first")
	if err != nil {
		t.Fatal(err)
	}
	wait(session.ID, "idle")
	second, err := k.Execute(session.ID, doc.Revision, "second")
	if err != nil || first == second {
		t.Fatal("execution IDs are not unique")
	}
	wait(session.ID, "idle")
	events, err := s.Store.Events(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		var output map[string]any
		json.Unmarshal(event.Data, &output)
		if output["execution_id"] == second && strings.Contains(string(event.Data), "42") {
			found = true
		}
	}
	if !found {
		t.Fatal("second cell did not retain first cell's variable")
	}
	restarted, err := k.Restart(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	wait(restarted.ID, "idle")
	exec, err := k.Execute(restarted.ID, doc.Revision, "second")
	if err != nil {
		t.Fatal(err)
	}
	wait(restarted.ID, "idle")
	events, _ = s.Store.Events(context.Background(), 0)
	found = false
	for _, e := range events {
		if strings.Contains(string(e.Data), exec) && strings.Contains(string(e.Data), "NameError") {
			found = true
		}
	}
	if !found {
		t.Fatal("restart retained variables")
	}
	loop := strings.Replace(preservedNotebook, "answer = 40", "import time; time.sleep(120)", 1)
	doc, err = d.Save(context.Background(), p.ID, "book.ipynb", doc.Revision, []byte(loop))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.Execute(restarted.ID, doc.Revision, "first"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err = k.Interrupt(restarted.ID); err != nil {
		t.Fatal(err)
	}
	wait(restarted.ID, "idle")
	mu.Lock()
	cfg.Local.AllowExecute = false
	mu.Unlock()
	wait(restarted.ID, "stopped")
	if _, err = k.Execute(restarted.ID, doc.Revision, "second"); err == nil {
		t.Fatal("revocation did not block execution")
	}
}
