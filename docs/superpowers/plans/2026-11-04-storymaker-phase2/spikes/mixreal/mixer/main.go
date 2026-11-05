package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: film")
		os.Exit(2)
	}
	check(film(os.Args[2:]))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// ---------- memory ----------
type procMem struct {
	cb, PageFaultCount                                 uint32
	PeakWorkingSetSize, WorkingSetSize                 uintptr
	QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage       uintptr
	QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage uintptr
	PagefileUsage, PeakPagefileUsage                   uintptr
}

func peakOf(h syscall.Handle) float64 {
	dll := syscall.NewLazyDLL("psapi.dll")
	pr := dll.NewProc("GetProcessMemoryInfo")
	var m procMem
	m.cb = uint32(unsafe.Sizeof(m))
	pr.Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.cb))
	return float64(m.PeakWorkingSetSize) / (1 << 20)
}

func peakWS() float64 { h, _ := syscall.GetCurrentProcess(); return peakOf(h) }

// ---------- meter ----------
type meter struct{ I, TP, LRA float64 }

func parseLN(s string) (meter, error) {
	i := strings.LastIndex(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return meter{}, fmt.Errorf("no json: %s", s)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s[i:j+1]), &m); err != nil {
		return meter{}, err
	}
	pf := func(k string) float64 { v, _ := strconv.ParseFloat(m[k], 64); return v }
	return meter{pf("input_i"), pf("input_tp"), pf("input_lra")}, nil
}

func measure(file string) (meter, error) {
	cmd := exec.Command(os.Getenv("FFMPEG"), "-hide_banner", "-nostats", "-i", file, "-af", "loudnorm=I=-16:TP=-1:LRA=11:print_format=json", "-f", "null", "-")
	out, _ := cmd.CombinedOutput()
	return parseLN(string(out))
}

// ---------- segmenting narration ----------
type seg struct{ off, n int64 }
type source struct {
	path string
	segs []seg
	next int
}

func loadSource(name string) (*source, error) {
	p := filepath.Join(mediaDir, "narr", name+".wav")
	d, err := readAllS16(p)
	if err != nil {
		return nil, err
	}
	frames := len(d) / 2
	const w = 480
	nw := frames / w
	silent := make([]bool, nw)
	for k := 0; k < nw; k++ {
		var s float64
		for i := 0; i < w; i++ {
			v := float64(d[(k*w+i)*2]) / 32768
			s += v * v
		}
		silent[k] = 10*math.Log10(s/w+1e-12) < -48
	}
	// separators: silent runs >= 250 ms
	var raw []seg
	start := -1
	k := 0
	for k < nw {
		if !silent[k] {
			if start < 0 {
				start = k
			}
			k++
			continue
		}
		j := k
		for j < nw && silent[j] {
			j++
		}
		if j-k >= 25 && start >= 0 {
			raw = append(raw, seg{int64(start * w), int64((k - start) * w)})
			start = -1
		}
		k = j
	}
	if start >= 0 {
		raw = append(raw, seg{int64(start * w), int64((nw - start) * w)})
	}
	// merge until >= 2.5 s, then pad 30 ms each side
	var segs []seg
	var cur *seg
	for _, r := range raw {
		if cur == nil {
			c := r
			cur = &c
		} else {
			cur.n = r.off + r.n - cur.off
		}
		if cur.n >= 2.5*rate {
			segs = append(segs, *cur)
			cur = nil
		}
	}
	if cur != nil && cur.n > 0.6*rate {
		segs = append(segs, *cur)
	}
	for i := range segs {
		pad := int64(0.03 * rate)
		o := segs[i].off - pad
		if o < 0 {
			o = 0
		}
		e := segs[i].off + segs[i].n + pad
		if e > int64(frames) {
			e = int64(frames)
		}
		segs[i] = seg{o, e - o}
	}
	return &source{path: p, segs: segs}, nil
}

func (s *source) take() (seg, string) {
	g := s.segs[s.next%len(s.segs)]
	s.next++
	return g, s.path
}

