package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/storymaker/render"
)

const storymakerUnknownProjectID = "11111111-1111-4111-8111-111111111111"

func TestStorymakerEventsAndJobsRefuseAnUnknownProject(t *testing.T) {
	s, token, _ := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		return render.Output{}, ctx.Err()
	})
	for _, route := range []string{"/events", "/jobs"} {
		w := storymakerRequest(s, token, http.MethodGet, "/projects/"+storymakerUnknownProjectID+route, nil, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d %s", route, w.Code, w.Body.String())
		}
	}
}

func TestStorymakerEventsCapConcurrentStreams(t *testing.T) {
	old := storymakerMaxEventStreams
	storymakerMaxEventStreams = 1
	defer func() { storymakerMaxEventStreams = old }()
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		return render.Output{}, ctx.Err()
	})
	srv := httptest.NewServer(handleStorymaker(s))
	defer srv.Close()
	open := func(ctx context.Context) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+p.Project.ID+"/events", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req.WithContext(ctx))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	first := open(ctx)
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first stream = %d", first.StatusCode)
	}
	second := open(ctx)
	second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second stream = %d, want 429", second.StatusCode)
	}
	// The slot comes back when the first stream ends.
	first.Body.Close()
	deadline := time.Now().Add(10 * time.Second)
	for {
		third := open(context.Background())
		code := third.StatusCode
		third.Body.Close()
		if code == http.StatusOK {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the slot was not released, last status %d", code)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
