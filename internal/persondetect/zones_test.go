package persondetect

import (
	"encoding/json"
	"math"
	"math/rand"
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
	// Only the below-threshold box stands in Top; the union must not contain it.
	top := Zone{Name: "Top", Points: zoneSquare(0, 0.2, 1, 0.4)}
	boxes := []Box{
		{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.7},
		{X1: 0.6, Y1: 0.2, X2: 0.8, Y2: 0.9, Score: 0.9},
		{X1: 0.1, Y1: 0.1, X2: 0.2, Y2: 0.3, Score: 0.4},
	}
	if got := ZonesContaining(boxes[2], []Zone{top}); !reflect.DeepEqual(got, []string{"Top"}) {
		t.Fatalf("fixture: the weak box must stand in Top, got %v", got)
	}
	hits, zones, best := classifyBoxes(boxes, 0.5, []Zone{left, bottom, top})
	if len(hits) != 2 || hits[1].Score != 0.9 || !reflect.DeepEqual(zones, []string{"Bottom", "Left"}) || best != 0.9 {
		t.Fatalf("hits = %+v zones = %v best = %v", hits, zones, best)
	}
	if hits, zones, best := classifyBoxes(nil, 0.5, nil); len(hits) != 0 || len(zones) != 0 || zones == nil || best != 0 {
		t.Fatalf("empty frame = %v %v %v", hits, zones, best)
	}
}

// reversePoints returns the polygon drawn in the opposite direction.
func reversePoints(pts []Point) []Point {
	out := make([]Point, len(pts))
	for i, p := range pts {
		out[len(pts)-1-i] = p
	}
	return out
}

// membership reports in which of the polygons a point lies.
func membership(p Point, polygons ...[]Point) []bool {
	out := make([]bool, len(polygons))
	for i, polygon := range polygons {
		out[i] = PointInPolygon(p, polygon)
	}
	return out
}

