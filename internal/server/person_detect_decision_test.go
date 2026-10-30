package server

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/persondetect"
)

// scriptedDecisions answers every request with one fixed response and keeps the last one.
type scriptedDecisions struct {
	*fakeDecisions
	response decisions.Response
	last     decisions.Request
}

func (d *scriptedDecisions) Evaluate(_ context.Context, r decisions.Request) (decisions.Response, error) {
	d.calls++
	d.last = r
	return d.response, d.err
}

// choiceResponse builds a normalized choice answer: top gets p, the rest share 1-p.
func choiceResponse(qid string, options []decisions.Option, top string, p float64) decisions.Response {
	probabilities := map[string]float64{}
	for _, option := range options {
		if option.ID == top {
			probabilities[option.ID] = p
		} else {
			probabilities[option.ID] = (1 - p) / float64(len(options)-1)
		}
	}
	return decisions.Response{Calibration: "uncalibrated", Answers: map[string]decisions.Answer{
		qid: {Type: "choice", Decision: top, Confidence: p, Probabilities: probabilities},
	}}
}

func testJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for x := 0; x < 64; x++ {
		for y := 0; y < 48; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 5), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func personWakeFixture(t *testing.T, response decisions.Response, supported bool) (*Server, *scriptedDecisions, persondetect.AlertEvent) {
	t.Helper()
	s, fake := hubLocal(t)
	if !supported {
		fake.status.ProjectorSHA256 = ""
	}
	scripted := &scriptedDecisions{fakeDecisions: fake, response: response}
	s.Decisions = scripted
	s.Cfg.DecisionConsumers.PersonWake = config.DecisionConsumerSwitch{Enabled: true}
	cover := filepath.Join(t.TempDir(), "cover.jpg")
	if err := os.WriteFile(cover, testJPEGBytes(t), 0o600); err != nil {
		t.Fatal(err)
	}
	return s, scripted, persondetect.AlertEvent{EventID: 1, CameraID: "door", CameraName: "Door", Zones: []string{"Porch"}, CoverPath: cover}
}

func TestPersonWakeDecisionImagesUnsupportedFiresAgent(t *testing.T) {
	s, d, ev := personWakeFixture(t, choiceResponse("scene", personWakeOptions, "no_human", 0.9), false)
	if suppress, hint := s.personWakeGate(context.Background(), ev); suppress || hint != "" || d.calls != 0 {
		t.Fatalf("suppress=%v hint=%q calls=%d", suppress, hint, d.calls)
	}
}

func TestPersonWakeDecisionOffMakesNoCall(t *testing.T) {
	s, d, ev := personWakeFixture(t, choiceResponse("scene", personWakeOptions, "no_human", 0.9), true)
	s.Cfg.DecisionConsumers.PersonWake = config.DecisionConsumerSwitch{}
	if suppress, _ := s.personWakeGate(context.Background(), ev); suppress || d.calls != 0 {
		t.Fatalf("suppress=%v calls=%d", suppress, d.calls)
	}
}

func TestPersonWakeDecisionNoHumanSuppresses(t *testing.T) {
	s, d, ev := personWakeFixture(t, choiceResponse("scene", personWakeOptions, "no_human", 0.9), true)
	if suppress, _ := s.personWakeGate(context.Background(), ev); !suppress {
		t.Fatal("no_human 0.9 did not suppress")
	}
	if d.calls != 1 || len(d.last.Images) != 1 || len(d.last.Questions) != 1 {
		t.Fatalf("calls=%d request=%+v", d.calls, d.last)
	}
}

func TestPersonWakeDecisionUnsure(t *testing.T) {
	s, _, ev := personWakeFixture(t, choiceResponse("scene", personWakeOptions, "no_human", 0.7), true)
	if suppress, _ := s.personWakeGate(context.Background(), ev); suppress {
		t.Fatal("0.7 suppressed")
	}
	s, d, ev := personWakeFixture(t, choiceResponse("scene", personWakeOptions, "courier", 0.8), true)
	if suppress, hint := s.personWakeGate(context.Background(), ev); suppress || hint != "courier" {
		t.Fatalf("suppress=%v hint=%q", suppress, hint)
	}
	d.err = context.DeadlineExceeded
	if suppress, hint := s.personWakeGate(context.Background(), ev); suppress || hint != "" {
		t.Fatalf("error: suppress=%v hint=%q", suppress, hint)
	}
}
