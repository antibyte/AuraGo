package videostudio

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"testing"
	"time"
)

// blurChart is a detailed picture for the blur_in tests: colour ramps, a fine diagonal pattern and a checker, so both
// blur nodes change it. With alpha, a soft disc gives the premultiplied recipe edges to work on, as in the spike (S8):
// alpha 255 inside, falling through 60 at the rim, 0 outside.
func blurChart(width, height int, alpha bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := color.NRGBA{R: uint8((x*37 + y*11) % 256), G: uint8((x*x/7 + y*29) % 256), B: uint8(20 + 200*((x/4+y/4)%2)), A: 255}
			if alpha {
				d := math.Hypot(float64(x-width/2), float64(y-height/2))
				c.A = uint8(min(max(60+(float64(height)/3-d)*20, 0), 255))
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// animMM is maskedmerge's 8-bit merge (spike S2): floor((b·(255 − m) + o·m + 127)/255). m = 0 gives b, and 255 gives o.
func animMM(b, o, m int) int { return (b*(255-m) + o*m + 127) / 255 }

// The blur_in export (index §4.24), on a 160x90 picture (S = 1.8, S/4 = 0.45) with blur_in in over 10 frames and out over 6
// of 20. Two cases: opaque video (the video path on a looped still, as keyChain's video case) and a still with alpha.
//  1. Each mask branch rendered alone gives m of animMaskLevel on every pixel of every window frame: all three planes of
//     a colour mask, and the alpha plane of an alpha mask.
//  2. The cascade (animBlurGraph, before an alpha layer's closing unpremultiply) equals
//     MM(MM(g(S), g(S/4), m1), sharp, m2) of this FFmpeg's own static dumps inside the spans. Colour is exact, and the
//     alpha plane is within 1 (spike S8). Outside the spans the cascade is the straight picture exactly.
//  3. Renders with 1, 7 and 16 filter threads are identical.
//  4. A window that starts at frame 5 renders the same frames, because masks and enable come from absolute frames.
//  5. The whole clip chain at rest equals the clip without blur_in. The opaque case is exact; the still's alpha is within
//     1 below FFmpeg 6 (the cascade's rgba → gbrap step, index §4.23).
func TestRenderAnimBlurMatchesCascade(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	root := t.TempDir()
	rate := Rate{Num: 25, Den: 1}
	const width, height, frames = 160, 90, 20
	major := ffmpegMajor(t, ffmpeg)
	type maskOutput struct {
		part, format string
		planes, from int // the output's planes; the first plane whose bytes all equal m
	}
	for _, tc := range []struct {
		name  string
		still bool
	}{{"video, opaque recipe", false}, {"still with alpha, premultiplied recipe", true}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, fmt.Sprintf("blur-%t.png", tc.still))
			writePicturePNG(t, path, blurChart(width, height, tc.still))
			clip := Clip{ID: "b", AssetID: "b", Duration: frames, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
				AnimIn: &ClipAnim{Preset: "blur_in", Duration: 10}, AnimOut: &ClipAnim{Preset: "blur_in", Duration: 6}}
			// D_in = 10: frames [0, 10); D_out = 6: frames [14, 20); frames 10–13 are at rest.
			chainAt := func(c Clip, at int) clipChain {
				local := c
				local.Duration = c.Duration - at
				return clipChain{item: layerClip{clip: local, whole: c, at: at, image: tc.still, pictureWidth: width, pictureHeight: height},
					canvasWidth: width, canvasHeight: height, rate: rate}
			}
			chain := chainAt(clip, 0)
			if animBlurAlpha(chain) != tc.still {
				t.Fatalf("animBlurAlpha = %v, want %v", animBlurAlpha(chain), tc.still)
			}
			plane := width * height
			planes, format := 3, "gbrp"
			if tc.still {
				planes, format = 4, "gbrap"
			}
			size := planes * plane

			// 1. The masks.
			outputs := []maskOutput{{"m", "gbrp", 3, 0}}
			if tc.still {
				outputs = []maskOutput{{"c", "gbrp", 3, 0}, {"a", "gbrap", 4, 3}}
			}
			for n := 1; n <= 2; n++ {
				filter, file := animMaskBranch(chain, n, tc.still)
				for _, o := range outputs {
					graph := filter
					for _, other := range outputs {
						if other.part != o.part {
							graph += ";[" + animLabel(chain, fmt.Sprintf("%s%d", other.part, n)) + "]nullsink"
						}
					}
					graph += ";[" + animLabel(chain, fmt.Sprintf("%s%d", o.part, n)) + "]format=" + o.format + "[vs_video_out]"
					for k, mask := range renderAnimGraph(t, ctx, ffmpeg, graph, []sendCmdFile{file}, rate, nil, frames, o.planes*plane, o.format) {
						level := animMaskLevel(clip, k, n)
						for i := o.from * plane; i < o.planes*plane; i++ {
							// Deviation from the plan (reported with Task 8): the alpha mask passes alphamerge's rgba and a
							// neighbour scale before format=gbrap, and that rgba → gbrap step adds 1 to alpha 128..254
							// (measured 0..255 on 8.1.1; the cascade's alpha tolerance of 1 covers it). Every other
							// mask byte, and all colour mask planes, equal m exactly.
							high := o.part == "a" && level >= 128 && level <= 254
							if d := int(mask[i]) - level; d != 0 && !(high && d == 1) {
								t.Fatalf("mask %s%d, frame %d: byte %d is %d, want m = %d (alpha mask: m or m+1 for m in 128..254)", o.part, n, k, i, mask[i], level)
							}
						}
					}
				}
			}

			// 2. The cascade against MM of the static nodes.
			dump := func(filters string) []byte {
				return renderAnimGraph(t, ctx, ffmpeg, "[0:V:0]"+filters+"[vs_video_out]", nil, rate, []string{path}, 1, size, format)[0]
			}
			sigma := blurSigma(animBlurRadius, height)
			lead := "format=gbrp"
			if tc.still {
				lead = "format=gbrap,premultiply=inplace=1"
			}
			sharp := dump(lead)
			g1 := dump(lead + ",gblur=sigma=" + effectNum(sigma) + ":steps=4")
			g2 := dump(lead + ",gblur=sigma=" + effectNum(sigma/4) + ":steps=4")
			straight := sharp
			if tc.still {
				straight = dump("format=gbrap")
			}
			renderCascade := func(c clipChain, extra ...string) [][]byte {
				cascade, _, files := animBlurGraph(c)
				graph := "[0:V:0]" + c.retime() + "," + c.window() + "," + cascade + ",format=" + format + "[vs_video_out]"
				return renderAnimGraph(t, ctx, ffmpeg, graph, files, rate, []string{path}, c.item.clip.Duration, size, format, extra...)
			}
			got := renderCascade(chain)
			worstAlpha, blurred := 0, 0
			for k, out := range got {
				a := animAt(clip, k)
				for i := range size {
					want, tolerance := int(straight[i]), 0
					if a.Blur {
						want = animMM(animMM(int(g1[i]), int(g2[i]), a.M1), int(sharp[i]), a.M2)
						if i >= 3*plane {
							tolerance = 1 // the alpha plane of the premultiplied recipe (spike S8)
						}
					}
					d := absInt(int(out[i]) - want)
					if d > tolerance {
						t.Fatalf("frame %d (m1 %d, m2 %d): byte %d (plane %d) is %d, the cascade model %d (tolerance %d)", k, a.M1, a.M2, i, i/plane, out[i], want, tolerance)
					}
					if i >= 3*plane {
						worstAlpha = max(worstAlpha, d)
					}
				}
				if a.Blur && !bytes.Equal(out, sharp) {
					blurred++
				}
			}
			if blurred == 0 {
				t.Fatal("no frame inside the spans differs from the sharp picture; the case does not test the blur")
			}
			t.Logf("%d blurred frames match MM(MM(g(S), g(S/4), m1), sharp, m2); largest alpha difference %d", blurred, worstAlpha)

			// 3. Filter threads.
			for _, threads := range [][]string{{"-filter_threads", "1", "-filter_complex_threads", "1"}, {"-filter_threads", "7", "-filter_complex_threads", "7"}, {"-filter_threads", "16", "-filter_complex_threads", "16"}} {
				for k, out := range renderCascade(chain, threads...) {
					if !bytes.Equal(out, got[k]) {
						t.Fatalf("frame %d differs with %v", k, threads)
					}
				}
			}

			// 4. A window that starts at frame 5.
			for i, out := range renderCascade(chainAt(clip, 5)) {
				if !bytes.Equal(out, got[5+i]) {
					t.Fatalf("frame %d differs when its window starts at frame 5", 5+i)
				}
			}

			// 5. The whole clip at rest.
			plain := clip
			plain.AnimIn, plain.AnimOut = nil, nil
			layers := renderClipFramesRGBA(t, ctx, ffmpeg, chain, path, frames)
			references := renderClipFramesRGBA(t, ctx, ffmpeg, chainAt(plain, 0), path, frames)
			tolerance := 0
			if tc.still && major > 0 && major < 6 {
				tolerance = 1
			}
			rest := 0
			for k := range frames {
				if animAt(clip, k).Blur {
					continue
				}
				rest++
				checkWipeLayer(t, fmt.Sprintf("frame %d at rest", k), layers[k], references[k], width, height, 0, width, false, tolerance)
			}
			if rest == 0 {
				t.Fatal("no frame at rest")
			}
		})
	}
}