func countTrue(flags []bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

// The slice 4 zone editor fills polygons with the SVG default (nonzero) rule, so
// everything the editor shows filled must count. Even-odd would leave holes where
// an outline winds over its own area.
func TestPointInPolygonUsesNonzeroWinding(t *testing.T) {
	var star []Point
	for k := 0; k < 5; k++ {
		a := -math.Pi/2 + float64(k)*4*math.Pi/5
		star = append(star, Point{0.5 + 0.4*math.Cos(a), 0.5 + 0.4*math.Sin(a)})
	}
	for _, tc := range []struct {
		name string
		p    Point
		want bool
	}{
		{"pentagram centre (wound twice)", Point{0.5, 0.5}, true},
		{"pentagram top tip", Point{0.5, 0.2}, true},
		{"pentagram outside", Point{0.05, 0.05}, false},
	} {
		if got := PointInPolygon(tc.p, star); got != tc.want {
			t.Errorf("%s: contains %+v = %v, want %v", tc.name, tc.p, got, tc.want)
		}
	}

	twice := []Point{{0.2, 0.2}, {0.8, 0.2}, {0.8, 0.8}, {0.2, 0.8}, {0.2, 0.2}, {0.8, 0.2}, {0.8, 0.8}, {0.2, 0.8}}
	for _, tc := range []struct {
		name string
		p    Point
		want bool
	}{
		{"rectangle traversed twice, centre", Point{0.5, 0.5}, true},
		{"rectangle traversed twice, left edge", Point{0.2, 0.5}, true},
		{"rectangle traversed twice, right edge", Point{0.8, 0.5}, false},
		{"rectangle traversed twice, outside", Point{0.1, 0.5}, false},
	} {
		if got := PointInPolygon(tc.p, twice); got != tc.want {
			t.Errorf("%s: contains %+v = %v, want %v", tc.name, tc.p, got, tc.want)
		}
		if got := PointInPolygon(tc.p, reversePoints(twice)); got != tc.want {
			t.Errorf("%s (reversed): contains %+v = %v, want %v", tc.name, tc.p, got, tc.want)
		}
	}

	// A lasso drawn as one outline around two overlapping squares, the second
	// started along a connector that is walked there and back (it cancels out).
	lasso := []Point{{0.1, 0.1}, {0.6, 0.1}, {0.6, 0.6}, {0.1, 0.6}, {0.1, 0.1}, {0.3, 0.3}, {0.9, 0.3}, {0.9, 0.9}, {0.3, 0.9}, {0.3, 0.3}}
	for _, tc := range []struct {
		name string
		p    Point
		want bool
	}{
		{"lasso overlap of both squares", Point{0.45, 0.45}, true},
		{"lasso first square only", Point{0.2, 0.5}, true},
		{"lasso second square only", Point{0.75, 0.7}, true},
		{"lasso outside", Point{0.05, 0.5}, false},
	} {
		if got := PointInPolygon(tc.p, lasso); got != tc.want {
			t.Errorf("%s: contains %+v = %v, want %v", tc.name, tc.p, got, tc.want)
		}
	}
}

// The bow-tie crosses itself at (0.5, 0.5); all coordinates are exact binary
// fractions, so the crossing is computed exactly.
func TestPointInPolygonBowTieHalves(t *testing.T) {
	bowTie := []Point{{0.125, 0.125}, {0.875, 0.875}, {0.875, 0.125}, {0.125, 0.875}}
	for _, polygon := range [][]Point{bowTie, reversePoints(bowTie)} {
		for _, tc := range []struct {
			name string
			p    Point
			want bool
		}{
			{"left half", Point{0.25, 0.5}, true},
			{"right half", Point{0.75, 0.5}, true},
			{"above the crossing", Point{0.5, 0.25}, false},
			{"below the crossing", Point{0.5, 0.75}, false},
			// Neither diagonal lies strictly right of the crossing point, so only the
			// right-hand side of the figure is counted: it belongs to the right half.
			{"crossing point", Point{0.5, 0.5}, true},
		} {
			if got := PointInPolygon(tc.p, polygon); got != tc.want {
				t.Errorf("bow-tie %s: contains %+v = %v, want %v", tc.name, tc.p, got, tc.want)
			}
		}
	}
}

func TestPointInPolygonDiamondVertices(t *testing.T) {
	diamond := []Point{{0.5, 0.1}, {0.9, 0.5}, {0.5, 0.9}, {0.1, 0.5}}
	for _, tc := range []struct {
		name string
		p    Point
		want bool
	}{
		{"inside", Point{0.5, 0.5}, true},
		{"right vertex", Point{0.9, 0.5}, false},
		{"left vertex", Point{0.1, 0.5}, true},
		{"top vertex", Point{0.5, 0.1}, false},
		{"bottom vertex", Point{0.5, 0.9}, false},
		{"left of the right vertex row", Point{0.3, 0.5}, true},
		{"right of the right vertex", Point{0.95, 0.5}, false},
	} {
		for _, polygon := range [][]Point{diamond, reversePoints(diamond)} {
			if got := PointInPolygon(tc.p, polygon); got != tc.want {
				t.Errorf("diamond %s: contains %+v = %v, want %v", tc.name, tc.p, got, tc.want)
			}
		}
	}
}

func TestSharedAxisAlignedEdgesBelongToExactlyOneZone(t *testing.T) {
	left := zoneSquare(0, 0, 0.5, 1)
	right := zoneSquare(0.5, 0, 1, 1)
	top := zoneSquare(0, 0, 1, 0.5)
	bottom := zoneSquare(0, 0.5, 1, 1)
	for _, y := range []float64{0, 0.1, 0.25, 0.5, 0.75, 0.999999} {
		p := Point{0.5, y}
		for _, flags := range [][]bool{
			membership(p, left, right),
			membership(p, left, reversePoints(right)),
			membership(p, reversePoints(left), right),
		} {
			if countTrue(flags) != 1 || flags[0] {
				t.Errorf("vertical shared edge at %+v: left/right = %v, want only the right zone", p, flags)
			}
		}
	}
	for _, x := range []float64{0, 0.1, 0.5, 0.75, 0.999999} {
		p := Point{x, 0.5}
		for _, flags := range [][]bool{
			membership(p, top, bottom),
			membership(p, top, reversePoints(bottom)),
			membership(p, reversePoints(top), bottom),
		} {
			if countTrue(flags) != 1 || flags[0] {
				t.Errorf("horizontal shared edge at %+v: top/bottom = %v, want only the bottom zone", p, flags)
			}
		}
	}
	// Four quadrants meet at the centre: exactly one claims it (the bottom right).
	quadrants := [][]Point{zoneSquare(0, 0, 0.5, 0.5), zoneSquare(0.5, 0, 1, 0.5), zoneSquare(0, 0.5, 0.5, 1), zoneSquare(0.5, 0.5, 1, 1)}
	if flags := membership(Point{0.5, 0.5}, quadrants...); !reflect.DeepEqual(flags, []bool{false, false, false, true}) {
		t.Errorf("quadrant centre membership = %v, want only the bottom right quadrant", flags)
	}
}

// Both zones interpolate the shared edge from its lower end whatever the drawing
// direction, so a point on or next to the edge is never in both zones or in none.
func TestSharedSlantedEdgeBelongsToExactlyOneZone(t *testing.T) {
	e1, e2 := Point{0.25, 0}, Point{0.75, 1}
	left := []Point{{0, 0}, e1, e2, {0, 1}}  // walks the shared edge e1 -> e2
	right := []Point{e1, {1, 0}, {1, 1}, e2} // closes along e2 -> e1
	for k := 0; k < 16; k++ {
		y := float64(k) / 16
		p := Point{0.25 + 0.5*y, y} // exactly on the edge
		for name, flags := range map[string][]bool{
			"opposite traversal": membership(p, left, right),
			"same traversal":     membership(p, left, reversePoints(right)),
			"both reversed":      membership(p, reversePoints(left), reversePoints(right)),
			"left reversed":      membership(p, reversePoints(left), right),
		} {
			if countTrue(flags) != 1 {
				t.Errorf("%s: point %+v on the shared edge is in %v, want exactly one zone", name, p, flags)
			}
		}
	}

	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 20000; trial++ {
		a := Point{0.3 + rng.Float64()*0.4, 0.05 + rng.Float64()*0.2}
		b := Point{0.3 + rng.Float64()*0.4, 0.75 + rng.Float64()*0.2}
		leftTri := []Point{a, b, {0.01, 0.5}}
		rightTri := []Point{b, a, {0.99, 0.5}}
		rightSame := []Point{a, b, {0.99, 0.5}}
		y := a.Y + rng.Float64()*(b.Y-a.Y)
		p := Point{a.X + (b.X-a.X)*((y-a.Y)/(b.Y-a.Y)), y} // on the edge up to rounding
		if n := countTrue(membership(p, leftTri, rightTri)); n != 1 {
			t.Fatalf("trial %d: %+v on edge %+v-%+v is in %d of the opposite-traversal zones", trial, p, a, b, n)
		}
		if n := countTrue(membership(p, leftTri, rightSame)); n != 1 {
			t.Fatalf("trial %d: %+v on edge %+v-%+v is in %d of the same-traversal zones", trial, p, a, b, n)
		}
	}
}

func TestBoxCountsNeverCountsANaNScore(t *testing.T) {
	box := Box{X1: 0.1, Y1: 0.1, X2: 0.3, Y2: 0.9, Score: math.NaN()}
	all := Zone{Name: "All", Points: zoneSquare(0, 0, 1, 1)}
	for _, zones := range [][]Zone{nil, {all}} {
		if counts, _ := BoxCounts(box, 0.5, zones); counts {
			t.Errorf("a NaN score counted with %d zones", len(zones))
		}
	}
	if hits, names, best := classifyBoxes([]Box{box}, 0.5, nil); len(hits) != 0 || len(names) != 0 || best != 0 {
		t.Errorf("NaN-score box classified as hit: %v %v %v", hits, names, best)
	}
}

func TestEmptyClassificationResultsMarshalAsEmptyLists(t *testing.T) {
	left := []Zone{{Name: "Left", Points: zoneSquare(0, 0, 0.5, 1)}}
	right := Box{X1: 0.6, Y1: 0.2, X2: 0.8, Y2: 0.9, Score: 0.9}
	weakLeft := Box{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.1}
	_, noZonesHit := BoxCounts(right, 0.5, nil)
	_, outsideHit := BoxCounts(right, 0.5, left)
	noHits, noNames, _ := classifyBoxes(nil, 0.5, nil)
	weakHits, weakNames, _ := classifyBoxes([]Box{weakLeft}, 0.5, left)
	for name, value := range map[string]any{
		"ZonesContaining without zones":      ZonesContaining(right, nil),
		"ZonesContaining outside every zone": ZonesContaining(right, left),
		"BoxCounts hit without zones":        noZonesHit,
		"BoxCounts hit outside every zone":   outsideHit,
		"classifyBoxes hits for no boxes":    noHits,
		"classifyBoxes zones for no boxes":   noNames,
		"classifyBoxes hits for weak box":    weakHits,
		"classifyBoxes zones for weak box":   weakNames,
	} {
		data, err := json.Marshal(value)
		if err != nil || string(data) != "[]" {
			t.Errorf("%s marshals as %s (%v), want []", name, data, err)
		}
	}
}
