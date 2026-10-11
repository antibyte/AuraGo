package persondetect

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"sort"

	"golang.org/x/image/draw"
)

// Box is a person detection normalized to the frame (0..1).
type Box struct {
	X1    float64 `json:"x1"`
	Y1    float64 `json:"y1"`
	X2    float64 `json:"x2"`
	Y2    float64 `json:"y2"`
	Score float64 `json:"score"`
}

// Frame limits, checked on the JPEG header before anything is decoded. A pixel
// cap alone does not bound memory: x/image/draw's kernel scaler allocates
// width × source height × 32 B per Scale (a 610×65535 grey JPEG has only
// 40 Mpx but needed 1.39 GB), and the progressive decoder allocates per block
// before it reads any scan data (611×65450 needed 1.95 GB). With at most 8192 px
// per side and 16 Mpx (4K UHD is 8.3 Mpx) the worst case for the 640 px model
// input is a 2048×8192 frame: about 170 MB for the scaler plus 25 MB for the
// decoded frame (measured 200 MB allocated).
const (
	maxFrameSide   = 8192
	maxFramePixels = 1 << 24
)

func validateFrameSize(width, height int) error {
	if width <= 0 || height <= 0 || width > maxFrameSide || height > maxFrameSide ||
		int64(width)*int64(height) > maxFramePixels {
		return fmt.Errorf("JPEG size %dx%d is not supported (at most %d px per side and %d Mpx)",
			width, height, maxFrameSide, maxFramePixels>>20)
	}
	return nil
}

// Preprocess decodes a JPEG, resizes it to size×size with x/image/draw's
// antialiased BiLinear kernel (no letterbox, like PIL's BILINEAR resize used
// for the reference numbers) and returns CHW RGB float32 values /255 plus the
// original frame size. Grayscale and CMYK JPEGs are converted to RGB by the
// resize; anything that is not a complete JPEG is an error.
func Preprocess(jpegData []byte, size int) (pixels []float32, width, height int, err error) {
	if size <= 0 {
		return nil, 0, 0, fmt.Errorf("input size must be positive")
	}
	if len(jpegData) == 0 {
		return nil, 0, 0, fmt.Errorf("image is empty")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode JPEG header: %w", err)
	}
	if err := validateFrameSize(config.Width, config.Height); err != nil {
		return nil, 0, 0, err
	}
	source, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode JPEG: %w", err)
	}
	bounds := source.Bounds()
	scaled := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(scaled, scaled.Bounds(), source, bounds, draw.Src, nil)
	plane := size * size
	pixels = make([]float32, 3*plane)
	for y := 0; y < size; y++ {
		row := scaled.Pix[y*scaled.Stride : y*scaled.Stride+4*size]
		for x := 0; x < size; x++ {
			index := y*size + x
			pixels[index] = float32(row[4*x]) / 255
			pixels[plane+index] = float32(row[4*x+1]) / 255
			pixels[2*plane+index] = float32(row[4*x+2]) / 255
		}
	}
	return pixels, bounds.Dx(), bounds.Dy(), nil
}

// DecodePersons turns D-FINE outputs (logits [queries×classes], boxes
// [queries×4] as cx, cy, w, h) into person boxes with sigmoid score ≥
// minScore, clamped to 0..1, degenerate boxes dropped, sorted by score
// (descending). Queries with a non-finite logit or box value never produce a
// box. Invalid sizes, or slices shorter than queries×classes / queries×4,
// return nil; trailing extra values are ignored.
func DecodePersons(logits, boxes []float32, queries, classes, personClass int, minScore float64) []Box {
	if queries <= 0 || classes <= 0 || personClass < 0 || personClass >= classes ||
		queries > len(logits)/classes || queries > len(boxes)/4 {
		return nil
	}
	found := make([]Box, 0, 8)
	for query := 0; query < queries; query++ {
		raw := float64(logits[query*classes+personClass])
		if !finite(raw) {
			continue
		}
		score := sigmoid(raw)
		if !(score >= minScore) {
			continue
		}
		cx, cy := float64(boxes[query*4]), float64(boxes[query*4+1])
		w, h := float64(boxes[query*4+2]), float64(boxes[query*4+3])
		if !finite(cx) || !finite(cy) || !finite(w) || !finite(h) {
			continue
		}
		box := Box{
			X1:    clamp01(cx - w/2),
			Y1:    clamp01(cy - h/2),
			X2:    clamp01(cx + w/2),
			Y2:    clamp01(cy + h/2),
			Score: score,
		}
		if box.X2 <= box.X1 || box.Y2 <= box.Y1 {
			continue
		}
		found = append(found, box)
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Score > found[j].Score })
	return found
}

