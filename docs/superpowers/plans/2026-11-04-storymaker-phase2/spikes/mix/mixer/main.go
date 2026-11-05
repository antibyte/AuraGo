package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: prep|run|resid|gateprep")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "prep":
		check(prep(os.Args[2:]))
	case "run":
		check(run(os.Args[2:]))
	case "resid":
		check(resid(os.Args[2:]))
	case "gateprep":
		check(gateprep(os.Args[2:]))
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// ---------- peak working set (Windows) ----------
type procMem struct {
	cb, PageFaultCount                                 uint32
	PeakWorkingSetSize, WorkingSetSize                 uintptr
	QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage       uintptr
	QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage uintptr
	PagefileUsage, PeakPagefileUsage                   uintptr
}

func peakWS() float64 {
	h, _ := syscall.GetCurrentProcess()
	dll := syscall.NewLazyDLL("psapi.dll")
	pr := dll.NewProc("GetProcessMemoryInfo")
	var m procMem
	m.cb = uint32(unsafe.Sizeof(m))
	pr.Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.cb))
	return float64(m.PeakWorkingSetSize) / (1 << 20)
}

// ---------- prep: source pool ----------
func prep(args []string) error {
	fs := flag.NewFlagSet("prep", flag.ExitOnError)
	speech := fs.String("speech", "", "48k stereo s16 speech wav")
	pool := fs.String("pool", "", "pool dir")
	fs.Parse(args)
	os.MkdirAll(*pool, 0o755)
	sp, err := readAllS16(*speech)
	if err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(7))
	totalFrames := len(sp) / 2
	for k := 0; k < 24; k++ {
		n := int((0.8 + rng.Float64()*1.4) * rate)
		start := rng.Intn(totalFrames - n)
		w, err := newWav(filepath.Join(*pool, fmt.Sprintf("narr_%02d.wav", k)), false)
		if err != nil {
			return err
		}
		buf := make([]int16, n*2)
		for i := 0; i < n; i++ {
			g := 1.0
			if i < 480 {
				g = float64(i) / 480
			}
			if n-i < 480 {
				g = float64(n-i) / 480
			}
			buf[i*2] = int16(float64(sp[(start+i)*2]) * g)
			buf[i*2+1] = int16(float64(sp[(start+i)*2+1]) * g)
		}
		w.writeS16(buf)
		w.Close()
	}
	for k := 0; k < 12; k++ {
		n := int((0.2 + rng.Float64()) * rate)
		f0 := 60 + rng.Float64()*400
		buf := make([]int16, n*2)
		for i := 0; i < n; i++ {
			t := float64(i) / rate
			env := math.Exp(-t * (4 + rng.Float64()*0.0 + float64(k)*0.8))
			v := 0.55*env*math.Sin(2*math.Pi*f0*t*(1+0.5*math.Exp(-t*20))) + 0.25*env*math.Exp(-t*15)*(rng.Float64()*2-1)
			if i < 48 {
				v *= float64(i) / 48
			}
			s := int16(v * 32767)
			buf[i*2], buf[i*2+1] = s, s
		}
		w, err := newWav(filepath.Join(*pool, fmt.Sprintf("sfx_%02d.wav", k)), false)
		if err != nil {
			return err
		}
		w.writeS16(buf)
		w.Close()
	}
	return nil
}

// ---------- plan ----------
func buildPlan(dur float64, clips int, pool, bed string, seed int64) (*Plan, error) {
	rng := rand.New(rand.NewSource(seed))
	N := int64(math.Round(dur * rate))
	pl := &Plan{N: N, BedPath: bed}
	nNarr := int(math.Min(float64(clips)*0.4, dur/1.7))
	t := int64(rate)
	for i := 0; i < nNarr; i++ {
		path := filepath.Join(pool, fmt.Sprintf("narr_%02d.wav", rng.Intn(24)))
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		info, _ := parseWAV(f)
		f.Close()
		if t+info.frames > N-rate {
			break
		}
		pl.Narr = append(pl.Narr, Clip{Start: t, Path: path, Gain: 1})
		pl.Spans = append(pl.Spans, Span{t, t + info.frames})
		t += info.frames + int64((0.2+rng.Float64()*0.2)*rate)
	}
	// merge spans closer than 300 ms
	var merged []Span
	for _, s := range pl.Spans {
		if len(merged) > 0 && s.A-merged[len(merged)-1].B < int64(0.3*rate) {
			merged[len(merged)-1].B = s.B
		} else {
			merged = append(merged, s)
		}
	}
	pl.Spans = merged
	nSfx := clips - len(pl.Narr)
	for i := 0; i < nSfx; i++ {
		path := filepath.Join(pool, fmt.Sprintf("sfx_%02d.wav", rng.Intn(12)))
		start := int64((0.5 + rng.Float64()*(dur-1.0)) * rate)
		pl.Sfx = append(pl.Sfx, Clip{Start: start, Path: path, Gain: float32(0.5 + rng.Float64()*0.5)})
		if i%3 == 0 {
			pl.Impacts = append(pl.Impacts, Impact{At: start + 120*rate/1000, DepthDB: 2 + rng.Float64()*4})
		}
	}
	return pl, nil
}

