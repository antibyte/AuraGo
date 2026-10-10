package cydcreator

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Board status and chip values.
const (
	BoardVerified  = "verified"
	BoardCommunity = "community"
	ChipESP32      = "esp32"
	ChipESP32S3    = "esp32s3"
)

// Board is one embedded hardware profile (boards/<id>.json).
type Board struct {
	ID      string       `json:"id"`
	Label   string       `json:"label"`
	Status  string       `json:"status"`
	Source  string       `json:"source"`
	Chip    string       `json:"chip"`
	FlashMB int          `json:"flash_mb"`
	PSRAM   bool         `json:"psram"`
	PIO     BoardPIO     `json:"pio"`
	Display BoardDisplay `json:"display"`
	Touch   BoardTouch   `json:"touch"`
	Extras  BoardExtras  `json:"extras"`
	LVGL    BoardLVGL    `json:"lvgl"`
}

// BoardPIO holds the PlatformIO board id and extra flags.
type BoardPIO struct {
	Board      string            `json:"board"`
	BuildFlags []string          `json:"build_flags,omitempty"`
	BoardBuild map[string]string `json:"board_build,omitempty"` // e.g. "arduino.memory_type": "qio_opi"
}

// BoardDisplay describes the panel. Width/Height are given in the board's
// native (designed) orientation, not the controller's RAM orientation.
type BoardDisplay struct {
	Driver            string         `json:"driver"` // ILI9341 ST7789 ST7796 NV3041A ST7262 EK9716
	Bus               string         `json:"bus"`    // spi | qspi | rgb
	Width             int            `json:"width"`
	Height            int            `json:"height"`
	NativeOrientation string         `json:"native_orientation"` // landscape | portrait
	Invert            bool           `json:"invert"`
	BGR               bool           `json:"bgr"`
	Pins              map[string]int `json:"pins"` // bus specific keys; "bl" backlight; -1 = not connected
	Hz                int            `json:"hz"`
	BacklightPWM      bool           `json:"bl_pwm"`
	Timing            map[string]int `json:"timing,omitempty"` // rgb panels only
	// OffsetRotation is LovyanGFX's panel offset_rotation (0-3 rotate, 4-7
	// mirrored): it turns the controller-RAM frame into the board's native
	// orientation, so rotation 0 shows the board as described by Width/Height.
	OffsetRotation int `json:"offset_rotation,omitempty"`
}

// BoardTouch describes the touch controller.
type BoardTouch struct {
	Driver           string            `json:"driver"` // XPT2046 | GT911 | CST820
	Bus              string            `json:"bus"`    // spi | i2c
	Pins             map[string]int    `json:"pins"`
	I2CAddr          int               `json:"i2c_addr,omitempty"`
	Calibration      *TouchCalibration `json:"calibration,omitempty"`
	NeedsCalibration bool              `json:"needs_calibration"`
	// OffsetRotation is LovyanGFX's touch offset_rotation for capacitive
	// controllers whose touch layer is turned or mirrored against the panel's
	// controller-RAM frame. XPT2046 boards keep 0 (the calibration carries it).
	OffsetRotation int `json:"offset_rotation,omitempty"`
}

// TouchCalibration holds raw XPT2046 ADC bounds, in the panel's native orientation.
type TouchCalibration struct {
	XMin int `json:"x_min"`
	XMax int `json:"x_max"`
	YMin int `json:"y_min"`
	YMax int `json:"y_max"`
}

// BoardExtras lists optional on-board peripherals; nil = not present.
type BoardExtras struct {
	RGBLED     *RGBLED `json:"rgb_led,omitempty"`
	LDR        *int    `json:"ldr,omitempty"`
	Speaker    *int    `json:"speaker,omitempty"`
	SDCS       *int    `json:"sd_cs,omitempty"`
	BootButton int     `json:"boot_button"`
}

// RGBLED is a three-pin status LED.
type RGBLED struct {
	R         int  `json:"r"`
	G         int  `json:"g"`
	B         int  `json:"b"`
	ActiveLow bool `json:"active_low"`
}

// BoardLVGL tunes LVGL draw buffers for the board's RAM.
type BoardLVGL struct {
	DrawBufferLines int  `json:"draw_buffer_lines"`
	DoubleBuffer    bool `json:"double_buffer"`
}

