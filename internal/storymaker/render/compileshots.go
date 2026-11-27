package render

import (
	"context"
	"math"

	"aurago/internal/storymaker"
)

// shotDefaultFraction is the point of a scene without a measured hold frame, as a share of its length.
const shotDefaultFraction = 0.7

// shotFrames picks the frame of each requested scene that the summary knows: the frame at AtS when the request has one, else the
// hold frame of the scene's layout block, else 70 percent into the scene. The frame stays inside the scene and inside the
// frames the page offers (frameLimit is the export limit, 0 for none); a request that cannot be served is skipped. atMS is the
// scene-local time of the frame in milliseconds.
func shotFrames(summary storymaker.CompileSummary, reqs []storymaker.ShotRequest, fps, frameLimit int) (frames []int, ids []string, atMS []int64) {
	byID := map[string]storymaker.CompileScene{}
	for _, sc := range summary.Scenes {
		byID[sc.ID] = sc
	}
	hold := map[string]float64{}
	for _, block := range summary.Layout {
		hold[block.ID] = block.HoldS
	}
	limit := int(math.Round(float64(summary.DurationMS) * float64(fps) / 1000))
	if frameLimit > 0 {
		limit = min(limit, frameLimit)
	}
	for _, req := range reqs {
		sc, ok := byID[req.SceneID]
		if !ok {
			continue
		}
		start := int(math.Round(float64(sc.StartMS) * float64(fps) / 1000))
		end := int(math.Round(float64(sc.EndMS) * float64(fps) / 1000))
		var n int
		switch held, measured := hold[req.SceneID]; {
		case req.HasAt:
			n = start + int(math.Round(req.AtS*float64(fps)))
		case measured:
			n = start + int(math.Round(held*float64(fps)))
		default:
			n = start + int(math.Round(shotDefaultFraction*float64(end-start)))
		}
		n = min(max(n, start), end-1)
		if n < start || n >= limit {
			continue
		}
		local := int64(math.Round(float64(n-start) * 1000 / float64(fps)))
		frames, ids, atMS = append(frames, n), append(ids, req.SceneID), append(atMS, local)
	}
	return frames, ids, atMS
}

// captureShots takes the pictures a model asked for from the open page and adds them to the summary: one JPEG per scene, or with
// sheet one JPEG contact sheet and the metadata of its pictures. A failure leaves the summary without pictures: they are an
// extra, never a reason to fail the check.
func (r *Runner) captureShots(ctx context.Context, page browserPage, summary *storymaker.CompileSummary, reqs []storymaker.ShotRequest, sheet bool, fps, frameLimit int, prefix, token string) {
	frames, ids, atMS := shotFrames(*summary, reqs, fps, frameLimit)
	if len(frames) == 0 {
		return
	}
	readyCtx, cancel := context.WithTimeout(ctx, stillsReadyLimit)
	defer cancel()
	if err := page.WaitReady(readyCtx); err != nil {
		r.deps.Logger.Warn("[Storymaker] The page was not ready for screenshots", "error", withoutSecrets(err.Error(), prefix, token))
		return
	}
	var (
		pngs  [][]byte
		kept  []int
		shots []storymaker.SceneShot
	)
	for i, n := range frames {
		data, err := page.Capture(ctx, n)
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] A screenshot could not be captured", "frame", n, "error", withoutSecrets(err.Error(), prefix, token))
			return
		}
		if len(data) > storymaker.MaxStillBytes {
			r.deps.Logger.Warn("[Storymaker] A screenshot is over the size limit", "frame", n, "bytes", len(data), "limit", storymaker.MaxStillBytes)
			continue
		}
		pngs, kept = append(pngs, data), append(kept, i)
	}
	if len(pngs) == 0 {
		return
	}
	if sheet {
		combined, err := storymaker.ContactSheet(pngs)
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] The screenshot sheet could not be made", "error", withoutSecrets(err.Error(), prefix, token))
			return
		}
		image, w, h, err := storymaker.ShotImage(combined)
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] The screenshot sheet could not be encoded", "error", withoutSecrets(err.Error(), prefix, token))
			return
		}
		for _, i := range kept {
			shots = append(shots, storymaker.SceneShot{SceneID: ids[i], AtMS: atMS[i]})
		}
		summary.Shots, summary.ShotSheet, summary.ShotSheetWidth, summary.ShotSheetHeight = shots, image, w, h
		return
	}
	for k, data := range pngs {
		i := kept[k]
		image, w, h, err := storymaker.ShotImage(data)
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] A screenshot could not be encoded", "scene", ids[i], "error", withoutSecrets(err.Error(), prefix, token))
			continue
		}
		shots = append(shots, storymaker.SceneShot{SceneID: ids[i], AtMS: atMS[i], Width: w, Height: h, Image: image})
	}
	summary.Shots = shots
}
