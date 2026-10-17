package goldenframe

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aurago/internal/videostudio"
)

// Names (asset ids) of the built-in media.
const (
	MediaGradient = "gradient" // gradient.mp4: 1280x720, 25 fps, 3 s, smooth slowly moving colour field
	MediaPortrait = "portrait" // portrait.mp4: 720x1280, 25 fps, 2 s
	MediaLogo     = "logo"     // logo.png: 640x360 RGBA, orange ellipse with soft transparent edges, centre (240, 120, 40)
	MediaRed      = "red"      // red.png: 640x360 opaque (255, 0, 0)
	MediaBlue     = "blue"     // blue.png: 640x360 opaque (0, 0, 255)
)

// Media is the generated fixture set of one golden run.
type Media struct {
	Dir    string
	Assets map[string]videostudio.Asset // by fixture name (= asset id): probed with videostudio.Probe, .cube files inspected with videostudio.InspectLUTFile
	Paths  map[string]string            // asset id -> file path
	Files  map[string]string            // asset id -> file name (served under /media/ by the browser harness)
}

// mediaGenerator writes one fixture into dir and returns its plain file name.
type mediaGenerator func(ctx context.Context, ffmpegPath, dir string) (string, error)

type namedMedia struct {
	name     string
	generate mediaGenerator
}

// mediaSet lists what MakeMedia writes: the built-ins, then what later slices add with registerMedia. Videos are
// rendered small with geq and scaled up bicubically, so they are smooth (bicubic export and bilinear GL sampling
// agree) and cheap to generate.
var mediaSet = []namedMedia{
	{MediaGradient, lavfiVideo("gradient.mp4", "nullsrc=s=320x180:r=25:d=3,format=gbrp,"+
		"geq=r=128+80*sin(X/40+T*0.8):g=128+80*sin(Y/30-T*0.6):b=128+80*sin((X+Y)/60+T*0.4),scale=1280:720:flags=bicubic")},
	{MediaPortrait, lavfiVideo("portrait.mp4", "nullsrc=s=180x320:r=25:d=2,format=gbrp,"+
		"geq=r=128+80*sin(Y/40+T):g=128+80*cos(X/25-T*0.5):b=128+80*sin((X-Y)/50+T*0.3),scale=720:1280:flags=bicubic")},
	{MediaLogo, pngFixture("logo.png", 640, 360, logoPixel)},
	{MediaRed, pngFixture("red.png", 640, 360, solid(color.NRGBA{R: 255, A: 255}))},
	{MediaBlue, pngFixture("blue.png", 640, 360, solid(color.NRGBA{B: 255, A: 255}))},
}

// registerMedia adds a fixture for a later slice; call it from an init function in that slice's cases file.
func registerMedia(name string, generate mediaGenerator) {
	for _, item := range mediaSet {
		if item.name == name {
			panic("goldenframe: media " + name + " is registered twice")
		}
	}
	mediaSet = append(mediaSet, namedMedia{name: name, generate: generate})
}

// MakeMedia writes every registered fixture into dir (created when missing) and probes it with videostudio.Probe. A .cube
// LUT is inspected with videostudio.InspectLUTFile instead, the way the server's probe job records LUT assets.
// Nothing binary is committed; each golden run generates its media.
func MakeMedia(ctx context.Context, ffmpegPath, dir string) (Media, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Media{}, fmt.Errorf("create golden media directory: %w", err)
	}
	m := Media{Dir: dir, Assets: map[string]videostudio.Asset{}, Paths: map[string]string{}, Files: map[string]string{}}
	for _, item := range mediaSet {
		file, err := item.generate(ctx, ffmpegPath, dir)
		if err != nil {
			return Media{}, fmt.Errorf("generate golden media %s: %w", item.name, err)
		}
		if file == "" || file != filepath.Base(file) {
			return Media{}, fmt.Errorf("golden media %s returned an invalid file name %q", item.name, file)
		}
		path := filepath.Join(dir, file)
		var asset videostudio.Asset
		if videostudio.IsLUTFileName(file) {
			asset, err = videostudio.InspectLUTFile(path)
		} else {
			asset, err = videostudio.Probe(ctx, ffmpegPath, path)
		}
		if err != nil {
			return Media{}, fmt.Errorf("probe golden media %s: %w", item.name, err)
		}
		asset.ID = item.name
		m.Assets[item.name], m.Paths[item.name], m.Files[item.name] = asset, path, file
	}
	return m, nil
}

// lavfiVideo encodes a lavfi source as BT.709-tagged, limited-range, all-intra H.264 (-g 1), so browser seeks
// land on exact frames and both decoders use the same matrix. The tags are set on the frames with setparams as well
// as on the encoder: with FFmpeg 8.1 the encoder options alone left the stream's primaries and transfer "unknown".
func lavfiVideo(file, source string) mediaGenerator {
	return func(ctx context.Context, ffmpegPath, dir string) (string, error) {
		args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", source,
			"-vf", "scale=out_color_matrix=bt709:out_range=tv,format=yuv420p,setparams=colorspace=bt709:color_primaries=bt709:color_trc=bt709:range=tv", "-an",
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "12", "-g", "1", "-pix_fmt", "yuv420p",
			"-colorspace", "bt709", "-color_primaries", "bt709", "-color_trc", "bt709", "-color_range", "tv",
			"-movflags", "+faststart", "-f", "mp4", filepath.Join(dir, file)}
		if output, err := exec.CommandContext(ctx, ffmpegPath, args...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return file, nil
	}
}

// pngFixture writes a straight-alpha PNG; pixel gives the colour of each pixel.
func pngFixture(file string, width, height int, pixel func(x, y int) color.NRGBA) mediaGenerator {
	return func(_ context.Context, _, dir string) (string, error) {
		img := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				img.SetNRGBA(x, y, pixel(x, y))
			}
		}
		out, err := os.Create(filepath.Join(dir, file))
		if err != nil {
			return "", err
		}
		if err := png.Encode(out, img); err != nil {
			_ = out.Close()
			return "", err
		}
		return file, out.Close()
	}
}

func solid(c color.NRGBA) func(int, int) color.NRGBA {
	return func(int, int) color.NRGBA { return c }
}

// logoPixel draws an orange ellipse (radii 280 x 150 around the centre) whose alpha falls smoothly to 0 over the outer
// quarter of its radius. Green rises from 60 to 180 across the width, so the centre is (240, 120, 40) and geometry
// errors show. Fully transparent pixels keep the same colour, so resampling at the edges cannot bleed a foreign hue.
func logoPixel(x, y int) color.NRGBA {
	dx := (float64(x) + 0.5 - 320) / 280
	dy := (float64(y) + 0.5 - 180) / 150
	t := math.Max(0, math.Min(1, (1-math.Hypot(dx, dy))/0.25))
	return color.NRGBA{R: 240, G: uint8(math.Round(60 + 120*float64(x)/639)), B: 40, A: uint8(math.Round(255 * t * t * (3 - 2*t)))}
}