// ---------- ffmpeg meter ----------
type meter struct{ I, TP, LRA float64 }

func measure(file string) (meter, error) {
	ff := os.Getenv("FFMPEG")
	cmd := exec.Command(ff, "-hide_banner", "-nostats", "-i", file, "-af", "loudnorm=I=-16:TP=-1:LRA=11:print_format=json", "-f", "null", "-")
	out, _ := cmd.CombinedOutput()
	s := string(out)
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

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dur := fs.Float64("dur", 180, "seconds")
	clips := fs.Int("clips", 1000, "clip count")
	dir := fs.String("dir", "", "out dir")
	pool := fs.String("pool", "", "pool dir")
	bedp := fs.String("bed", "", "bed wav")
	duck := fs.Float64("duck", 8, "duck depth dB")
	atk := fs.Float64("atk", 50, "ms")
	rel := fs.Float64("rel", 400, "ms")
	look := fs.Float64("look", 50, "lookahead ms")
	carveDB := fs.Float64("carve", -5, "dB")
	over := fs.Float64("overdrive", 0, "dB added to final master gain (clipping demo)")
	bedRel := fs.Float64("bedrel", -12, "bed reference relative to narration LU")
	fs.Parse(args)
	os.MkdirAll(*dir, 0o755)
	plan, err := buildPlan(*dur, *clips, *pool, *bedp, 1)
	if err != nil {
		return err
	}
	fmt.Printf("plan: N=%d samples, narr clips=%d, sfx clips=%d, impacts=%d, spans=%d\n", plan.N, len(plan.Narr), len(plan.Sfx), len(plan.Impacts), len(plan.Spans))
	base := Params{DuckDepthDB: *duck, DuckAttackMs: *atk, DuckReleaseMs: *rel, LookaheadMs: *look, CarveDB: *carveDB, CarveRampMs: 20, Dips: true, FadeInS: 1, FadeOutS: 2, LoopXfadeS: 2}

	// pass 1: narration stem + raw bed at unity
	t0 := time.Now()
	p1 := base
	p1.BedRaw = true
	f := func(n string) string { return filepath.Join(*dir, n) }
	if _, err := runMix(plan, p1, Outputs{Narr: f("p1_narr.wav"), Bed: f("p1_bedraw.wav")}); err != nil {
		return err
	}
	d1 := time.Since(t0)
	narrM, err := measure(f("p1_narr.wav"))
	if err != nil {
		return err
	}
	bedM, err := measure(f("p1_bedraw.wav"))
	if err != nil {
		return err
	}
	bedGain := narrM.I + *bedRel - bedM.I
	fmt.Printf("pass1 %.2fs  narr I=%.2f TP=%.2f  bedraw I=%.2f TP=%.2f -> bed gain %.2f dB\n", d1.Seconds(), narrM.I, narrM.TP, bedM.I, bedM.TP, bedGain)

	// pass 2: full mix at unity master, float
	t1 := time.Now()
	p2 := base
	p2.BedGainDB = bedGain
	st2, err := runMix(plan, p2, Outputs{Mix: f("p2_mix_f32.wav"), FloatMix: true})
	if err != nil {
		return err
	}
	d2 := time.Since(t1)
	mixM, err := measure(f("p2_mix_f32.wav"))
	if err != nil {
		return err
	}
	stemPeakDB := 20 * math.Log10(math.Max(st2.PeakNarr, math.Max(st2.PeakBed, st2.PeakSfx)))
	gLoud := -16 - mixM.I
	gTP := -1 - mixM.TP
	gStem := -0.1 - stemPeakDB
	G := math.Min(gLoud, math.Min(gTP, gStem))
	fmt.Printf("pass2 %.2fs  mix I=%.2f TP=%.2f LRA=%.2f samplepeak=%.2f dBFS  stempeak=%.2f dBFS\n", d2.Seconds(), mixM.I, mixM.TP, mixM.LRA, 20*math.Log10(st2.PeakMix), stemPeakDB)
	fmt.Printf("   gains: loudness %.2f  truepeak %.2f  stemguard %.2f -> G=%.2f dB\n", gLoud, gTP, gStem, G)
	fmt.Printf("   under speech: mean duck %.2f dB, narr-minus-bed %.2f dB (rms, ducked+carved bed)\n", st2.MeanDuckDBWhenSpeech, 10*math.Log10(st2.SpeechNarrSq/st2.SpeechBedSq))

	// pass 3: final
	t2 := time.Now()
	p3 := p2
	p3.MasterDB = G + *over
	st3, err := runMix(plan, p3, Outputs{Narr: f("narr.wav"), Bed: f("bed.wav"), Sfx: f("sfx.wav"), Mix: f("mix.wav")})
	if err != nil {
		return err
	}
	d3 := time.Since(t2)
	fmt.Printf("pass3 %.2fs  blocks=%d stemClamps=%d mixClamps=%d inlineNullMismatch=%d peakMix=%.2f dBFS maxActive=%d\n", d3.Seconds(), st3.Blocks, st3.StemClamps, st3.MixClamps, st3.NullMismatch, 20*math.Log10(st3.PeakMix), st3.MaxActiveClips)
	fmt.Printf("TOTAL mixer wall (3 passes, excl. ffmpeg meters) %.2fs, peak working set %.1f MiB\n", (d1 + d2 + d3).Seconds(), peakWS())
	nulls, maxErr, total := nullCheck(f("mix.wav"), f("narr.wav"), f("bed.wav"), f("sfx.wav"))
	fmt.Printf("NULL (read back from files): samples=%d nonzero=%d maxAbsErr=%d LSB\n", total, nulls, maxErr)
	fin, _ := measure(f("mix.wav"))
	fmt.Printf("final mix measured: I=%.2f TP=%.2f LRA=%.2f\n", fin.I, fin.TP, fin.LRA)
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

// ---------- resid: decoded mix vs sum of decoded stems ----------
func readFloatWav(path string) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := parseWAV(f)
	if err != nil {
		return nil, err
	}
	bps := int64(info.bits / 8)
	raw := make([]byte, info.frames*2*bps)
	f.ReadAt(raw, info.off)
	out := make([]float32, info.frames*2)
	for i := range out {
		if info.bits == 16 {
			out[i] = float32(int16(binary.LittleEndian.Uint16(raw[i*2:]))) / 32768
		} else {
			out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		}
	}
	return out, nil
}

