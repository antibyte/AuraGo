package videostudio

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Encoder codec names.
const (
	CodecX264  = "libx264"
	CodecVAAPI = "h264_vaapi"
	CodecQSV   = "h264_qsv"
	CodecNVENC = "h264_nvenc"
	CodecAMF   = "h264_amf"
)

const defaultRenderDevice = "/dev/dri/renderD128"

// Encoder selects the video encoder. The zero value is libx264 at CRF 23.
type Encoder struct {
	Codec    string // one of the Codec* names; "" means libx264
	Device   string // DRM render node for VAAPI (and QSV on Linux); "" means /dev/dri/renderD128
	Quality  int    // CRF-equivalent quality 1–51; 0 means 23
	Lossless bool   // libx264 only: -qp 0, used by tests to compare segmentations bit-exactly
}

// Hardware reports whether the encoder runs on a GPU.
func (e Encoder) Hardware() bool { return e.Codec != "" && e.Codec != CodecX264 }

func (e Encoder) quality() int {
	if e.Quality <= 0 {
		return 23
	}
	return e.Quality
}

func (e Encoder) device() string {
	if e.Device != "" {
		return e.Device
	}
	return defaultRenderDevice
}

// globalArgs initialise the hardware device; they precede all inputs.
func (e Encoder) globalArgs() []string {
	switch e.Codec {
	case CodecVAAPI:
		return []string{"-vaapi_device", e.device()}
	case CodecQSV:
		if runtime.GOOS == "linux" {
			return []string{"-init_hw_device", "vaapi=va:" + e.device(), "-init_hw_device", "qsv=hw@va", "-filter_hw_device", "hw"}
		}
		return []string{"-init_hw_device", "qsv=hw", "-filter_hw_device", "hw"}
	}
	return nil
}

// filterTail uploads composed frames to the GPU for encoders that need hardware frames.
func (e Encoder) filterTail() string {
	if e.Codec == CodecVAAPI || e.Codec == CodecQSV {
		return ",format=nv12,hwupload"
	}
	return ""
}

func (e Encoder) videoArgs(rate Rate) []string {
	quality := strconv.Itoa(e.quality())
	gop := strconv.FormatInt((2*rate.Num+rate.Den-1)/rate.Den, 10)
	var args []string
	switch e.Codec {
	case CodecVAAPI:
		args = []string{"-c:v", CodecVAAPI, "-rc_mode", "CQP", "-qp", strconv.Itoa(e.quality() + 1)}
	case CodecQSV:
		args = []string{"-c:v", CodecQSV, "-global_quality", quality}
	case CodecNVENC:
		args = []string{"-c:v", CodecNVENC, "-preset", "p4", "-rc", "vbr", "-cq", quality, "-b:v", "0", "-pix_fmt", "yuv420p"}
	case CodecAMF:
		args = []string{"-c:v", CodecAMF, "-rc", "cqp", "-qp_i", quality, "-qp_p", quality, "-qp_b", quality, "-pix_fmt", "yuv420p"}
	default:
		args = []string{"-c:v", CodecX264, "-preset", "veryfast"}
		if e.Lossless {
			args = append(args, "-qp", "0")
		} else {
			args = append(args, "-crf", quality)
		}
		args = append(args, "-force_key_frames", "expr:eq(n,0)", "-pix_fmt", "yuv420p")
	}
	return append(args, "-g", gop, "-colorspace", "bt709", "-color_primaries", "bt709", "-color_trc", "bt709")
}

// EncoderStatus describes the encoder chosen for exports.
type EncoderStatus struct {
	Encoder  Encoder `json:"-"`
	Name     string  `json:"name"`
	Device   string  `json:"device,omitempty"`
	Hardware bool    `json:"hardware"`
	Note     string  `json:"note,omitempty"`
}

// testEncode encodes a one-second test clip; tests replace it.
var testEncode = func(ctx context.Context, ffmpeg string, encoder Encoder) error {
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, encoder.globalArgs()...)
	args = append(args, "-f", "lavfi", "-i", "testsrc2=s=1280x720:r=30:d=1", "-vf", "format=yuv420p"+encoder.filterTail())
	args = append(args, encoder.videoArgs(Rate{Num: 30, Den: 1})...)
	args = append(args, "-f", "null", "-")
	_, err := runSmallCommand(ctx, ffmpeg, args, 64<<10)
	return err
}

// SelectEncoder picks the export encoder. Mode "off" uses libx264, a named mode (vaapi, qsv, nvenc, amf) tests
// only that encoder, and "auto" tests the platform's candidates in order. The first encoder that encodes a test
// clip wins; otherwise the status names libx264 and why hardware encoding was not used.
func SelectEncoder(ctx context.Context, ffmpegPath, mode, device string) EncoderStatus {
	software := EncoderStatus{Name: CodecX264}
	ffmpeg, err := resolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		software.Note = err.Error()
		return software
	}
	var failures []string
	for _, candidate := range encoderCandidates(runtime.GOOS, mode, device, fileExists) {
		checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := testEncode(checkCtx, ffmpeg, candidate)
		cancel()
		if err == nil {
			status := EncoderStatus{Encoder: candidate, Name: candidate.Codec, Hardware: true}
			if candidate.Codec == CodecVAAPI || candidate.Codec == CodecQSV && runtime.GOOS == "linux" {
				status.Device = candidate.device()
			}
			return status
		}
		failures = append(failures, candidate.Codec+": "+firstLine(err.Error()))
	}
	if len(failures) > 0 {
		software.Note = "hardware encoding unavailable (" + strings.Join(failures, "; ") + ")"
	}
	return software
}

func encoderCandidates(goos, mode, device string, exists func(string) bool) []Encoder {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	named := map[string]string{"vaapi": CodecVAAPI, "qsv": CodecQSV, "nvenc": CodecNVENC, "amf": CodecAMF}
	if codec, ok := named[mode]; ok {
		return []Encoder{{Codec: codec, Device: device}}
	}
	if mode != "auto" {
		return nil
	}
	var order []string
	switch goos {
	case "linux":
		order = []string{CodecVAAPI, CodecQSV, CodecNVENC}
	case "windows":
		order = []string{CodecNVENC, CodecQSV, CodecAMF}
	}
	candidates := []Encoder{}
	for _, codec := range order {
		candidate := Encoder{Codec: codec, Device: device}
		needsNode := codec == CodecVAAPI || codec == CodecQSV && goos == "linux"
		if needsNode && !exists(candidate.device()) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
