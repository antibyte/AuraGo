package ui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type goldfishHotspotProfile struct {
	Name           string  `json:"name"`
	ViewportWidth  int     `json:"viewportWidth"`
	ViewportHeight int     `json:"viewportHeight"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	DPR            float64 `json:"dpr"`
	Theme          string  `json:"theme"`
}

type goldfishArtTarget struct {
	ID       string                  `json:"id"`
	Landmark string                  `json:"landmark"`
	Points   [][2]float64            `json:"points"`
	Variants map[string][][2]float64 `json:"variants,omitempty"`
}

type goldfishArtReference struct {
	Room             string              `json:"room"`
	VisualSignature  string              `json:"visualSignature"`
	Targets          []goldfishArtTarget `json:"targets"`
	Exits            []goldfishArtTarget `json:"exits"`
	BackgroundPoints [][2]float64        `json:"backgroundPoints"`
}

func goldfishReadArtReference(t *testing.T, room string) goldfishArtReference {
	t.Helper()
	var ref goldfishArtReference
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/goldfish-hotspots/"+room+".json")), &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Room != room || len(ref.Targets) != 5 {
		t.Fatalf("incomplete art reference for %s", room)
	}
	for _, target := range ref.Targets {
		if target.Landmark == "" || len(target.Points) < 3 {
			t.Fatalf("%s/%s needs a visible landmark and at least three art points", room, target.ID)
		}
		pointSets := [][][2]float64{target.Points}
		for _, points := range target.Variants {
			pointSets = append(pointSets, points)
		}
		for _, points := range pointSets {
			if len(points) < 3 {
				t.Fatalf("%s/%s variant needs at least three art points", room, target.ID)
			}
			for _, point := range points {
				if point[0] <= 0 || point[0] >= 1 || point[1] <= 0 || point[1] >= 1 {
					t.Fatalf("%s/%s art point outside scene", room, target.ID)
				}
			}
		}
	}
	return ref
}

// Bind visual approval to the complete scene inputs, including transparent actor pixels and pivots.
func goldfishVisualSignature(t *testing.T, room string) string {
	t.Helper()
	var manifest struct {
		Rooms   map[string]map[string]any `json:"rooms"`
		Sprites map[string]map[string]any `json:"sprites"`
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "img/goldfish-adventure/manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	art := manifest.Rooms[room]
	delete(art, "audio")
	sprites := map[string]any{"r0st": manifest.Sprites["r0st"]}
	for _, actor := range art["actors"].([]any) {
		value := actor.(map[string]any)
		id := value["id"].(string)
		if sprite, ok := value["sprite"].(string); ok {
			id = sprite
		}
		sprites[id] = manifest.Sprites[id]
	}
	data, err := json.Marshal(map[string]any{"room": art, "sprites": sprites})
	if err != nil {
		t.Fatal(err)
	}
	images := map[string]bool{}
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for _, child := range v {
				collect(child)
			}
		case []any:
			for _, child := range v {
				collect(child)
			}
		case string:
			if strings.HasPrefix(v, "/img/goldfish-adventure/") && strings.HasSuffix(v, ".png") {
				images[v] = true
			}
		}
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	collect(decoded)
	hash := sha256.New()
	hash.Write(data)
	var paths []string
	for path := range images {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		pixels, err := os.ReadFile(filepath.FromSlash(strings.TrimPrefix(path, "/")))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(pixels)
		fmt.Fprintf(hash, "\n%s:%x", path, sum)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func TestGoldfishAdventureHotspotReferenceIntegrity(t *testing.T) {
	signatures := map[string]string{}
	for number := 1; number <= 16; number++ {
		room := fmt.Sprintf("R%02d", number)
		signature := goldfishVisualSignature(t, room)
		signatures[room] = signature
		ref := goldfishReadArtReference(t, room)
		if ref.VisualSignature != signature {
			t.Errorf("%s artwork/staging changed: visual re-review required (signature %s)", room, signature)
		}
	}
	if base := goldfishHotspotArtifactDir(t, ""); base != "" {
		data, err := json.MarshalIndent(signatures, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "visual-signatures.json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

type goldfishPointerResult struct {
	Room       string     `json:"room"`
	Variant    int        `json:"variant"`
	Profile    string     `json:"profile"`
	Target     string     `json:"target"`
	Point      [2]float64 `json:"point"`
	Expected   string     `json:"expected"`
	Observed   string     `json:"observed"`
	Passed     bool       `json:"passed"`
	Screenshot string     `json:"screenshot,omitempty"`
	Kind       string     `json:"kind"`
}

func goldfishWriteEvidence(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		err = os.WriteFile(path, append(data, '\n'), 0644)
	}
	if err != nil {
		t.Error(err)
	}
}

func goldfishPointerMove(page *rod.Page, point [2]float64) {
	position := page.MustEval(`point => {
        const stage=document.querySelector('.agfish-stage'), canvas=document.querySelector('.agfish-canvas');
        if(!stage||!canvas) throw new Error('Goldfish stage is missing');
        let r=canvas.getBoundingClientRect(), x=r.x+point[0]*r.width, y=r.y+point[1]*r.height;
        if(x<0||y<0||x>=innerWidth||y>=innerHeight){
            stage.scrollIntoView({block:'nearest',inline:'nearest',behavior:'instant'});
            r=canvas.getBoundingClientRect(); x=r.x+point[0]*r.width; y=r.y+point[1]*r.height;
        }
        const hit=document.elementFromPoint(x,y);
        if(r.width<=0||r.height<=0||x<0||y<0||x>=innerWidth||y>=innerHeight||hit?.closest('.agfish-stage')!==stage)
            throw new Error('Goldfish pointer point is not visible: '+JSON.stringify({x,y,width:r.width,height:r.height,hit:hit?.className||hit?.tagName||''}));
        return [x,y];
    }`, point).Arr()
	page.Mouse.MustMoveTo(position[0].Num(), position[1].Num())
}

func TestGoldfishAdventureHotspotTargetsBrowser(t *testing.T) {
	for number := 1; number <= 16; number++ {
		room := fmt.Sprintf("R%02d", number)
		t.Run(room, func(t *testing.T) {
			ref := goldfishReadArtReference(t, room)
			page := goldfishHotspotFixture(t)
			inventory := page.MustEval(`room => { const r=GoldfishContent.rooms.find(r=>r.id===room); return {targets:r.hotspots.map(h=>h.id).sort(), exits:r.exits.map(e=>e.id).sort()}; }`, room)
			var targetIDs, exitIDs []string
			for _, target := range ref.Targets {
				targetIDs = append(targetIDs, target.ID)
			}
			sort.Strings(targetIDs)
			for _, exit := range ref.Exits {
				exitIDs = append(exitIDs, exit.ID)
				if len(exit.Points) == 0 {
					t.Fatalf("%s has no visible arrow point", exit.ID)
				}
			}
			sort.Strings(exitIDs)
			if !page.MustEval(`(room, targets, exits) => {const r=GoldfishContent.rooms.find(r=>r.id===room);return JSON.stringify(r.hotspots.map(h=>h.id).sort())===JSON.stringify(targets) && JSON.stringify(r.exits.map(e=>e.id).sort())===JSON.stringify(exits||[]); }`, room, targetIDs, exitIDs).Bool() {
				t.Fatalf("reference inventory differs from authored room: %s", inventory.JSON("", ""))
			}
			base := goldfishHotspotArtifactDir(t, room)
			var results []goldfishPointerResult
			t.Cleanup(func() {
				if base != "" {
					data, err := json.MarshalIndent(results, "", "  ")
					if err != nil {
						t.Error(err)
						return
					}
					if err = os.WriteFile(filepath.Join(base, "results.json"), append(data, '\n'), 0644); err != nil {
						t.Error(err)
					}
				}
			})
			variants := page.MustEval(`room => GoldfishHotspotFixture.rooms[room].length`, room).Int()
			for _, profile := range goldfishHotspotProfiles {
				t.Run(profile.Name, func(t *testing.T) {
					for variant := 0; variant < variants; variant++ {
						goldfishOpenHotspotState(page, room, variant, profile)
						if !page.MustEval(`()=>document.querySelectorAll('button[data-agfish-hotspot]').length===5 && [...document.querySelectorAll('span[data-agfish-hotspot]')].every(n=>n.tabIndex<0&&n.getAttribute('aria-hidden')==='true')`).Bool() {
							t.Error("composite regions must retain exactly one keyboard target per object")
						}
						geometry := page.MustEval(`() => GoldfishHotspotFixture.geometry()`)
						cw, ch := geometry.Get("canvas.width").Num(), geometry.Get("canvas.height").Num()
						wantWidth := math.Max(320, math.Min(2560, math.Round(geometry.Get("clientWidth").Num()*profile.DPR)))
						if math.Abs(cw/ch-16.0/9) > 0.01 || math.Abs(cw-geometry.Get("scene.width").Num()) > 1 || geometry.Get("backing.0").Num() != wantWidth || math.Abs(geometry.Get("window.width").Num()-float64(profile.Width)) > 1 || math.Abs(geometry.Get("window.height").Num()-float64(profile.Height)) > 1 {
							t.Errorf("%s v%d invalid scene sizing: %s", room, variant, geometry.JSON("", ""))
						}
						shot := ""
						if base != "" {
							shot = filepath.Join(base, fmt.Sprintf("%s-v%02d.png", profile.Name, variant))
							page.MustScreenshot(shot)
						}
						variantKey := page.MustEval(`(room,v)=>{const s=GoldfishHotspotFixture.rooms[room][v];return s.ending||s.routes.hall||'';}`, room, variant).Str()
						probe := func(id, kind string, point [2]float64) bool {
							goldfishPointerMove(page, point)
							actual := page.MustEval(`point=>{const r=document.querySelector('.agfish-canvas').getBoundingClientRect();const n=document.elementFromPoint(r.x+point[0]*r.width,r.y+point[1]*r.height)?.closest('[data-agfish-hotspot],[data-agfish-exit]');return {id:n?.dataset.agfishHotspot||n?.dataset.agfishExit||'', sentence:document.querySelector('.agfish-sentence').textContent};}`, point)
							expected := page.MustEval(`(room,id,kind)=>{const r=GoldfishContent.rooms.find(r=>r.id===room),v=kind==='hotspot'?r.hotspots.find(h=>h.id===id):r.exits.find(e=>e.id===id);return v?{id:kind==='hotspot'?v.id:v.to,label:GoldfishContent.texts[v.label]}:{id:'',label:''};}`, room, id, kind)
							ok := actual.Get("id").Str() == expected.Get("id").Str() && strings.Contains(actual.Get("sentence").Str(), expected.Get("label").Str())
							results = append(results, goldfishPointerResult{room, variant, profile.Name, id, point, expected.Get("id").Str() + " / " + expected.Get("label").Str(), actual.Get("id").Str() + " / " + actual.Get("sentence").Str(), ok, shot, "hover"})
							if !ok {
								t.Errorf("%s v%d %s at %v: got %s", room, variant, id, point, actual.JSON("", ""))
							}
							return ok
						}
						for _, target := range ref.Targets {
							points := target.Points
							if alternate := target.Variants[variantKey]; len(alternate) > 0 {
								points = alternate
							}
							passed := true
							for _, point := range points {
								if !probe(target.ID, "hotspot", point) {
									passed = false
								}
							}
							if !passed {
								continue
							}
							goldfishPointerMove(page, points[0])
							page.Mouse.MustClick(proto.InputMouseButtonRight)
							err := page.Timeout(8 * time.Second).Wait(rod.Eval(`(room,v,id)=>{const s=GoldfishHotspotFixture.rooms[room][v],h=GoldfishContent.rooms.find(r=>r.id===room).hotspots.find(h=>h.id===id);const first=GoldfishState.lines(s,h.look)[0];return !document.querySelector('.agfish-speech').hidden && document.querySelector('.agfish-line').textContent===GoldfishContent.texts[first.text];}`, room, variant, target.ID))
							if err != nil {
								t.Errorf("%s %s look response: %v; actual=%s", room, target.ID, err, page.MustEval(`()=>document.querySelector('.agfish-line').textContent`).Str())
							}
							want := page.MustEval(`(room,v,id)=>{const s=GoldfishHotspotFixture.rooms[room][v],h=GoldfishContent.rooms.find(r=>r.id===room).hotspots.find(h=>h.id===id);return GoldfishContent.texts[GoldfishState.lines(s,h.look)[0].text];}`, room, variant, target.ID).Str()
							actual := page.MustEval(`()=>document.querySelector('.agfish-line').textContent`).Str()
							results = append(results, goldfishPointerResult{room, variant, profile.Name, target.ID, points[0], want, actual, err == nil, shot, "look"})
							page.MustEval(`()=>GoldfishHotspotFixture.dismiss()`)
						}
						for _, point := range ref.BackgroundPoints {
							probe("background", "background", point)
						}
						for _, exit := range ref.Exits {
							goldfishOpenHotspotState(page, room, variant, profile)
							if !probe(exit.ID, "exit", exit.Points[0]) {
								continue
							}
							expected := page.MustEval(`(room,v,id)=>{const s=GoldfishHotspotFixture.rooms[room][v],e=GoldfishContent.rooms.find(r=>r.id===room).exits.find(e=>e.id===id);return {open:GoldfishState.conditions(s,e),to:e.to,line:GoldfishState.lines(s,e.blocked)[0]?.text};}`, room, variant, exit.ID)
							page.Mouse.MustClick(proto.InputMouseButtonLeft)
							err := page.Timeout(10 * time.Second).Wait(rod.Eval(`(room,e)=>e.open?document.querySelector('.agfish-app').dataset.room===e.to&&!document.querySelector('.agfish-busy,.agfish-art-loading'):document.querySelector('.agfish-app').dataset.room===room&&!document.querySelector('.agfish-speech').hidden&&document.querySelector('.agfish-line').textContent===GoldfishContent.texts[e.line]`, room, expected))
							if err != nil {
								t.Errorf("%s exit %s open=%v: %v", room, exit.ID, expected.Get("open").Bool(), err)
							}
							actual := page.MustEval(`()=>({room:document.querySelector('.agfish-app').dataset.room,line:document.querySelector('.agfish-line').textContent})`)
							results = append(results, goldfishPointerResult{room, variant, profile.Name, exit.ID, exit.Points[0], expected.JSON("", ""), actual.JSON("", ""), err == nil, shot, "exit"})
							page.MustEval(`()=>GoldfishHotspotFixture.dismiss()`)
						}
					}
				})
			}
			if errors := page.MustEval(`()=>fixtureErrors.join('\n')`).Str(); errors != "" {
				t.Error(errors)
			}
		})
	}
}

var goldfishHotspotProfiles = []goldfishHotspotProfile{
	{"wide", 1440, 1000, 980, 680, 1, "standard"},
	{"narrow", 1280, 900, 700, 660, 1, "fruity-light"},
	{"short", 1280, 900, 900, 480, 1, "fruity-dark"},
	{"minimum-hidpi", 1024, 768, 360, 280, 2, "standard"},
}

func goldfishHotspotFixture(t *testing.T) *rod.Page {
	t.Helper()
	// Give the full action matrix the same 30-minute budget as its Go test.
	page := auraDriftFixture(t).CancelTimeout().Timeout(30 * time.Minute)
	t.Cleanup(func() { page.Close() })
	goldfishManualReading(page)
	page.MustEval(`async () => { await fixtureOpen('goldfish-adventure'); }`)
	page.MustWait(`() => !!window.GoldfishSave && !!document.querySelector('.agfish-start') && !document.querySelector('.agfish-art-loading')`)
	page.MustEval(`source => { (0, eval)(source); }`, readDesktopAssetText(t, "testdata/goldfish-hotspot-fixture.js"))
	coverage := page.MustEval(`async () => await GoldfishHotspotFixture.build()`)
	if coverage.Get("journeys").Int() != 24 || len(coverage.Get("variants").Map()) != 16 {
		t.Fatalf("incomplete reachable state matrix: %s", coverage.JSON("", ""))
	}
	if base := goldfishHotspotArtifactDir(t, ""); base != "" {
		goldfishWriteEvidence(t, filepath.Join(base, "coverage.json"), coverage)
	}
	return page
}

func goldfishOpenHotspotState(page *rod.Page, room string, variant int, profile goldfishHotspotProfile) {
	page.MustSetViewport(profile.ViewportWidth, profile.ViewportHeight, profile.DPR, false)
	page.MustEval(`async (room, variant, profile) => await GoldfishHotspotFixture.open(GoldfishHotspotFixture.rooms[room][variant], profile)`, room, variant, profile)
	page.MustElement(".agfish-start .agfish-primary:not([hidden])").MustClick()
	page.MustEval(`async () => await GoldfishHotspotFixture.ready()`)
}

func goldfishHotspotArtifactDir(t *testing.T, suffix string) string {
	t.Helper()
	base := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if base == "" {
		return ""
	}
	base = filepath.Join(base, suffix)
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	return base
}

// These are real app screenshots: the semantic targets, arrows and actors share the same scene.
func TestGoldfishAdventureHotspotScenesBrowser(t *testing.T) {
	page := goldfishHotspotFixture(t)
	base := goldfishHotspotArtifactDir(t, "app-scenes")
	for number := 1; number <= 16; number++ {
		room := fmt.Sprintf("R%02d", number)
		count := page.MustEval(`room => GoldfishHotspotFixture.rooms[room].length`, room).Int()
		for variant := 0; variant < count; variant++ {
			goldfishOpenHotspotState(page, room, variant, goldfishHotspotProfiles[0])
			if base != "" {
				stem := filepath.Join(base, fmt.Sprintf("%s-v%02d", room, variant))
				page.MustScreenshot(stem + ".png")
				ref := goldfishReadArtReference(t, room)
				page.MustEval(`(ref, variant)=>{
                    const s=GoldfishHotspotFixture.rooms[ref.room][variant], key=s.ending||s.routes.hall||'';
                    const overlay=document.createElement('div');overlay.dataset.goldfishAudit='';
                    Object.assign(overlay.style,{position:'absolute',inset:'0',pointerEvents:'none',zIndex:'10'});
                    const groups=ref.targets.concat(ref.exits,{id:'gap',points:ref.backgroundPoints});
                    groups.forEach((g,index)=>{for(const [x,y] of g.variants?.[key]||g.points){
                        const mark=document.createElement('span');mark.textContent='+'+g.id;
                        Object.assign(mark.style,{position:'absolute',left:x*100+'%',top:y*100+'%',transform:'translate(-4px,-7px)',font:'bold 11px monospace',color:['#ff705e','#ffe35e','#85ff71','#75eaff','#dfadff'][index%5],textShadow:'1px 1px 2px black,-1px -1px 2px black',whiteSpace:'nowrap'});
                        overlay.append(mark);
                    }});document.querySelector('.agfish-scene').append(overlay);
                }`, ref, variant)
				page.MustScreenshot(stem + "-points.png")
				page.MustEval(`()=>document.querySelector('[data-goldfish-audit]').remove()`)
				geometry := page.MustEval(`(room, variant) => ({geometry: GoldfishHotspotFixture.geometry(), state: GoldfishHotspotFixture.rooms[room][variant]})`, room, variant)
				var pretty any
				if err := json.Unmarshal([]byte(geometry.JSON("", "")), &pretty); err != nil {
					t.Fatal(err)
				}
				data, err := json.MarshalIndent(pretty, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(stem+".json", append(data, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if errors := page.MustEval(`() => fixtureErrors.join('\n')`).Str(); errors != "" {
		t.Fatal(errors)
	}
}