// The gate for the timeline enable of maskedmerge, premultiply and unpremultiply (Task 8, plan decision 11). Each filter
// runs alone on a constant picture, with animSpanExpr's enable, so the pixels show exactly the frames on which it ran:
//   - maskedmerge of 10 towards 200 under a full mask: 200 on the frames it runs, 10 on the others;
//   - premultiply of colour 200 with alpha 128: about 100 when it runs, 200 otherwise;
//   - unpremultiply behind an always-on premultiply of colour 200 with alpha 128: about 199 when it runs, about 100
//     otherwise. (FFmpeg 8 tracks the frame's alpha mode: a frame that is not premultiplied passes unpremultiply.)
//
// The cascade test cannot see an early switch-on or a late switch-off (the masks are 255 there, so the pixels do not
// change); this test sees every frame. Rates 25 and 30000/1001, windows starting at the clip, inside the in span and
// inside the out span, filter threads default, 1 and 16. A frame that switches early or late fails the test; the fallback
// for maskedmerge is animMergeGated = false (Step 4). premultiply and unpremultiply have no fallback.
func TestRenderAnimBlurEnableGate(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	clip := Clip{ID: "g", AssetID: "g", Duration: 24, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		AnimIn: &ClipAnim{Preset: "blur_in", Duration: 7}, AnimOut: &ClipAnim{Preset: "blur_in", Duration: 6}}
	// D_in = 7: frames [0, 7); D_out = 6: frames [18, 24).
	const size = 8 * 8
	for _, tc := range []struct {
		name, format string
		planes       int
		graph        func(source, enable string) string
		// ran reports the frame's plane-0 value as "the filter ran" (true) or "it passed the picture" (false), or fails.
		ran func(value byte) (bool, bool)
	}{
		{"maskedmerge", "gbrp", 3, func(source, enable string) string {
			return fmt.Sprintf("color=c=0x0a0a0a:%[1]s,format=gbrp[vs_a];color=c=0xc8c8c8:%[1]s,format=gbrp[vs_b];color=c=white:%[1]s,format=gbrp[vs_m];[vs_a][vs_b][vs_m]maskedmerge=enable='%[2]s'[vs_video_out]", source, enable)
		}, func(v byte) (bool, bool) { return v == 200, v == 200 || v == 10 }},
		{"premultiply", "gbrap", 4, func(source, enable string) string {
			return fmt.Sprintf("color=c=0xc8c8c8@0.5:%s,format=gbrap,premultiply=inplace=1:enable='%s'[vs_video_out]", source, enable)
		}, func(v byte) (bool, bool) { return v < 150, v < 150 || v == 200 }},
		{"unpremultiply", "gbrap", 4, func(source, enable string) string {
			return fmt.Sprintf("color=c=0xc8c8c8@0.5:%s,format=gbrap,premultiply=inplace=1,unpremultiply=inplace=1:enable='%s'[vs_video_out]", source, enable)
		}, func(v byte) (bool, bool) { return v > 150, v > 150 || (v >= 99 && v <= 101) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "maskedmerge" && !animMergeGated {
				t.Skip("animMergeGated is false: the merges run on every frame (Task 8 gate fallback)")
			}
			ranFrames := 0
			for _, rate := range []Rate{{Num: 25, Den: 1}, {Num: 30000, Den: 1001}} {
				for _, at := range []int{0, 5, 19} {
					frames := clip.Duration - at
					enable := animSpanExpr(clip, at, rate, "blur_in").String()
					source := fmt.Sprintf("s=8x8:r=%s:d=%s", rate, rate.SecondsArg(frames))
					cut := fmt.Sprintf(",trim=end_frame=%d,setpts=PTS-STARTPTS", frames)
					graph := tc.graph(source+cut, enable)
					for _, threads := range [][]string{nil, {"-filter_threads", "1", "-filter_complex_threads", "1"}, {"-filter_threads", "16", "-filter_complex_threads", "16"}} {
						out := renderAnimGraph(t, ctx, ffmpeg, graph, nil, rate, nil, frames, tc.planes*size, tc.format, threads...)
						for k, frame := range out {
							want := animAt(clip, at+k).Blur
							got, ok := tc.ran(frame[0])
							if !ok || got != want {
								t.Fatalf("rate %s, window at %d, threads %v: frame %d has plane-0 value %d, which means ran = %v (valid %v), want ran = %v (enable %s)", rate, at, threads, at+k, frame[0], got, ok, want, enable)
							}
							for _, v := range frame[:size] {
								if v != frame[0] {
									t.Fatalf("rate %s, window at %d: frame %d is not uniform (%d and %d)", rate, at, at+k, v, frame[0])
								}
							}
							if got {
								ranFrames++
							}
						}
					}
				}
			}
			if ranFrames == 0 {
				t.Fatal("the filter never ran; the case does not test the enable")
			}
		})
	}
}
