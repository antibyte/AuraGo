package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/storymaker"
)

// compileReport is the part of the page's compile report Go uses (spec "Compile report and job
// order"); unknown fields are ignored.
type compileReport struct {
	FrameCount     int                `json:"frame_count"`
	FPS            int                `json:"fps"`
	Width          int                `json:"width"`
	Height         int                `json:"height"`
	DurationMS     float64            `json:"duration_ms"`
	CatalogVersion string             `json:"catalog_version"`
	Scenes         []reportScene      `json:"scenes"`
	Issues         []storymaker.Issue `json:"issues"`
}

type reportScene struct {
	ID         string `json:"id"`
	StartFrame int    `json:"start_frame"`
	EndFrame   int    `json:"end_frame"`
	Kind       string `json:"kind"`
}

// reportRefusal is a report Go will not render: the code is a job error code, the message is shown
// to the user and to the model.
type reportRefusal struct {
	Code    string
	Message string
}

func (r *reportRefusal) Error() string { return r.Code + ": " + r.Message }

// reportExpect is what Go knows independently of the page.
type reportExpect struct {
	FPS            int
	Width, Height  int
	MaxFrames      int
	CatalogVersion string
	SceneIDs       []string
}

// parseReport decodes and validates a compile report: the numbers must agree with the job (fps,
// canvas, catalog version), frame_count must not exceed the Go-computed limit, the scene ranges must
// be monotonic and cover frame_count exactly, and an error issue refuses the render.
func parseReport(body []byte, want reportExpect) (compileReport, *reportRefusal) {
	var rep compileReport
	if err := json.Unmarshal(body, &rep); err != nil {
		return rep, &reportRefusal{CodeRenderFailed, "The compile report is not valid JSON."}
	}
	bad := func(format string, args ...any) (compileReport, *reportRefusal) {
		return rep, &reportRefusal{CodeRenderFailed, "The compile report was refused: " + fmt.Sprintf(format, args...)}
	}
	if rep.FPS != want.FPS || rep.Width != want.Width || rep.Height != want.Height {
		return bad("it describes %dx%d at %d fps, the job is %dx%d at %d fps", rep.Width, rep.Height, rep.FPS, want.Width, want.Height, want.FPS)
	}
	if rep.CatalogVersion != want.CatalogVersion {
		return bad("the catalog version %q differs from %q", rep.CatalogVersion, want.CatalogVersion)
	}
	for _, is := range rep.Issues {
		if is.Severity == storymaker.SeverityError {
			code := CodeInvalidStory
			if is.Code == storymaker.CodeCodeError || is.Code == storymaker.CodeDeadline {
				code = CodeCodeError
			}
			msg := is.Message
			if is.SceneID != "" {
				msg = "Scene " + is.SceneID + ": " + msg
			}
			return rep, &reportRefusal{code, clip(strings.TrimSpace(msg), 400)}
		}
	}
	if rep.FrameCount < 1 {
		return bad("the story has no frames")
	}
	if rep.FrameCount > want.MaxFrames {
		return rep, &reportRefusal{CodeInvalidStory, fmt.Sprintf("The video would have %d frames; the limit is %d (render.max_duration_seconds).", rep.FrameCount, want.MaxFrames)}
	}
	if len(rep.Scenes) != len(want.SceneIDs) {
		return bad("it lists %d scenes, the story has %d", len(rep.Scenes), len(want.SceneIDs))
	}
	maxEnd := 0
	for i, sc := range rep.Scenes {
		if sc.ID != want.SceneIDs[i] {
			return bad("scene %d is %q, the story has %q", i, sc.ID, want.SceneIDs[i])
		}
		if sc.StartFrame < 0 || sc.EndFrame <= sc.StartFrame || sc.EndFrame > rep.FrameCount {
			return bad("scene %q has the invalid range %d to %d", sc.ID, sc.StartFrame, sc.EndFrame)
		}
		if i > 0 {
			prev := rep.Scenes[i-1]
			// Transitions overlap scenes, so a start may lie before the previous end, but the ranges
			// still move forward.
			if sc.StartFrame <= prev.StartFrame || sc.EndFrame < prev.EndFrame {
				return bad("the scene ranges are not monotonic at %q", sc.ID)
			}
		}
		if sc.EndFrame > maxEnd {
			maxEnd = sc.EndFrame
		}
	}
	if maxEnd != rep.FrameCount {
		return bad("frame_count %d is not the last scene end %d", rep.FrameCount, maxEnd)
	}
	return rep, nil
}