//go:embed boards/*.json
var boardFS embed.FS

var (
	boardsOnce sync.Once
	boardsList []Board
	boardsErr  error
)

// Boards returns every embedded profile, validated and sorted by ID. The
// result is a deep copy; callers may modify it freely.
func Boards() ([]Board, error) {
	boardsOnce.Do(func() { boardsList, boardsErr = loadBoards(boardFS, "boards") })
	if boardsErr != nil {
		return nil, boardsErr
	}
	out := make([]Board, len(boardsList))
	for i, b := range boardsList {
		out[i] = cloneBoard(b)
	}
	return out, nil
}

// BoardByID returns one profile; unknown ids return *Error CYD_BAD_REQUEST.
func BoardByID(id string) (Board, error) {
	boards, err := Boards()
	if err != nil {
		return Board{}, err
	}
	for _, b := range boards {
		if b.ID == id {
			return b, nil
		}
	}
	return Board{}, NewError(CodeBadRequest, fmt.Sprintf("unknown board %q", id), map[string]any{"board": id})
}

// ScreenSize returns the canvas size for a project orientation. Landscape
// variants put the long side horizontally, portrait variants vertically.
func (b Board) ScreenSize(orientation string) (w, h int) {
	long := max(b.Display.Width, b.Display.Height)
	short := min(b.Display.Width, b.Display.Height)
	switch orientation {
	case "landscape", "landscape_flipped":
		return long, short
	case "portrait", "portrait_flipped":
		return short, long
	}
	return b.Display.Width, b.Display.Height
}

// PIOEnv is the PlatformIO environment name of the board (== ID).
func (b Board) PIOEnv() string { return b.ID }

func loadBoards(fsys fs.FS, dir string) ([]Board, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read board profiles: %w", err)
	}
	var boards []Board
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".json" {
			continue
		}
		file := path.Join(dir, e.Name())
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		b, err := parseBoard(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if want := strings.TrimSuffix(e.Name(), ".json"); b.ID != want {
			return nil, fmt.Errorf("%s: id %q does not match the file name", file, b.ID)
		}
		if prev, dup := seen[b.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate board id %q (also in %s)", file, b.ID, prev)
		}
		seen[b.ID] = file
		if err := validateBoard(b); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		boards = append(boards, b)
	}
	if len(boards) == 0 {
		return nil, errors.New("no board profiles embedded")
	}
	sort.Slice(boards, func(i, j int) bool { return boards[i].ID < boards[j].ID })
	return boards, nil
}