func resid(args []string) error {
	var d [4][]float32
	for i := 0; i < 4; i++ {
		x, err := readFloatWav(args[i])
		if err != nil {
			return err
		}
		d[i] = x
	}
	n := len(d[0])
	for i := 1; i < 4; i++ {
		if len(d[i]) < n {
			n = len(d[i])
		}
	}
	fmt.Printf("lengths (frames): mix=%d narr=%d bed=%d sfx=%d\n", len(d[0])/2, len(d[1])/2, len(d[2])/2, len(d[3])/2)
	// optional lag between mix and stem-sum searched in [-lagMax, lagMax]
	var se, sm float64
	var pe, pm float64
	for i := 0; i < n; i++ {
		m := float64(d[0][i])
		e := m - float64(d[1][i]) - float64(d[2][i]) - float64(d[3][i])
		se += e * e
		sm += m * m
		pe = math.Max(pe, math.Abs(e))
		pm = math.Max(pm, math.Abs(m))
	}
	db := func(v float64) float64 { return 10 * math.Log10(v) }
	fmt.Printf("decoded mix: rms %.2f dBFS, peak %.2f dBFS\n", db(sm/float64(n)), 20*math.Log10(pm))
	fmt.Printf("residual mix-(n+b+s): rms %.2f dBFS (%.2f dB below mix rms), peak %.2f dBFS\n", db(se/float64(n)), db(sm/se), 20*math.Log10(pe))
	return nil
}

// ---------- gate prep ----------
func gateprep(args []string) error {
	fs := flag.NewFlagSet("gateprep", flag.ExitOnError)
	frames := fs.Int("frames", 91, "")
	fps := fs.Int("fps", 30, "")
	dir := fs.String("dir", "", "")
	fs.Parse(args)
	os.MkdirAll(*dir, 0o755)
	vf, err := os.Create(filepath.Join(*dir, fmt.Sprintf("frames_%d_%d.bin", *frames, *fps)))
	if err != nil {
		return err
	}
	for i := 0; i < *frames; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 320, 180))
		for y := 0; y < 180; y++ {
			for x := 0; x < 320; x++ {
				img.Set(x, y, color.RGBA{uint8((x + i*4) % 256), uint8(y), uint8(i * 255 / *frames), 255})
			}
		}
		if err := png.Encode(vf, img); err != nil {
			return err
		}
	}
	vf.Close()
	samples := int(math.Round(float64(*frames) * rate / float64(*fps)))
	fmt.Printf("frames=%d fps=%d expected samples=%d (%.6f s)\n", *frames, *fps, samples, float64(samples)/rate)
	for _, v := range []struct {
		name  string
		delta int
	}{{"exact", 0}, {"short", -rate}, {"long", rate}} {
		n := samples + v.delta
		w, err := newWav(filepath.Join(*dir, fmt.Sprintf("audio_%s_%d_%d.wav", v.name, *frames, *fps)), false)
		if err != nil {
			return err
		}
		buf := make([]int16, n*2)
		for i := 0; i < n; i++ {
			s := int16(8000 * math.Sin(2*math.Pi*440*float64(i)/rate))
			buf[i*2], buf[i*2+1] = s, s
		}
		w.writeS16(buf)
		w.Close()
		fmt.Printf("  %s: %d samples\n", v.name, n)
	}
	return nil
}