// IoU is the intersection over union of two boxes (0 for empty unions and for
// boxes with non-finite coordinates).
func IoU(a, b Box) float64 {
	width := math.Max(0, math.Min(a.X2, b.X2)-math.Max(a.X1, b.X1))
	height := math.Max(0, math.Min(a.Y2, b.Y2)-math.Max(a.Y1, b.Y1))
	intersection := width * height
	union := boxArea(a) + boxArea(b) - intersection
	if !(union > 0) {
		return 0
	}
	if value := intersection / union; finite(value) {
		return value
	}
	return 0
}

// SameDetections compares two detection sets: same count, then greedy
// best-IoU matching; every pair needs IoU ≥ minIoU and |score difference| ≤
// maxScoreDiff. The reason names the count mismatch or the worst values over
// all pairs (the minimum IoU and the maximum score difference may come from
// different pairs); two empty sets are the same (the caller decides whether
// that is enough). The greedy matching can reject near-duplicate boxes whose
// scores are crossed, and it requires equal counts, so a box whose score sits
// right at the decode threshold makes it flaky: use SameConfidentDetections to
// compare sets that were decoded at a threshold.
func SameDetections(a, b []Box, minIoU, maxScoreDiff float64) (bool, string) {
	if len(a) != len(b) {
		return false, fmt.Sprintf("count %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return true, "no detections in either set"
	}
	used := make([]bool, len(b))
	worstIoU, worstDiff := 1.0, 0.0
	for _, box := range a {
		best, bestIoU := -1, -1.0
		for j, other := range b {
			if used[j] {
				continue
			}
			if value := IoU(box, other); value > bestIoU {
				best, bestIoU = j, value
			}
		}
		if best < 0 {
			return false, "unmatched detection"
		}
		used[best] = true
		worstIoU = math.Min(worstIoU, bestIoU)
		worstDiff = math.Max(worstDiff, math.Abs(box.Score-b[best].Score))
	}
	reason := fmt.Sprintf("min IoU %.4f, max score diff %.4f", worstIoU, worstDiff)
	return worstIoU >= minIoU && worstDiff <= maxScoreDiff, reason
}

// SameConfidentDetections compares two detection sets that both were decoded
// at threshold-maxScoreDiff, so a box right at the threshold cannot flip the
// verdict through tiny numeric differences between two runtimes. Only
// confident boxes (score ≥ threshold+maxScoreDiff) have to be found in the
// other set: every confident box of a needs a distinct box of b with IoU ≥
// minIoU and |score difference| ≤ maxScoreDiff, and the other way round.
// Boxes in the band below the confidence cutoff may stay unmatched; they can
// still be the partner of a confident box. Each direction is an exact maximum
// bipartite matching (augmenting paths), so the verdict does not depend on the
// order of the boxes or of the arguments and never rejects a pairing that
// exists.
//
// On success the reason reports the matched count and the worst IoU and score
// difference over the matched pairs. On failure it names the first confident
// box without a partner ("set a" and "set b" are the arguments) and its
// closest candidate. Two sets without confident boxes are the same ("no
// confident detections in either set"): callers decide separately whether
// that is enough. Non-finite boxes and invalid parameters never pass.
func SameConfidentDetections(a, b []Box, threshold, minIoU, maxScoreDiff float64) (bool, string) {
	cutoff := threshold + maxScoreDiff
	if !finite(threshold) || !finite(minIoU) || !finite(maxScoreDiff) || maxScoreDiff < 0 || !finite(cutoff) {
		return false, fmt.Sprintf("invalid comparison parameters (threshold %v, min IoU %v, max score diff %v)", threshold, minIoU, maxScoreDiff)
	}
	for _, set := range []struct {
		name  string
		boxes []Box
	}{{"a", a}, {"b", b}} {
		for _, box := range set.boxes {
			if !finite(box.X1) || !finite(box.Y1) || !finite(box.X2) || !finite(box.Y2) || !finite(box.Score) {
				return false, fmt.Sprintf("non-finite detection in set %s", set.name)
			}
		}
	}
	forward, confidentA, failure := matchConfident(a, b, "a", "b", cutoff, minIoU, maxScoreDiff)
	if failure != "" {
		return false, failure
	}
	backward, confidentB, failure := matchConfident(b, a, "b", "a", cutoff, minIoU, maxScoreDiff)
	if failure != "" {
		return false, failure
	}
	if confidentA == 0 && confidentB == 0 {
		return true, "no confident detections in either set"
	}
	worstIoU, worstDiff := 1.0, 0.0
	for _, match := range append(forward, backward...) {
		worstIoU = math.Min(worstIoU, match.iou)
		worstDiff = math.Max(worstDiff, match.scoreDiff)
	}
	plural := "s"
	if confidentA == 1 {
		plural = ""
	}
	if confidentA == confidentB {
		return true, fmt.Sprintf("matched %d confident detection%s (min IoU %.4f, max score diff %.4f)",
			confidentA, plural, worstIoU, worstDiff)
	}
	return true, fmt.Sprintf("matched %d confident detection%s in a and %d in b (min IoU %.4f, max score diff %.4f)",
		confidentA, plural, confidentB, worstIoU, worstDiff)
}

type detectionMatch struct {
	iou, scoreDiff float64
}

type detectionEdge struct {
	to  int
	iou float64
}

// matchConfident assigns every confident box of from (score ≥ cutoff) a
// distinct box of to with Kuhn's augmenting paths. It returns the matched
// pairs and the number of confident boxes, or a failure reason naming the
// first confident box that no augmenting path can place.
func matchConfident(from, to []Box, fromName, toName string, cutoff, minIoU, maxScoreDiff float64) (matches []detectionMatch, confident int, failure string) {
	var sources []int
	for index, box := range from {
		if box.Score >= cutoff {
			sources = append(sources, index)
		}
	}
	if len(sources) == 0 {
		return nil, 0, ""
	}
	// Candidate partners per confident box, best IoU first so the first try is
	// usually the natural pairing.
	edges := make([][]detectionEdge, len(sources))
	for u, index := range sources {
		for j, other := range to {
			iou := IoU(from[index], other)
			if iou >= minIoU && math.Abs(from[index].Score-other.Score) <= maxScoreDiff {
				edges[u] = append(edges[u], detectionEdge{to: j, iou: iou})
			}
		}
		sort.SliceStable(edges[u], func(x, y int) bool { return edges[u][x].iou > edges[u][y].iou })
	}
	owner := make([]int, len(to)) // index into sources, or -1
	for j := range owner {
		owner[j] = -1
	}
	var augment func(u int, visited []bool) bool
	augment = func(u int, visited []bool) bool {
		for _, edge := range edges[u] {
			if visited[edge.to] {
				continue
			}
			visited[edge.to] = true
			if owner[edge.to] < 0 || augment(owner[edge.to], visited) {
				owner[edge.to] = u
				return true
			}
		}
		return false
	}
	for u := range sources {
		if !augment(u, make([]bool, len(to))) {
			return nil, 0, describeUnmatched(from[sources[u]], to, len(edges[u]) > 0, fromName, toName)
		}
	}
	matches = make([]detectionMatch, 0, len(sources))
	for j, u := range owner {
		if u >= 0 {
			box := from[sources[u]]
			matches = append(matches, detectionMatch{iou: IoU(box, to[j]), scoreDiff: math.Abs(box.Score - to[j].Score)})
		}
	}
	return matches, len(sources), ""
}

// describeUnmatched explains why a confident box has no partner: the other set
// is empty, every compatible box is needed by other confident boxes, or no box
// is compatible (then the closest candidate shows which condition failed).
func describeUnmatched(box Box, to []Box, hasCompatible bool, fromName, toName string) string {
	reason := fmt.Sprintf("confident detection in set %s (score %.4f, box [%.3f %.3f %.3f %.3f]) has no match in set %s",
		fromName, box.Score, box.X1, box.Y1, box.X2, box.Y2, toName)
	if len(to) == 0 {
		return reason + fmt.Sprintf(" (set %s is empty)", toName)
	}
	if hasCompatible {
		return reason + fmt.Sprintf(" (its compatible boxes in set %s are all needed by other confident detections)", toName)
	}
	best, bestIoU := 0, -1.0
	for j, other := range to {
		if iou := IoU(box, other); iou > bestIoU {
			best, bestIoU = j, iou
		}
	}
	return reason + fmt.Sprintf(" (best candidate IoU %.4f, score diff %.4f)", bestIoU, math.Abs(box.Score-to[best].Score))
}

func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}

func clamp01(value float64) float64 {
	return math.Min(1, math.Max(0, value))
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func boxArea(box Box) float64 {
	return math.Max(0, box.X2-box.X1) * math.Max(0, box.Y2-box.Y1)
}