// parseBoard decodes one profile strictly: unknown keys and trailing data fail.
func parseBoard(data []byte) (Board, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var b Board
	if err := dec.Decode(&b); err != nil {
		return Board{}, fmt.Errorf("decode board: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Board{}, errors.New("decode board: trailing data after the profile")
	}
	return b, nil
}

type pinRule struct {
	required []string // must be present and >= 0
	optional []string // may be absent or -1
	inputOK  []string // roles that may use input-only GPIOs
}

var (
	boardIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,47}$`)
	pioBoardPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	buildFlagPattern = regexp.MustCompile(`^-(D[A-Za-z_][A-Za-z0-9_]*(=[A-Za-z0-9_.+-]*)?|m[a-z0-9-]+)$`)
	boardBuildValue  = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)
	// boardBuildKeys are the only board_build options a profile may set; the
	// generator owns everything else (partitions, filesystem, embed_files, ...).
	boardBuildKeys    = []string{"arduino.memory_type", "flash_mode", "f_flash", "f_cpu"}
	displayDriverBus  = map[string]string{"ILI9341": "spi", "ST7789": "spi", "ST7796": "spi", "NV3041A": "qspi", "ST7262": "rgb", "EK9716": "rgb"}
	touchDriverBus    = map[string]string{"XPT2046": "spi", "GT911": "i2c", "CST820": "i2c"}
	touchI2CAddresses = map[string][]int{"GT911": {0x5D, 0x14}, "CST820": {0x15}}
	displayPinRules   = map[string]pinRule{
		"spi":  {required: []string{"sclk", "mosi", "cs", "dc"}, optional: []string{"miso", "rst", "bl"}, inputOK: []string{"miso"}},
		"qspi": {required: []string{"cs", "sclk", "d0", "d1", "d2", "d3"}, optional: []string{"rst", "bl"}},
		"rgb": {required: []string{"de", "vsync", "hsync", "pclk",
			"r0", "r1", "r2", "r3", "r4", "g0", "g1", "g2", "g3", "g4", "g5", "b0", "b1", "b2", "b3", "b4"},
			optional: []string{"bl", "disp"}},
	}
	touchPinRules = map[string]pinRule{
		"spi": {required: []string{"sclk", "mosi", "miso", "cs"}, optional: []string{"irq"}, inputOK: []string{"miso", "irq"}},
		"i2c": {required: []string{"sda", "scl"}, optional: []string{"int", "rst"}, inputOK: []string{"int"}},
	}
	rgbTimingRequired = []string{"pclk_hz", "hsync_polarity", "hsync_front_porch", "hsync_pulse_width",
		"hsync_back_porch", "vsync_polarity", "vsync_front_porch", "vsync_pulse_width", "vsync_back_porch", "pclk_active_neg"}
	rgbTimingOptional = []string{"de_idle_high", "pclk_idle_high"}
	rgbTimingFlags    = []string{"hsync_polarity", "vsync_polarity", "pclk_active_neg", "de_idle_high", "pclk_idle_high"}
)

// validateBoard checks a decoded profile. All problems are reported together.
func validateBoard(b Board) error {
	var probs []string
	bad := func(format string, args ...any) { probs = append(probs, fmt.Sprintf(format, args...)) }

	if !boardIDPattern.MatchString(b.ID) {
		bad("id %q must match %s", b.ID, boardIDPattern)
	}
	if strings.TrimSpace(b.Label) == "" || len(b.Label) > 80 {
		bad("label must be 1..80 bytes")
	}
	if b.Status != BoardVerified && b.Status != BoardCommunity {
		bad("status %q must be verified or community", b.Status)
	}
	if !strings.HasPrefix(b.Source, "https://") {
		bad("source must be an https:// URL")
	}
	if b.Chip != ChipESP32 && b.Chip != ChipESP32S3 {
		bad("chip %q must be esp32 or esp32s3", b.Chip)
	}
	if _, err := PartitionTable(b.FlashMB); err != nil {
		bad("flash_mb %d must be 4, 8 or 16", b.FlashMB)
	}
	if !pioBoardPattern.MatchString(b.PIO.Board) {
		bad("pio.board %q is invalid", b.PIO.Board)
	}
	for _, f := range b.PIO.BuildFlags {
		if !buildFlagPattern.MatchString(f) {
			bad("pio.build_flags entry %q must be -DNAME[=value] or -m<option> without quotes or spaces", f)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(b.PIO.BoardBuild)) {
		v := b.PIO.BoardBuild[k]
		switch {
		case k == "partitions" || k == "filesystem":
			bad("pio.board_build.%s is generated and must not be set", k)
		case !slices.Contains(boardBuildKeys, k):
			bad("pio.board_build key %q is not allowed (allowed: %s)", k, strings.Join(boardBuildKeys, ", "))
		case !boardBuildValue.MatchString(v):
			bad("pio.board_build %q=%q is invalid", k, v)
		}
	}
	// Without the memory type the arduino core does not start PSRAM, and the
	// octal-PSRAM pin check below would be skipped.
	if b.Chip == ChipESP32S3 && b.PSRAM && b.PIO.BoardBuild["arduino.memory_type"] == "" {
		bad(`esp32s3 boards with PSRAM must set pio.board_build["arduino.memory_type"] (e.g. qio_opi)`)
	}

	d := b.Display
	if bus, ok := displayDriverBus[d.Driver]; !ok {
		bad("display.driver %q is unknown", d.Driver)
	} else if bus != d.Bus {
		bad("display.driver %s needs bus %s, not %q", d.Driver, bus, d.Bus)
	}
	if d.Width < 1 || d.Width > 1024 || d.Height < 1 || d.Height > 1024 {
		bad("display size %dx%d must be within 1..1024", d.Width, d.Height)
	}
	switch d.NativeOrientation {
	case "landscape":
		if d.Width < d.Height {
			bad("landscape display must be at least as wide as high")
		}
	case "portrait":
		if d.Height < d.Width {
			bad("portrait display must be at least as high as wide")
		}
	default:
		bad("display.native_orientation %q must be landscape or portrait", d.NativeOrientation)
	}
	switch d.Bus {
	case "spi", "qspi":
		if d.Hz < 1_000_000 || d.Hz > 80_000_000 {
			bad("display.hz %d must be within 1..80 MHz", d.Hz)
		}
		if len(d.Timing) > 0 {
			bad("display.timing is only allowed for rgb panels")
		}
		if d.Bus == "qspi" && b.Chip != ChipESP32S3 {
			bad("qspi panels need an esp32s3")
		}
	case "rgb":
		if d.Hz != 0 {
			bad("display.hz must be 0 for rgb panels (use timing.pclk_hz)")
		}
		if b.Chip != ChipESP32S3 || !b.PSRAM {
			bad("rgb panels need an esp32s3 with PSRAM")
		}
		probs = append(probs, checkRGBTiming(d.Timing)...)
	}
	if rule, ok := displayPinRules[d.Bus]; ok {
		probs = append(probs, checkPins(b, "display", d.Pins, rule)...)
	}
	if d.OffsetRotation < 0 || d.OffsetRotation > 7 {
		bad("display.offset_rotation %d must be within 0..7", d.OffsetRotation)
	}

	t := b.Touch
	if bus, ok := touchDriverBus[t.Driver]; !ok {
		bad("touch.driver %q is unknown", t.Driver)
	} else if bus != t.Bus {
		bad("touch.driver %s needs bus %s, not %q", t.Driver, bus, t.Bus)
	}
	if rule, ok := touchPinRules[t.Bus]; ok {
		probs = append(probs, checkPins(b, "touch", t.Pins, rule)...)
	}
	if t.OffsetRotation < 0 || t.OffsetRotation > 7 {
		bad("touch.offset_rotation %d must be within 0..7", t.OffsetRotation)
	} else if t.Driver == "XPT2046" && t.OffsetRotation != 0 {
		bad("touch.offset_rotation must be 0 for XPT2046 (the calibration carries the orientation)")
	}
	switch t.Driver {
	case "XPT2046":
		c := t.Calibration
		switch {
		case c == nil:
			bad("XPT2046 touch needs a default calibration")
		case c.XMin < 0 || c.YMin < 0 || c.XMax > 4095 || c.YMax > 4095 || c.XMin >= c.XMax || c.YMin >= c.YMax:
			bad("touch.calibration must satisfy 0 <= min < max <= 4095")
		}
		if t.I2CAddr != 0 {
			bad("touch.i2c_addr is only allowed for i2c touch")
		}
	case "GT911", "CST820":
		if t.Calibration != nil || t.NeedsCalibration {
			bad("capacitive touch must not carry a calibration")
		}
		if !slices.Contains(touchI2CAddresses[t.Driver], t.I2CAddr) {
			bad("touch.i2c_addr 0x%X is not valid for %s", t.I2CAddr, t.Driver)
		}
	}

	e := b.Extras
	probs = append(probs, checkSinglePin(b, "extras.boot_button", e.BootButton, true)...)
	if e.RGBLED != nil {
		probs = append(probs, checkSinglePin(b, "extras.rgb_led.r", e.RGBLED.R, false)...)
		probs = append(probs, checkSinglePin(b, "extras.rgb_led.g", e.RGBLED.G, false)...)
		probs = append(probs, checkSinglePin(b, "extras.rgb_led.b", e.RGBLED.B, false)...)
	}
	if e.LDR != nil {
		probs = append(probs, checkSinglePin(b, "extras.ldr", *e.LDR, true)...)
		// ADC2 cannot be read while Wi-Fi is running, so the LDR needs an ADC1 pin.
		if gpioExists(b.Chip, *e.LDR) && !gpioADC1(b.Chip, *e.LDR) {
			bad("extras.ldr: GPIO %d is not an ADC1 pin (ADC2 is unusable while Wi-Fi runs)", *e.LDR)
		}
	}
	if e.Speaker != nil {
		probs = append(probs, checkSinglePin(b, "extras.speaker", *e.Speaker, false)...)
	}
	if e.SDCS != nil {
		probs = append(probs, checkSinglePin(b, "extras.sd_cs", *e.SDCS, false)...)
	}
	probs = append(probs, checkPinConflicts(b)...)
	probs = append(probs, checkSPIBusSharing(b)...)

	long := max(d.Width, d.Height)
	if b.LVGL.DrawBufferLines < 1 || b.LVGL.DrawBufferLines > long {
		bad("lvgl.draw_buffer_lines %d must be within 1..%d", b.LVGL.DrawBufferLines, long)
	} else if !b.PSRAM {
		bufBytes := b.LVGL.DrawBufferLines * long * 2
		if b.LVGL.DoubleBuffer {
			bufBytes *= 2
		}
		if bufBytes > maxInternalDrawBuffer {
			bad("lvgl draw buffers need %d bytes; boards without PSRAM allow %d", bufBytes, maxInternalDrawBuffer)
		}
	}

	if len(probs) > 0 {
		return fmt.Errorf("board %q: %s", b.ID, strings.Join(probs, "; "))
	}
	return nil
}

func checkRGBTiming(timing map[string]int) []string {
	var probs []string
	for _, k := range rgbTimingRequired {
		if _, ok := timing[k]; !ok {
			probs = append(probs, fmt.Sprintf("display.timing.%s is required", k))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(timing)) {
		v := timing[k]
		switch {
		case !slices.Contains(rgbTimingRequired, k) && !slices.Contains(rgbTimingOptional, k):
			probs = append(probs, fmt.Sprintf("display.timing.%s is unknown", k))
		case slices.Contains(rgbTimingFlags, k) && v != 0 && v != 1:
			probs = append(probs, fmt.Sprintf("display.timing.%s must be 0 or 1", k))
		case k == "pclk_hz" && (v < 1_000_000 || v > 40_000_000):
			probs = append(probs, "display.timing.pclk_hz must be within 1..40 MHz")
		// Porch and pulse widths are int8_t in LovyanGFX Bus_RGB::config_t.
		case k != "pclk_hz" && (v < 0 || v > 127):
			probs = append(probs, fmt.Sprintf("display.timing.%s=%d is out of range", k, v))
		}
	}
	return probs
}

func checkPins(b Board, group string, pins map[string]int, rule pinRule) []string {
	var probs []string
	for _, role := range rule.required {
		if _, ok := pins[role]; !ok {
			probs = append(probs, fmt.Sprintf("%s.pins.%s is required", group, role))
		}
	}
	for _, role := range slices.Sorted(maps.Keys(pins)) {
		pin := pins[role]
		required := slices.Contains(rule.required, role)
		if !required && !slices.Contains(rule.optional, role) {
			probs = append(probs, fmt.Sprintf("%s.pins.%s is not a pin of this bus", group, role))
			continue
		}
		if pin == -1 && !required {
			continue
		}
		probs = append(probs, checkSinglePin(b, group+".pins."+role, pin, slices.Contains(rule.inputOK, role))...)
	}
	return probs
}

// checkSinglePin validates one GPIO for the board's chip.
func checkSinglePin(b Board, label string, pin int, inputOK bool) []string {
	if !gpioExists(b.Chip, pin) {
		return []string{fmt.Sprintf("%s: GPIO %d does not exist on %s", label, pin, b.Chip)}
	}
	if reason := gpioReserved(b, pin); reason != "" {
		return []string{fmt.Sprintf("%s: GPIO %d is %s", label, pin, reason)}
	}
	if !inputOK && gpioInputOnly(b.Chip, pin) {
		return []string{fmt.Sprintf("%s: GPIO %d is input-only", label, pin)}
	}
	return nil
}

func gpioExists(chip string, pin int) bool {
	switch chip {
	case ChipESP32:
		return (pin >= 0 && pin <= 19) || (pin >= 21 && pin <= 23) || (pin >= 25 && pin <= 27) || (pin >= 32 && pin <= 39)
	case ChipESP32S3:
		return (pin >= 0 && pin <= 21) || (pin >= 26 && pin <= 48)
	}
	return false
}

func gpioReserved(b Board, pin int) string {
	switch b.Chip {
	case ChipESP32:
		if pin >= 6 && pin <= 11 {
			return "wired to the SPI flash"
		}
	case ChipESP32S3:
		if pin >= 26 && pin <= 32 {
			return "wired to the SPI flash/PSRAM"
		}
		if pin >= 33 && pin <= 37 && strings.Contains(b.PIO.BoardBuild["arduino.memory_type"], "opi") {
			return "used by octal PSRAM"
		}
	}
	return ""
}

func gpioInputOnly(chip string, pin int) bool {
	return chip == ChipESP32 && pin >= 34 && pin <= 39
}

func gpioADC1(chip string, pin int) bool {
	switch chip {
	case ChipESP32:
		return pin >= 32 && pin <= 39
	case ChipESP32S3:
		return pin >= 1 && pin <= 10
	}
	return false
}

type pinUse struct{ group, role string }

// checkPinConflicts rejects every GPIO used twice. The only allowed reuse is
// a touch controller on the display's SPI bus: when both buses are spi, touch
// may use the identical sclk/mosi/miso pins in the same role.
func checkPinConflicts(b Board) []string {
	uses := map[int][]pinUse{}
	add := func(group, role string, pin int) {
		if pin >= 0 {
			uses[pin] = append(uses[pin], pinUse{group, role})
		}
	}
	for _, role := range slices.Sorted(maps.Keys(b.Display.Pins)) {
		add("display", role, b.Display.Pins[role])
	}
	for _, role := range slices.Sorted(maps.Keys(b.Touch.Pins)) {
		add("touch", role, b.Touch.Pins[role])
	}
	e := b.Extras
	add("extras", "boot_button", e.BootButton)
	if e.RGBLED != nil {
		add("extras", "rgb_led.r", e.RGBLED.R)
		add("extras", "rgb_led.g", e.RGBLED.G)
		add("extras", "rgb_led.b", e.RGBLED.B)
	}
	if e.LDR != nil {
		add("extras", "ldr", *e.LDR)
	}
	if e.Speaker != nil {
		add("extras", "speaker", *e.Speaker)
	}
	if e.SDCS != nil {
		add("extras", "sd_cs", *e.SDCS)
	}
	var probs []string
	for _, pin := range slices.Sorted(maps.Keys(uses)) {
		list := uses[pin]
		if len(list) < 2 || sharedSPIBus(b, list) {
			continue
		}
		names := make([]string, len(list))
		for i, u := range list {
			names[i] = u.group + "." + u.role
		}
		probs = append(probs, fmt.Sprintf("GPIO %d is used twice (%s)", pin, strings.Join(names, ", ")))
	}
	return probs
}

// checkSPIBusSharing requires a touch controller to share the display's SPI
// bus completely (sclk, mosi and miso) or not at all; the template decides
// bus sharing from sclk alone.
func checkSPIBusSharing(b Board) []string {
	if b.Display.Bus != "spi" || b.Touch.Bus != "spi" {
		return nil
	}
	shared := 0
	for _, role := range []string{"sclk", "mosi", "miso"} {
		if pin, ok := b.Display.Pins[role]; ok && pin >= 0 && b.Touch.Pins[role] == pin {
			shared++
		}
	}
	if shared != 0 && shared != 3 {
		return []string{"touch must use the display's sclk, mosi and miso exactly or none of them"}
	}
	return nil
}

func sharedSPIBus(b Board, list []pinUse) bool {
	if len(list) != 2 || b.Display.Bus != "spi" || b.Touch.Bus != "spi" {
		return false
	}
	a, c := list[0], list[1]
	return a.group == "display" && c.group == "touch" && a.role == c.role &&
		(a.role == "sclk" || a.role == "mosi" || a.role == "miso")
}

func cloneBoard(b Board) Board {
	out := b
	out.PIO.BuildFlags = slices.Clone(b.PIO.BuildFlags)
	out.PIO.BoardBuild = maps.Clone(b.PIO.BoardBuild)
	out.Display.Pins = maps.Clone(b.Display.Pins)
	out.Display.Timing = maps.Clone(b.Display.Timing)
	out.Touch.Pins = maps.Clone(b.Touch.Pins)
	if b.Touch.Calibration != nil {
		c := *b.Touch.Calibration
		out.Touch.Calibration = &c
	}
	if b.Extras.RGBLED != nil {
		l := *b.Extras.RGBLED
		out.Extras.RGBLED = &l
	}
	out.Extras.LDR = dupIntPtr(b.Extras.LDR)
	out.Extras.Speaker = dupIntPtr(b.Extras.Speaker)
	out.Extras.SDCS = dupIntPtr(b.Extras.SDCS)
	return out
}

func dupIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
