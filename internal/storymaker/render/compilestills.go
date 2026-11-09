package render

import (
	"context"
	"math"
	"time"

	"aurago/internal/storymaker"
)

// stillsReadyLimit bounds the wait for the page to become ready for screenshots after the report.
const stillsReadyLimit = 60 * time.Second

// stillFrames picks, for each measured scene, the frame to capture: the hold frame of its layout block, inside the scene and
// inside the frames the page offers (frameLimit is the export limit, 0 for none). A scene that is not in the summary, or whose
// hold frame lies outside, gets no still.
func stillFrames(summary storymaker.CompileSummary, fps, frameLimit int) (frames []int, ids []string, indexes []int) {
	byID := map[string]storymaker.CompileScene{}
	for _, sc := range summary.Scenes {
		byID[sc.ID] = sc
	}
	limit := int(math.Round(float64(summary.DurationMS) * float64(fps) / 1000))
	if frameLimit > 0 {
		limit = min(limit, frameLimit)
	}
	for _, block := range summary.Layout {
		sc, ok := byID[block.ID]
		if !ok {
			continue
		}
		start := int(math.Round(float64(sc.StartMS) * float64(fps) / 1000))
		end := int(math.Round(float64(sc.EndMS) * float64(fps) / 1000))
		n := min(start+int(math.Round(block.HoldS*float64(fps))), end-1)
		if n < start || n >= limit {
			continue
		}
		frames, ids, indexes = append(frames, n), append(ids, block.ID), append(indexes, block.Index)
	}
	return frames, ids, indexes
}

// captureStills takes one still per measured scene from the open page and adds the hashes and the contact sheet to the
// summary. A failure leaves the summary as it was: the stills are an extra, never a reason to fail the check.
func (r *Runner) captureStills(ctx context.Context, page browserPage, summary *storymaker.CompileSummary, fps, frameLimit int) {
	frames, ids, indexes := stillFrames(*summary, fps, frameLimit)
	if len(frames) == 0 {
		return
	}
	readyCtx, cancel := context.WithTimeout(ctx, stillsReadyLimit)
	defer cancel()
	if err := page.WaitReady(readyCtx); err != nil {
		r.deps.Logger.Warn("[Storymaker] The page was not ready for stills", "error", err)
		return
	}
	pngs := make([][]byte, 0, len(frames))
	for _, n := range frames {
		data, err := page.Capture(ctx, n)
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] A still could not be captured", "frame", n, "error", err)
			return
		}
		pngs = append(pngs, data)
	}
	sheet, stills, err := storymaker.BuildStills(ids, indexes, pngs)
	if err != nil {
		r.deps.Logger.Warn("[Storymaker] The stills could not be assembled", "error", err)
		return
	}
	summary.Sheet, summary.Stills = sheet, stills
}
