package persondetect

import "sort"

// footPointEdge keeps foot points on the bottom or right frame edge inside zones
// drawn up to that edge: the even-odd test treats a polygon's bottom and right
// edges as outside, and persons cut off by the frame have Y2 == 1.
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

// PointInPolygon applies the even-odd rule. Points on a left or top edge are
// inside, points on a right or bottom edge outside. Polygons with fewer than
// three points contain nothing.
func PointInPolygon(p Point, polygon []Point) bool {
	if len(polygon) < 3 {
		return false
	}
	inside := false
	j := len(polygon) - 1
	for i := 0; i < len(polygon); i++ {
		a, b := polygon[i], polygon[j]
		if (a.Y > p.Y) != (b.Y > p.Y) {
			x := (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y) + a.X
			if p.X < x {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// ZonesContaining returns the names of the zones whose polygon contains the box's
// foot point, in zone order.
func ZonesContaining(b Box, zones []Zone) []string {
	foot := FootPoint(b)
	var names []string
	for _, zone := range zones {
		if PointInPolygon(foot, zone.Points) {
			names = append(names, zone.Name)
		}
	}
	return names
}

// BoxCounts reports whether a box is a hit: score at or above minScore and, when
// the camera has zones, a foot point inside at least one of them. hit lists the
// zones containing the foot point regardless of the score.
func BoxCounts(b Box, minScore float64, zones []Zone) (counts bool, hit []string) {
	hit = ZonesContaining(b, zones)
	if b.Score < minScore {
		return false, hit
	}
	return len(zones) == 0 || len(hit) > 0, hit
}

// classifyBoxes returns the hits among boxes, the sorted union of the zones they
// hit and the highest hit score (0 without hits).
func classifyBoxes(boxes []Box, minScore float64, zones []Zone) ([]Box, []string, float64) {
	var hits []Box
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
