package persondetect

import "sort"

// footPointEdge keeps foot points on the bottom or right frame edge inside zones
// drawn up to that edge: PointInPolygon treats a polygon's bottom and right edges
// as outside, and persons cut off by the frame have Y2 == 1.
const footPointEdge = 1 - 1e-6

// FootPoint is the bottom center of a box, where a standing person touches the
// ground, clamped into [0, footPointEdge].
func FootPoint(b Box) Point {
	return Point{X: clampFootCoordinate((b.X1 + b.X2) / 2), Y: clampFootCoordinate(b.Y2)}
}

func clampFootCoordinate(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > footPointEdge {
		return footPointEdge
	}
	return v
}

// PointInPolygon reports whether p lies inside the polygon under the nonzero
// winding rule, which is also what an SVG fill does by default, so the zone
// editor and the detector agree: an outline that loops back over its own area (a
// rectangle traced twice, a pentagram, a lasso) counts wherever it is filled.
// Polygons with fewer than three points contain nothing.
//
// A ray from p towards +X adds one for every edge that crosses it towards larger
// Y and subtracts one for every edge that crosses it towards smaller Y (Sunday's
// crossing rule); p is inside when the sum is not zero. Edges are half-open in Y
// ([lower, upper)), so a vertex on the ray is counted once and horizontal edges
// never count, and only crossings strictly right of p count. For axis-aligned
// edges that makes a point on a left or top edge inside and a point on a right or
// bottom edge outside; a diamond's left vertex is inside and its right vertex
// outside.
//
// The crossing's x is interpolated from the edge's lower end, whichever way the
// edge is drawn, so two zones sharing an edge compute the very same crossing and
// a point on the shared edge, slanted or not, lands in exactly one of them. On a
// slanted edge "on the edge" can only be as exact as floating point allows.
func PointInPolygon(p Point, polygon []Point) bool {
	if len(polygon) < 3 {
		return false
	}
	winding := 0
	prev := polygon[len(polygon)-1]
	for _, cur := range polygon {
		from, to := prev, cur
		prev = cur
		if (from.Y > p.Y) == (to.Y > p.Y) {
			continue
		}
		direction, lower, upper := 1, from, to
		if from.Y > to.Y {
			direction, lower, upper = -1, to, from
		}
		x := (upper.X-lower.X)*(p.Y-lower.Y)/(upper.Y-lower.Y) + lower.X
		if p.X < x {
			winding += direction
		}
	}
	return winding != 0
}

// ZonesContaining returns the names of the zones whose polygon contains the box's
// foot point, in zone order. The result is never nil, so it marshals as [].
func ZonesContaining(b Box, zones []Zone) []string {
	foot := FootPoint(b)
	names := make([]string, 0)
	for _, zone := range zones {
		if PointInPolygon(foot, zone.Points) {
			names = append(names, zone.Name)
		}
	}
	return names
}

// BoxCounts reports whether a box is a hit: score at or above minScore and, when
// the camera has zones, a foot point inside at least one of them. hit lists the
// zones containing the foot point regardless of the score (never nil). A NaN
// score never counts.
func BoxCounts(b Box, minScore float64, zones []Zone) (counts bool, hit []string) {
	hit = ZonesContaining(b, zones)
	if !(b.Score >= minScore) { // not "<": every comparison with NaN is false
		return false, hit
	}
	return len(zones) == 0 || len(hit) > 0, hit
}

// classifyBoxes returns the hits among boxes, the sorted union of the zones they
// hit and the highest hit score (0 without hits). Both lists are never nil.
func classifyBoxes(boxes []Box, minScore float64, zones []Zone) ([]Box, []string, float64) {
	hits := []Box{}
	set := map[string]struct{}{}
	best := 0.0
	for _, b := range boxes {
		counts, names := BoxCounts(b, minScore, zones)
		if !counts {
			continue
		}
		hits = append(hits, b)
		if b.Score > best {
			best = b.Score
		}
		for _, name := range names {
			set[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return hits, names, best
}