// ---------- sheet ----------
type take struct {
	File     string  `json:"file"`
	ImpactMs float64 `json:"impact_ms"`
	L50      float64 `json:"l50_lufs"`
	DurMs    float64 `json:"dur_ms"`
}

var mediaDir string

func loadSheet(p string) (map[string][]take, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var raw map[string]struct {
		Takes []take `json:"takes"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := map[string][]take{}
	for k, v := range raw {
		out[k] = v.Takes
	}
	return out, nil
}

var overDB = map[string]float64{"pop": 4.5, "ding": 4.5, "success": 4.5, "error": 4.5, "whoosh": 0, "click": 0, "typing": 0}
var dipKinds = map[string]bool{"pop": true, "ding": true, "success": true, "error": true}

type sfxEv struct {
	kind string
	tk   take
	at   int64
}

// ---------- pipe metering ----------
type pipeRun struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	errb bytes.Buffer
	peak float64
	stop chan struct{}
	wg   sync.WaitGroup
	t0   time.Time
	s16  bool
	buf  []byte
}

func startPipe(spec string, s16 bool) (*pipeRun, error) {
	parts := strings.Split(spec, "|")
	pr := &pipeRun{s16: s16, stop: make(chan struct{}), t0: time.Now()}
	pr.cmd = exec.Command(parts[0], parts[1:]...)
	pr.cmd.Stderr = &pr.errb
	in, err := pr.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	pr.in = in
	if err := pr.cmd.Start(); err != nil {
		return nil, err
	}
	h, err := syscall.OpenProcess(0x1000, false, uint32(pr.cmd.Process.Pid))
	if err == nil {
		pr.wg.Add(1)
		go func() {
			defer pr.wg.Done()
			for {
				if v := peakOf(h); v > pr.peak {
					pr.peak = v
				}
				select {
				case <-pr.stop:
					return
				case <-time.After(50 * time.Millisecond):
				}
			}
		}()
	}
	return pr, nil
}

func (p *pipeRun) write(mixF []float32) error {
	bs := 4
	if p.s16 {
		bs = 2
	}
	if cap(p.buf) < len(mixF)*bs {
		p.buf = make([]byte, len(mixF)*bs)
	}
	b := p.buf[:len(mixF)*bs]
	for i, v := range mixF {
		if p.s16 {
			r := math.Round(float64(v) * 0.25 * 32768) // -12 dB
			if r > 32767 {
				r = 32767
			} else if r < -32768 {
				r = -32768
			}
			binary.LittleEndian.PutUint16(b[i*2:], uint16(int16(r)))
		} else {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
		}
	}
	_, err := p.in.Write(b)
	return err
}

func (p *pipeRun) finish() (meter, time.Duration, string) {
	p.in.Close()
	p.cmd.Wait()
	d := time.Since(p.t0)
	close(p.stop)
	p.wg.Wait()
	m, _ := parseLN(p.errb.String())
	return m, d, p.errb.String()
}

// ---------- film ----------
func film(args []string) error {
	fs := flag.NewFlagSet("film", flag.ExitOnError)
	shape := fs.String("shape", "A", "label")
	dur := fs.Float64("dur", 120, "seconds")
	music := fs.String("music", "", "music name in media/music (empty = silent)")
	vol := fs.Float64("vol", 0.25, "music volume")
	narrA := fs.String("narrA", "", "scene type A sources, comma separated")
	narrB := fs.String("narrB", "", "scene type B sources (alternating scenes)")
	sfxMode := fs.String("sfx", "many", "many|one|none")
	seed := fs.Int64("seed", 1, "seed")
	root := fs.String("root", "", "mixreal dir")
	dir := fs.String("dir", "", "out dir")
	sheetP := fs.String("sheet", "", "sheet.json")
	pipeSpec := fs.String("pipe", "", "exe|arg|arg: stream pass-2 mix into this command")
	pipeS16 := fs.Bool("pipes16", false, "stream s16 at -12 dB instead of f32")
	gainB := fs.Float64("gainB", 0, "dB gain on scene type B narration clips")
	keepStems := fs.Bool("keep", true, "write stems in pass 3")
	fs.Parse(args)
	mediaDir = filepath.Join(*root, "media")
	os.MkdirAll(*dir, 0o755)
	f := func(n string) string { return filepath.Join(*dir, n) }
	rng := rand.New(rand.NewSource(*seed))
	N := int64(math.Round(*dur * rate))
	plan := &Plan{N: N}
	musicName := *music
	hasMusic := musicName != ""
	if !hasMusic {
		musicName = "silent"
	}
	plan.BedPath = filepath.Join(mediaDir, "music", musicName+".wav")

	// narration
	var A, B []*source
	for _, n := range strings.Split(*narrA, ",") {
		if n != "" {
			s, err := loadSource(n)
			if err != nil {
				return err
			}
			A = append(A, s)
		}
	}
	for _, n := range strings.Split(*narrB, ",") {
		if n != "" {
			s, err := loadSource(n)
			if err != nil {
				return err
			}
			B = append(B, s)
		}
	}
	hasNarr := len(A) > 0
	if hasNarr {
		t := int64(1.0 * rate)
		if *dur < 20 {
			t = int64(0.8 * rate)
		}
		endLimit := N - int64(2.2*rate)
		sc := 0
	scenes:
		for t < endLimit {
			set := A
			if len(B) > 0 && sc%2 == 1 {
				set = B
			}
			cg := float32(1)
			if len(B) > 0 && sc%2 == 1 {
				cg = float32(dbToLin(*gainB))
			}
			nsent := 2 + rng.Intn(3)
			for k := 0; k < nsent; k++ {
				g, p := set[k%len(set)].take()
				if t+g.n > endLimit {
					break scenes
				}
				plan.Narr = append(plan.Narr, Clip{Start: t, Path: p, Gain: cg, Off: g.off, Len: g.n})
				plan.Spans = append(plan.Spans, Span{t, t + g.n})
				t += g.n + int64((0.12+rng.Float64()*0.2)*rate)
			}
			t += int64((0.4 + 0.3 + rng.Float64()*1.6) * rate) // 0.4 s tail + scene gap
			sc++
		}
		var merged []Span
		for _, s := range plan.Spans {
			if len(merged) > 0 && s.A-merged[len(merged)-1].B < int64(0.3*rate) {
				merged[len(merged)-1].B = s.B
			} else {
				merged = append(merged, s)
			}
		}
		plan.Spans = merged
	}

	// sfx events
	sheet, err := loadSheet(*sheetP)
	if err != nil {
		return err
	}
	kinds := []string{"whoosh", "pop", "click", "ding", "typing", "success", "error"}
	var evs []sfxEv
	switch *sfxMode {
	case "many":
		t := 2.0 + rng.Float64()*2
		for t < *dur-2.5 {
			k := kinds[rng.Intn(len(kinds))]
			tk := sheet[k][rng.Intn(len(sheet[k]))]
			at := int64(t * rate)
			if at+int64(tk.DurMs*rate/1000) < N {
				evs = append(evs, sfxEv{k, tk, at})
			}
			t += 4 + rng.Float64()*4
		}
	case "one":
		tk := sheet["pop"][0]
		evs = append(evs, sfxEv{"pop", tk, int64(5.0 * rate)})
	}

	base := Params{DuckDepthDB: 8, DuckAttackMs: 50, DuckReleaseMs: 400, LookaheadMs: 50, CarveDB: -5, CarveRampMs: 20, Dips: true, FadeInS: 1, FadeOutS: 2, LoopXfadeS: 2}
	fmt.Printf("== shape %s: dur=%.0fs music=%q vol=%.2f narr clips=%d sfx hits=%d\n", *shape, *dur, *music, *vol, len(plan.Narr), len(evs))

	// pass 1
	var narrM, bedM meter
	narrM.I, bedM.I = math.NaN(), math.NaN()
	p1 := base
	p1.BedRaw = true
	if _, err := runMix(plan, p1, Outputs{Narr: f("p1_narr.wav"), Bed: f("p1_bedraw.wav")}); err != nil {
		return err
	}
	if hasNarr {
		if narrM, err = measure(f("p1_narr.wav")); err != nil {
			return err
		}
	}
	if hasMusic {
		if bedM, err = measure(f("p1_bedraw.wav")); err != nil {
			return err
		}
	}
	volDB := 20 * math.Log10(*vol/0.25)
	var refBed float64
	if hasNarr {
		refBed = narrM.I - 12 + volDB
	} else {
		refBed = -18 + volDB
	}
	bedGain := 0.0
	if hasMusic {
		bedGain = refBed - bedM.I
	}
	// sfx reference
	sfxRef := refBed
	if !hasMusic {
		if hasNarr {
			sfxRef = narrM.I - 12
		} else {
			sfxRef = -18
		}
	}
	gmin, gmax := 1e9, -1e9
	for _, e := range evs {
		g := sfxRef + overDB[e.kind] - e.tk.L50
		gmin, gmax = math.Min(gmin, g), math.Max(gmax, g)
		plan.Sfx = append(plan.Sfx, Clip{Start: e.at, Path: filepath.Join(mediaDir, "sfx", e.tk.File), Gain: float32(dbToLin(g))})
		if dipKinds[e.kind] {
			plan.Impacts = append(plan.Impacts, Impact{At: e.at + int64(e.tk.ImpactMs*rate/1000), DepthDB: 3})
		}
	}
	fmt.Printf("narration I=%.2f TP=%.2f | bed raw I=%.2f -> bed ref I=%.2f gain=%+.2f dB | sfx ref=%.2f gains %+.1f..%+.1f dB dips=%d\n", narrM.I, narrM.TP, bedM.I, refBed, bedGain, sfxRef, gmin, gmax, len(plan.Impacts))

	// pass 2
	p2 := base
	p2.BedGainDB = bedGain
	var pr *pipeRun
	if *pipeSpec != "" {
		if pr, err = startPipe(*pipeSpec, *pipeS16); err != nil {
			return err
		}
		p2.Tee = pr.write
	}
	t1 := time.Now()
	st2, err := runMix(plan, p2, Outputs{Mix: f("p2_mix_f32.wav"), FloatMix: true})
	if err != nil {
		return err
	}
	d2 := time.Since(t1)
	if pr != nil {
		pm, pd, errs := pr.finish()
		off := 0.0
		if *pipeS16 {
			off = 12.0412
		}
		fmt.Printf("PIPE: I=%.2f TP=%.2f LRA=%.2f (s16 offset +%.2f applied: I=%.2f TP=%.2f) wall(stream+meter)=%.1fs peak RSS of piped process=%.0f MiB\n", pm.I, pm.TP, pm.LRA, off, pm.I+off, pm.TP+off, pd.Seconds(), pr.peak)
		if i := strings.Index(errs, "Maximum resident"); i >= 0 {
			fmt.Println("  container time -v:", strings.TrimSpace(errs[i:min(len(errs), i+60)]))
		}
		if i := strings.Index(errs, "Elapsed (wall"); i >= 0 {
			fmt.Println("  container time -v:", strings.TrimSpace(errs[i:min(len(errs), i+80)]))
		}
		if i := strings.Index(errs, "ffmpeg version"); i >= 0 {
			fmt.Println("  ", strings.SplitN(errs[i:], "\n", 2)[0])
		}
	}
	t3 := time.Now()
	mixM, err := measure(f("p2_mix_f32.wav"))
	if err != nil {
		return err
	}
	fmt.Printf("FILE-meter of p2 float mix: I=%.2f TP=%.2f LRA=%.2f (meter wall %.1fs); pass2 mixer wall %.1fs\n", mixM.I, mixM.TP, mixM.LRA, time.Since(t3).Seconds(), d2.Seconds())
	stemPeak := math.Max(st2.PeakNarr, math.Max(st2.PeakBed, st2.PeakSfx))
	stemPeakDB := 20 * math.Log10(stemPeak)
	gLoud := -16 - mixM.I
	gTP := -1 - mixM.TP
	gStem := -0.1 - stemPeakDB
	G := math.Min(gLoud, math.Min(gTP, gStem))
	bound := "loudness"
	if G == gTP {
		bound = "truepeak"
	}
	if G == gStem {
		bound = "stemguard"
	}
	if G == gLoud {
		bound = "loudness"
	}
	fmt.Printf("pre-gain mix I=%.2f TP=%.2f | gains: loud %+.2f tp %+.2f stem %+.2f -> G=%+.2f dB BOUND BY %s\n", mixM.I, mixM.TP, gLoud, gTP, gStem, G, bound)
	if st2.SpeechFrames > 0 && st2.SpeechBedSq > 0 {
		fmt.Printf("under speech: mean duck %.2f dB, narr-minus-bed %.2f dB (rms)\n", st2.MeanDuckDBWhenSpeech, 10*math.Log10(st2.SpeechNarrSq/st2.SpeechBedSq))
	} else {
		fmt.Printf("under speech: n/a (speech frames %d)\n", st2.SpeechFrames)
	}

	// pass 3
	p3 := p2
	p3.Tee = nil
	p3.MasterDB = G
	o3 := Outputs{Mix: f("mix.wav")}
	if *keepStems {
		o3.Narr, o3.Bed, o3.Sfx = f("narr.wav"), f("bed.wav"), f("sfx.wav")
	}
	st3, err := runMix(plan, p3, o3)
	if err != nil {
		return err
	}
	fmt.Printf("pass3: stemClamps=%d mixClamps=%d inlineNullMismatch=%d peakMix=%.2f dBFS\n", st3.StemClamps, st3.MixClamps, st3.NullMismatch, 20*math.Log10(st3.PeakMix))
	if *keepStems {
		nz, me, tot := nullCheck(f("mix.wav"), f("narr.wav"), f("bed.wav"), f("sfx.wav"))
		fmt.Printf("NULL (files): samples=%d nonzero=%d maxAbsErr=%d LSB\n", tot, nz, me)
	}
	fin, _ := measure(f("mix.wav"))
	fmt.Printf("FINAL (loudnorm): I=%.2f TP=%.2f LRA=%.2f  -> delta from -16: %+.2f LU\n", fin.I, fin.TP, fin.LRA, fin.I+16)
	fmt.Printf("mixer peak working set %.0f MiB\n", peakWS())
	return nil
}

func nullCheck(mix, n, b, s string) (nonzero int64, maxErr int64, total int64) {
	files := make([]*os.File, 4)
	var info [4]wavInfo
	for i, p := range []string{mix, n, b, s} {
		f, err := os.Open(p)
		check(err)
		defer f.Close()
		files[i] = f
		info[i], err = parseWAV(f)
		check(err)
	}
	const chunk = 1 << 16
	bufs := [4][]byte{}
	for i := range bufs {
		bufs[i] = make([]byte, chunk*4)
	}
	for pos := int64(0); pos < info[0].frames; pos += chunk {
		cnt := int64(chunk)
		if pos+cnt > info[0].frames {
			cnt = info[0].frames - pos
		}
		for i := 0; i < 4; i++ {
			files[i].ReadAt(bufs[i][:cnt*4], info[i].off+pos*4)
		}
		for k := int64(0); k < cnt*2; k++ {
			var v [4]int64
			for i := 0; i < 4; i++ {
				v[i] = int64(int16(binary.LittleEndian.Uint16(bufs[i][k*2:])))
			}
			d := v[0] - (v[1] + v[2] + v[3])
			if d != 0 {
				nonzero++
				if d < 0 {
					d = -d
				}
				if d > maxErr {
					maxErr = d
				}
			}
			total++
		}
	}
	return
}

var _ = sort.Ints
