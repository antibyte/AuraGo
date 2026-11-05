package render

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

// Stored FFmpeg 8.1.1 output: a default framelog line and the ebur128 summary from the spike's
// speech.ebu.txt, and the loudnorm JSON of the same file from speech.pass2.txt. The frame line says
// I -17.3 and the JSON says -16.69, so only the summary line gives -16.6.
const meterStderrSpeech = `[Parsed_ebur128_0 @ 000002499d844d40] t: 1.599979   TARGET:-23 LUFS    M: -29.6 S:-120.7     I: -17.3 LUFS       LRA:   0.0 LU  FTPK:  -inf  -inf dBFS  TPK:  -2.4  -2.4 dBFS
[Parsed_ebur128_0 @ 000002499d844d40] Summary:

  Integrated loudness:
    I:         -16.6 LUFS
    Threshold: -27.1 LUFS

  Loudness range:
    LRA:         2.7 LU
    Threshold: -37.5 LUFS
    LRA low:   -18.5 LUFS
    LRA high:  -15.8 LUFS

  True peak:
    Peak:       -1.0 dBFS
[Parsed_loudnorm_0 @ 00000185953e53c0]
{
	"input_i" : "-16.69",
	"input_tp" : "-1.00",
	"input_lra" : "2.10",
	"input_thresh" : "-27.16",
	"output_i" : "-16.29",
	"output_tp" : "-1.00",
	"output_lra" : "1.60",
	"output_thresh" : "-26.79",
	"normalization_type" : "dynamic",
	"target_offset" : "0.29"
}
[out#0/null @ 00000185953e4800] video:0KiB audio:20628KiB subtitle:0KiB other streams:0KiB global headers:0KiB muxing overhead: unknown
`

// The JSON is stored from the spike's silent.pass1.txt. ebur128 reads silence as -70.0 (mixreal/ebur.txt).
const meterStderrSilent = `[Parsed_ebur128_0 @ 000002499d844d40] Summary:

  Integrated loudness:
    I:         -70.0 LUFS
[Parsed_loudnorm_0 @ 0000015e6e443a80]
{
	"input_i" : "-inf",
	"input_tp" : "-inf",
	"input_lra" : "0.00",
	"input_thresh" : "-70.00",
	"output_i" : "-inf",
	"output_tp" : "-inf",
	"output_lra" : "0.00",
	"output_thresh" : "-70.00",
	"normalization_type" : "dynamic",
	"target_offset" : "inf"
}
`

func TestMeterArgsPinTheGraph(t *testing.T) {
	want := []string{
		"-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,pipe", "-format_whitelist", "f32le",
		"-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "pipe:0",
		"-af", "ebur128=framelog=verbose,loudnorm=I=-16:TP=-1:LRA=11:print_format=json", "-f", "null", "-",
	}
	if got := MeterArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("MeterArgs() = %q", got)
	}
}

func TestMeterParseTakesIFromTheSummaryAndTPFromTheJSON(t *testing.T) {
	got, err := parseMeterOutput(meterStderrSpeech)
	if err != nil {
		t.Fatalf("parseMeterOutput: %v", err)
	}
	if got.I != -16.6 || got.TP != -1.0 || got.Silent {
		t.Fatalf("reading = %+v, want I -16.6, TP -1, not silent", got)
	}
}

func TestMeterParseReadsMinusInfAsSilent(t *testing.T) {
	got, err := parseMeterOutput(meterStderrSilent)
	if err != nil {
		t.Fatalf("parseMeterOutput: %v", err)
	}
	if !got.Silent || !math.IsInf(got.I, -1) || !math.IsInf(got.TP, -1) {
		t.Fatalf("reading = %+v, want silent with -Inf levels", got)
	}
}

func TestMeterParseRefusesOutputWithoutBothMeters(t *testing.T) {
	if _, err := parseMeterOutput("    I:         -16.6 LUFS\n"); err == nil {
		t.Fatal("output without the JSON was accepted")
	}
	if _, err := parseMeterOutput(`{"input_i":"-16.69","input_tp":"-1.00"}`); err == nil {
		t.Fatal("output without the summary was accepted")
	}
}

func runMeterOnSine(t *testing.T, ffmpeg string, seconds int, amp float64) MeterReading {
	t.Helper()
	m, err := StartMeter(context.Background(), ffmpeg)
	if err != nil {
		t.Fatalf("StartMeter: %v", err)
	}
	const rate, blockFrames = 48000, 4800
	block := make([]float32, blockFrames*2)
	for start := 0; start < seconds*rate; start += blockFrames {
		for i := 0; i < blockFrames; i++ {
			v := float32(amp * math.Sin(2*math.Pi*1000*float64(start+i)/rate))
			block[i*2], block[i*2+1] = v, v
		}
		if err := m.Write(block); err != nil {
			_, closeErr := m.Close()
			t.Fatalf("Write: %v (Close: %v)", err, closeErr)
		}
	}
	reading, err := m.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	return reading
}

func TestMeterRealGraph(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")

	// EBU Tech 3341 case 1: a 1 kHz stereo sine reads its per-channel peak level in LUFS. So amplitude
	// 0.1 (-20 dBFS) reads -20 LUFS, and the true peak of a sine is its peak, -20 dBTP.
	loud := runMeterOnSine(t, ffmpeg, 5, 0.1)
	if loud.Silent || math.Abs(loud.I+20) > 0.5 || math.Abs(loud.TP+20) > 0.5 {
		t.Fatalf("sine reading = %+v, want about -20 LUFS and -20 dBTP", loud)
	}

	silent := runMeterOnSine(t, ffmpeg, 5, 0)
	if !silent.Silent {
		t.Fatalf("silence reading = %+v, want silent", silent)
	}
}

// meterToneWAV is a 48 kHz stereo PCM16 WAV of a 1 kHz sine at amplitude 0.1.
func meterToneWAV(seconds int) []byte {
	const rate = 48000
	frames := seconds * rate
	data := make([]byte, 44+frames*4)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+frames*4))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*4)
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(frames*4))
	for i := 0; i < frames; i++ {
		v := int16(0.1 * 32767 * math.Sin(2*math.Pi*1000*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[44+i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[46+i*4:], uint16(v))
	}
	return data
}

func TestMeterStoredReadsAFileFromItsInput(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	// The same reading as TestMeterRealGraph: amplitude 0.1 reads -20 LUFS.
	reading, err := MeterStored(context.Background(), ffmpeg, bytes.NewReader(meterToneWAV(5)))
	if err != nil {
		t.Fatalf("MeterStored: %v", err)
	}
	if reading.Silent || math.Abs(reading.I+20) > 0.5 {
		t.Fatalf("reading = %+v, want about -20 LUFS", reading)
	}
	if _, err := MeterStored(context.Background(), ffmpeg, bytes.NewReader([]byte("this is not audio"))); err == nil {
		t.Fatal("bytes that are no audio were measured")
	}
}
