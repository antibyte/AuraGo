package persondetect

import (
	"reflect"
	"testing"
)

func zoneSquare(x1, y1, x2, y2 float64) []Point {
	return []Point{{x1, y1}, {x2, y1}, {x2, y2}, {x1, y2}}
}

func TestFootPointIsBottomCenterInsideTheFrame(t *testing.T) {
	if got := FootPoint(Box{X1: 0.25, Y1: 0.1, X2: 0.75, Y2: 0.75}); got != (Point{X: 0.5, Y: 0.75}) {
		t.Fatalf("FootPoint = %+v", got)
	}
	if got := FootPoint(Box{X1: 0.5, Y1: 0.5, X2: 1, Y2: 1}); got.X != 0.75 || got.Y != footPointEdge {
		t.Fatalf("bottom edge foot point = %+v", got)
	}
	bottom := Zone{Name: "Bottom", Points: zoneSquare(0, 0.5, 1, 1)}
	if counts, _ := BoxCounts(Box{X1: 0.25, Y1: 0.6, X2: 0.75, Y2: 1, Score: 0.9}, 0.5, []Zone{bottom}); !counts {
		t.Fatal("a person cut off by the bottom frame edge must count in a zone drawn to that edge")
	}
}

func TestPointInPolygonConvexConcaveAndEdges(t *testing.T) {
	square := zoneSquare(0.2, 0.2, 0.8, 0.8)
	for _, tc := range []struct {
		p    Point
		want bool
	}{
		{Point{0.5, 0.5}, true},
		{Point{0.1, 0.5}, false},
		{Point{0.2, 0.5}, true},  // left edge counts as inside
		{Point{0.8, 0.5}, false}, // right edge counts as outside
		{Point{0.5, 0.2}, true},  // top edge inside
		{Point{0.5, 0.8}, false}, // bottom edge outside
	} {
		if got := PointInPolygon(tc.p, square); got != tc.want {
			t.Errorf("square contains %+v = %v, want %v", tc.p, got, tc.want)
		}
	}
	notched := []Point{{0.1, 0.1}, {0.9, 0.1}, {0.9, 0.9}, {0.6, 0.9}, {0.6, 0.3}, {0.4, 0.3}, {0.4, 0.9}, {0.1, 0.9}}
	for _, tc := range []struct {
		p    Point
		want bool
	}{
		{Point{0.5, 0.6}, false}, // inside the notch
		{Point{0.2, 0.6}, true},  // left leg
		{Point{0.8, 0.6}, true},  // right leg
		{Point{0.5, 0.2}, true},  // top bar
	} {
		if got := PointInPolygon(tc.p, notched); got != tc.want {
			t.Errorf("concave polygon contains %+v = %v, want %v", tc.p, got, tc.want)
		}
	}
	if PointInPolygon(Point{0.5, 0.5}, square[:2]) {
		t.Fatal("a polygon with two points contains nothing")
	}
}

func TestZonesContainingAndBoxCounts(t *testing.T) {
	left := Zone{Name: "Left", Points: zoneSquare(0, 0, 0.5, 1)}
	all := Zone{Name: "All", Points: zoneSquare(0, 0, 1, 1)}
	inLeft := Box{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.7}
	inRight := Box{X1: 0.6, Y1: 0.2, X2: 0.8, Y2: 0.9, Score: 0.7}
	if got := ZonesContaining(inLeft, []Zone{left, all}); !reflect.DeepEqual(got, []string{"Left", "All"}) {
		t.Fatalf("left box zones = %v", got)
	}
	if got := ZonesContaining(inRight, []Zone{left, all}); !reflect.DeepEqual(got, []string{"All"}) {
		t.Fatalf("right box zones = %v", got)
	}
	if counts, hit := BoxCounts(inRight, 0.5, []Zone{left}); counts || len(hit) != 0 {
		t.Fatalf("box outside every zone counted: %v %v", counts, hit)
	}
	weak := inLeft
	weak.Score = 0.4
	if counts, hit := BoxCounts(weak, 0.5, []Zone{left}); counts || !reflect.DeepEqual(hit, []string{"Left"}) {
		t.Fatalf("weak box = %v %v, want not counting but inside Left", counts, hit)
	}
	if counts, _ := BoxCounts(inRight, 0.5, nil); !counts {
		t.Fatal("without zones every box at or above the threshold counts")
	}
	exact := inRight
	exact.Score = 0.5
	if counts, _ := BoxCounts(exact, 0.5, nil); !counts {
		t.Fatal("a score equal to the threshold counts")
	}
}

func TestClassifyBoxes(t *testing.T) {
	left := Zone{Name: "Left", Points: zoneSquare(0, 0, 0.5, 1)}
	bottom := Zone{Name: "Bottom", Points: zoneSquare(0, 0.5, 1, 1)}
	boxes := []Box{
		{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.7},
		{X1: 0.6, Y1: 0.2, X2: 0.8, Y2: 0.9, Score: 0.9},
		{X1: 0.1, Y1: 0.1, X2: 0.2, Y2: 0.3, Score: 0.4},
	}
	hits, zones, best := classifyBoxes(boxes, 0.5, []Zone{left, bottom})
	if len(hits) != 2 || hits[1].Score != 0.9 || !reflect.DeepEqual(zones, []string{"Bottom", "Left"}) || best != 0.9 {
		t.Fatalf("hits = %+v zones = %v best = %v", hits, zones, best)
	}
	if hits, zones, best := classifyBoxes(nil, 0.5, nil); len(hits) != 0 || len(zones) != 0 || zones == nil || best != 0 {
		t.Fatalf("empty frame = %v %v %v", hits, zones, best)
	}
}
