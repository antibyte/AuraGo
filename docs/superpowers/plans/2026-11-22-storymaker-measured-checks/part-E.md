# Part E: Audio checks: speech pieces, mix, export and job card

> Generated from the drafted and tested commits f7436c8fd..8b8e5922a. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task E1: detect a dropout inside a synthesized piece

**Files:**
- Create: `internal/storymaker/piececheck.go`
- Create: `internal/storymaker/piececheck_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/piececheck_test.go`:

```go
package storymaker

import (
	"math"
	"reflect"
	"testing"
)

const pieceTestRate = 24000

// pieceTestSpeech is a mono 24 kHz sine of the given amplitude (a 220 Hz tone stands in for a voice).
func pieceTestSpeech(seconds, amp float64) DecodedAudio {
	samples := make([]float32, int(seconds*pieceTestRate))
	for i := range samples {
		samples[i] = float32(amp * math.Sin(2*math.Pi*220*float64(i)/pieceTestRate))
	}
	return DecodedAudio{Rate: pieceTestRate, Channels: 1, Samples: samples}
}

// pieceTestZero silences the samples from startMS for lengthMS.
func pieceTestZero(a DecodedAudio, startMS, lengthMS int) {
	for i := startMS * pieceTestRate / 1000; i < (startMS+lengthMS)*pieceTestRate/1000; i++ {
		a.Samples[i] = 0
	}
}

func pieceTestCheck(a DecodedAudio) []string {
	return CheckPiece(a, ComputeEnvelope(a))
}

func TestCheckPieceDropout(t *testing.T) {
	// A 0.1 sine is -23 dBFS. 100 ms of digital silence in the middle falls 57 dB in one step and stays under -80 dBFS.
	a := pieceTestSpeech(2, 0.1)
	pieceTestZero(a, 1000, 100)
	if got := pieceTestCheck(a); !reflect.DeepEqual(got, []string{"dropout"}) {
		t.Errorf("a 100 ms hole: %v", got)
	}
}

func TestCheckPieceDropoutIgnoresWhatSpeechDoesNaturally(t *testing.T) {
	clean := pieceTestSpeech(2, 0.1)
	short := pieceTestSpeech(2, 0.1)
	pieceTestZero(short, 1000, 30)
	quiet := pieceTestSpeech(2, 0.1)
	for i := 1000 * pieceTestRate / 1000; i < 1100*pieceTestRate/1000; i++ {
		quiet.Samples[i] *= 0.01 // -63 dBFS: a fall of 40 dB, but not below -80 dBFS
	}
	// Speech that ends in digital silence is an ending, not a hole: nothing speaks afterwards.
	ending := pieceTestSpeech(2, 0.1)
	pieceTestZero(ending, 1500, 500)
	// A hole that lasts 3.5 s is a pause, not a dropout.
	long := pieceTestSpeech(8, 0.1)
	pieceTestZero(long, 2000, 3500)
	for name, a := range map[string]DecodedAudio{"clean": clean, "30 ms hole": short, "shallow hole": quiet, "ending": ending, "long pause": long} {
		if got := pieceTestCheck(a); len(got) != 0 {
			t.Errorf("%s: %v", name, got)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestCheckPiece' -count=1`
Expected: internal\storymaker\piececheck_test.go:28:9: undefined: CheckPiece

- [ ] **Step 3: Implement**

Create `internal/storymaker/piececheck.go`:

```go
package storymaker

const (
	// A dropout is a fall from at least dropoutFromPower to below dropoutToPower in one 10 ms step inside speech,
	// followed by dropoutMinWindows to dropoutMaxWindows of silence and then speech again.
	dropoutFromPower  = 3.1622776601683795e-5 // -45 dBFS
	dropoutToPower    = 1e-8                  // -80 dBFS
	dropoutMinWindows = 5                     // 50 ms
	dropoutMaxWindows = 300                   // 3 s
)

// CheckPiece reports the faults of one synthesized piece: "clipped", "dropout" (sorted, each at most once). env is the
// envelope of the same audio; its speech runs say where a fall counts as a hole and not as the end of the speech.
func CheckPiece(audio DecodedAudio, env Envelope) []string {
	var faults []string
	if pieceDropout(audio, env) {
		faults = append(faults, "dropout")
	}
	return faults
}

// windowPowers is the mean power per 10 ms window with the windowing of ComputeEnvelope, but unclamped, because the
// envelope's level byte stops at -60 dBFS and a dropout lies below -80 dBFS.
func windowPowers(a DecodedAudio) []float64 {
	if a.Rate <= 0 || a.Channels <= 0 || len(a.Samples) < a.Channels {
		return nil
	}
	channels := int64(a.Channels)
	frames := int64(len(a.Samples)) / channels
	rate := int64(a.Rate)
	out := make([]float64, (frames*envelopeRate+rate-1)/rate)
	for i := range out {
		first := int64(i) * rate / envelopeRate
		end := min(int64(i+1)*rate/envelopeRate, frames)
		if end <= first {
			continue
		}
		sum := 0.0
		samples := a.Samples[first*channels : end*channels]
		for _, s := range samples {
			sum += float64(s) * float64(s)
		}
		out[i] = sum / float64(len(samples))
	}
	return out
}

// pieceDropout finds a hole inside speech: the window before the fall lies in a speech run, the quiet stretch lasts
// 50 ms to 3 s, and a speech level follows it. Speech that simply ends has nothing after it.
func pieceDropout(audio DecodedAudio, env Envelope) bool {
	power := windowPowers(audio)
	runs := env.SpeechRuns()
	inRun := func(window int) bool {
		ms := window * envelopeWindowMS
		for _, r := range runs {
			if ms >= r[0] && ms < r[1] {
				return true
			}
		}
		return false
	}
	for i := 1; i < len(power); i++ {
		if power[i-1] < dropoutFromPower || power[i] >= dropoutToPower || !inRun(i-1) {
			continue
		}
		j := i
		for j < len(power) && power[j] < dropoutToPower {
			j++
		}
		n := j - i
		if n >= dropoutMinWindows && n <= dropoutMaxWindows && j < len(env.Levels) && env.Levels[j] >= speechLevel {
			return true
		}
		i = j
	}
	return false
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestCheckPiece' -count=1`
Expected: --- PASS: TestCheckPieceDropout, --- PASS: TestCheckPieceDropoutIgnoresWhatSpeechDoesNaturally; ok aurago/internal/storymaker

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/piececheck.go internal/storymaker/piececheck_test.go
git commit -m "storymaker: detect a dropout inside a synthesized piece"
```

### Task E2: detect clipping inside a synthesized piece

**Files:**
- Modify: `internal/storymaker/piececheck.go`
- Modify: `internal/storymaker/piececheck_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/piececheck_test.go`, replace:

```go
		}
	}
}
```

with:

```go
		}
	}
}

// pieceTestBurst overwrites count samples from startMS with v.
func pieceTestBurst(a DecodedAudio, startMS, count int, v float32) {
	for i := 0; i < count; i++ {
		a.Samples[startMS*pieceTestRate/1000+i] = v
	}
}

func TestCheckPieceClipping(t *testing.T) {
	hard := pieceTestSpeech(3, 0.3)
	for _, ms := range []int{200, 500, 800} {
		pieceTestBurst(hard, ms, 3, 1) // three samples at full scale, three times inside one second
	}
	flat := pieceTestSpeech(3, 0.3)
	for _, ms := range []int{200, 500, 800} {
		pieceTestBurst(flat, ms, 4, 0.25) // four equal samples: a flat top that was scaled down
	}
	for name, a := range map[string]DecodedAudio{"hard": hard, "flat": flat} {
		if got := pieceTestCheck(a); !reflect.DeepEqual(got, []string{"clipped"}) {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestCheckPieceClippingNeedsThreeRunsInOneSecond(t *testing.T) {
	two := pieceTestSpeech(3, 0.3)
	pieceTestBurst(two, 200, 3, 1)
	pieceTestBurst(two, 500, 3, 1)
	spread := pieceTestSpeech(3, 0.3)
	for _, ms := range []int{200, 1200, 2200} {
		pieceTestBurst(spread, ms, 3, 1) // three runs, but each a second apart
	}
	short := pieceTestSpeech(3, 0.3)
	for _, ms := range []int{200, 500, 800} {
		pieceTestBurst(short, ms, 2, 1) // two samples are a peak, not a run
	}
	quietFlat := pieceTestSpeech(3, 0.3)
	for _, ms := range []int{200, 500, 800} {
		pieceTestBurst(quietFlat, ms, 6, 0.01) // equal samples below 0.05 are silence, not a flat top
	}
	for name, a := range map[string]DecodedAudio{"two runs": two, "spread runs": spread, "short runs": short, "quiet flat": quietFlat} {
		if got := pieceTestCheck(a); len(got) != 0 {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestCheckPieceReportsBothFaultsSorted(t *testing.T) {
	a := pieceTestSpeech(3, 0.3)
	for _, ms := range []int{200, 500, 800} {
		pieceTestBurst(a, ms, 3, 1)
	}
	pieceTestZero(a, 1500, 100)
	if got := pieceTestCheck(a); !reflect.DeepEqual(got, []string{"clipped", "dropout"}) {
		t.Errorf("both: %v", got)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestCheckPiece' -count=1`
Expected: piececheck_test.go:79: hard: []  (also flat: [], and both: [dropout])

- [ ] **Step 3: Implement**

In `internal/storymaker/piececheck.go`, replace:

```go
package storymaker

const (
	// A dropout is a fall from at least dropoutFromPower to below dropoutToPower in one 10 ms step inside speech,
	// followed by dropoutMinWindows to dropoutMaxWindows of silence and then speech again.
```

with:

```go
package storymaker

import (
	"math"
	"sort"
)

const (
	// A dropout is a fall from at least dropoutFromPower to below dropoutToPower in one 10 ms step inside speech,
	// followed by dropoutMinWindows to dropoutMaxWindows of silence and then speech again.
```

In `internal/storymaker/piececheck.go`, replace:

```go
	dropoutToPower    = 1e-8                  // -80 dBFS
	dropoutMinWindows = 5                     // 50 ms
	dropoutMaxWindows = 300                   // 3 s
)

// CheckPiece reports the faults of one synthesized piece: "clipped", "dropout" (sorted, each at most once). env is the
// envelope of the same audio; its speech runs say where a fall counts as a hole and not as the end of the speech.
func CheckPiece(audio DecodedAudio, env Envelope) []string {
	var faults []string
	if pieceDropout(audio, env) {
		faults = append(faults, "dropout")
	}
```

with:

```go
	dropoutToPower    = 1e-8                  // -80 dBFS
	dropoutMinWindows = 5                     // 50 ms
	dropoutMaxWindows = 300                   // 3 s

	// A clipped piece has clipRunsPerSecond runs of clipRunHard samples at or above clipFull, or of clipRunFlat equal
	// samples at or above clipFlatFloor, inside one second.
	clipFull          = 0.999
	clipFlatFloor     = 0.05
	clipRunHard       = 3
	clipRunFlat       = 4
	clipRunsPerSecond = 3
)

// CheckPiece reports the faults of one synthesized piece: "clipped", "dropout" (sorted, each at most once). env is the
// envelope of the same audio; its speech runs say where a fall counts as a hole and not as the end of the speech.
func CheckPiece(audio DecodedAudio, env Envelope) []string {
	var faults []string
	if pieceClipped(audio) {
		faults = append(faults, "clipped")
	}
	if pieceDropout(audio, env) {
		faults = append(faults, "dropout")
	}
```

In `internal/storymaker/piececheck.go`, replace:

```go
	}
	return false
}
```

with:

```go
	}
	return false
}

// pieceClipped finds runs of consecutive samples of one channel: at least clipRunHard at full scale, or at least
// clipRunFlat equal ones above clipFlatFloor. A sample can belong to both kinds, so a run is a maximal stretch of
// flagged samples and counts once. The piece is clipped with clipRunsPerSecond runs inside one second.
func pieceClipped(a DecodedAudio) bool {
	if a.Rate <= 0 || a.Channels <= 0 {
		return false
	}
	frames := len(a.Samples) / a.Channels
	var starts []int
	for ch := 0; ch < a.Channels; ch++ {
		at := func(f int) float32 { return a.Samples[f*a.Channels+ch] }
		flagged := make([]bool, frames)
		for i := 0; i < frames; {
			j := i + 1
			for j < frames && at(j) == at(i) {
				j++
			}
			if j-i >= clipRunFlat && math.Abs(float64(at(i))) >= clipFlatFloor {
				for k := i; k < j; k++ {
					flagged[k] = true
				}
			}
			i = j
		}
		for i := 0; i < frames; {
			if math.Abs(float64(at(i))) < clipFull {
				i++
				continue
			}
			j := i
			for j < frames && math.Abs(float64(at(j))) >= clipFull {
				j++
			}
			if j-i >= clipRunHard {
				for k := i; k < j; k++ {
					flagged[k] = true
				}
			}
			i = j
		}
		for i := 0; i < frames; i++ {
			if flagged[i] && (i == 0 || !flagged[i-1]) {
				starts = append(starts, i)
			}
		}
	}
	sort.Ints(starts)
	for k := 0; k+clipRunsPerSecond <= len(starts); k++ {
		if starts[k+clipRunsPerSecond-1]-starts[k] < a.Rate {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestCheckPiece' -count=1`
Expected: --- PASS: TestCheckPieceClipping, TestCheckPieceClippingNeedsThreeRunsInOneSecond, TestCheckPieceReportsBothFaultsSorted; ok aurago/internal/storymaker

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/piececheck.go internal/storymaker/piececheck_test.go
git commit -m "storymaker: detect clipping inside a synthesized piece"
```

### Task E3: keep the piece faults in the envelope file

**Files:**
- Modify: `internal/storymaker/envelope.go`
- Modify: `internal/storymaker/envelope_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/envelope_test.go`, replace:

```go
		if _, err := ParseEnvelope([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
```

with:

```go
		if _, err := ParseEnvelope([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestEnvelopeFaultsRoundTrip(t *testing.T) {
	env := envelopeOfRuns(600, [2]int{0, 500})
	env.Faults = []string{"clipped", "dropout"}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Faults, []string{"clipped", "dropout"}) {
		t.Errorf("faults %v", got.Faults)
	}
	clean, _ := json.Marshal(envelopeOfRuns(600, [2]int{0, 500}))
	if strings.Contains(string(clean), "faults") {
		t.Errorf("a clean envelope names no faults: %s", clean)
	}
	// A workspace file is not trusted: only the two known faults survive, each once, sorted.
	odd := strings.Replace(string(data), `["clipped","dropout"]`, `["dropout","x","dropout","clipped"]`, 1)
	got, err = ParseEnvelope([]byte(odd))
	if err != nil || !reflect.DeepEqual(got.Faults, []string{"clipped", "dropout"}) {
		t.Errorf("odd faults %v, %v", got.Faults, err)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestEnvelopeFaultsRoundTrip' -count=1`
Expected: internal\storymaker\envelope_test.go:190:6: env.Faults undefined (type Envelope has no field or method Faults)

- [ ] **Step 3: Implement**

In `internal/storymaker/envelope.go`, replace:

```go
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)
```

with:

```go
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)
```

In `internal/storymaker/envelope.go`, replace:

```go
	// through them. Envelopes written before this field stay valid: they have no text.
	Text     string `json:"text,omitempty"`
	Language string `json:"language,omitempty"`
}

// levelByte is round((clamp(dBFS,-60,0)+60)*255/60), silence 0.
```

with:

```go
	// through them. Envelopes written before this field stay valid: they have no text.
	Text     string `json:"text,omitempty"`
	Language string `json:"language,omitempty"`
	// Faults names what CheckPiece found in the synthesized audio: "clipped" and "dropout", sorted. A piece with none
	// writes no key.
	Faults []string `json:"faults,omitempty"`
}

// levelByte is round((clamp(dBFS,-60,0)+60)*255/60), silence 0.
```

In `internal/storymaker/envelope.go`, replace:

```go
	if n := len(e.Levels); n < want-1 || n > want+1 {
		return Envelope{}, fmt.Errorf("storymaker: envelope has %d levels, want %d", n, want)
	}
	return e, nil
}
```

with:

```go
	if n := len(e.Levels); n < want-1 || n > want+1 {
		return Envelope{}, fmt.Errorf("storymaker: envelope has %d levels, want %d", n, want)
	}
	e.Faults = knownPieceFaults(e.Faults)
	return e, nil
}
```

In `internal/storymaker/envelope.go`, replace:

```go
	}
	return levels[i]
}
```

with:

```go
	}
	return levels[i]
}

// knownPieceFaults keeps the faults CheckPiece can name, each once, sorted: an env.json is an ordinary workspace file.
func knownPieceFaults(in []string) []string {
	var out []string
	for _, f := range []string{"clipped", "dropout"} {
		if slices.Contains(in, f) {
			out = append(out, f)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestEnvelopeFaultsRoundTrip' -count=1`
Expected: ok aurago/internal/storymaker (full package run; TestEnvelopeFaultsRoundTrip passes)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/envelope.go internal/storymaker/envelope_test.go
git commit -m "storymaker: keep the piece faults in the envelope file"
```

### Task E4: make a faulty narration piece once more and keep its faults

**Files:**
- Modify: `internal/server/storymaker_narrate.go`
- Modify: `internal/server/storymaker_narrate_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/server/storymaker_narrate_test.go`, replace:

```go
		t.Fatalf("usage.json = %+v", cur)
	}
}
```

with:

```go
		t.Fatalf("usage.json = %+v", cur)
	}
}

// narrateJobDropoutWAV is narrateJobWAV with 100 ms of digital silence in the middle of the tone.
func narrateJobDropoutWAV() []byte {
	data := narrateJobWAV()
	const rate = 24000
	zero := data[44+2*rate/2 : 44+2*(rate/2+rate/10)]
	for i := range zero {
		zero[i] = 0
	}
	return data
}

// narrateJobStoredFaults reads the faults of the stored envelope of one piece.
func narrateJobStoredFaults(t *testing.T, env *narrateJobEnv, text string) []string {
	t.Helper()
	hash := storymaker.PieceKey(storymaker.Voice{Provider: "phonon", Voice: phonon.DefaultVoiceForGender("de", ""), Language: "de"}, text)
	rc, _, err := env.rt.projects.OpenAudio(context.Background(), env.id, "narration", hash+".env.json")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := storymaker.ParseEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	return stored.Faults
}

func TestStorymakerNarrateRetriesAFaultyPieceOnce(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	var calls []string
	env.s.storymakerSynthOverride = func(_ context.Context, _ tools.TTSConfig, text string) ([]byte, string, error) {
		calls = append(calls, text)
		if len(calls) == 1 {
			return narrateJobDropoutWAV(), "wav", nil
		}
		return narrateJobWAV(), "wav", nil
	}
	job := env.run(t)
	if job.Status != render.StatusSucceeded || len(calls) != 2 {
		t.Fatalf("job = %+v, calls = %q", job, calls)
	}
	if faults := narrateJobStoredFaults(t, env, narrateJobFirst); len(faults) != 0 {
		t.Errorf("the clean second take is stored, but its faults are %v", faults)
	}
}

func TestStorymakerNarrateKeepsASecondTakeThatStillFails(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	var calls []string
	env.s.storymakerSynthOverride = func(_ context.Context, _ tools.TTSConfig, text string) ([]byte, string, error) {
		calls = append(calls, text)
		return narrateJobDropoutWAV(), "wav", nil
	}
	job := env.run(t)
	// One retry and no more, however faulty the takes are, and the job still succeeds.
	if job.Status != render.StatusSucceeded || len(calls) != 2 {
		t.Fatalf("job = %+v, calls = %q", job, calls)
	}
	if faults := narrateJobStoredFaults(t, env, narrateJobFirst); !slices.Equal(faults, []string{"dropout"}) {
		t.Errorf("stored faults = %v", faults)
	}
}

func TestStorymakerNarrateKeepsTheFirstTakeWhenTheRetryFails(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	calls := 0
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		calls++
		if calls == 1 {
			return narrateJobDropoutWAV(), "wav", nil
		}
		return nil, "", errors.New("provider down")
	}
	job := env.run(t)
	if job.Status != render.StatusSucceeded || calls != 2 {
		t.Fatalf("job = %+v, calls = %d", job, calls)
	}
	if faults := narrateJobStoredFaults(t, env, narrateJobFirst); !slices.Equal(faults, []string{"dropout"}) {
		t.Errorf("stored faults = %v", faults)
	}
}

func TestStorymakerNarrateChargesTheRetryToTheBudget(t *testing.T) {
	env := newNarrateUsageEnv(t)
	run := &storymakerNarrateRun{rt: env.rt}
	copied := *env.s.ConfigSnapshot()
	cfg := &copied
	cfg.Storymaker.Narration.MaxJobChars = 100
	cfg.Storymaker.Narration.MaxDailyChars = 1000
	text := strings.Repeat("a", 60)
	// A cloud voice is charged each time; the second take of 60 characters goes over the job cap of 100.
	if charge, err := storymakerReserve(run, cfg, "openai", text); err != nil || charge != 60 {
		t.Fatalf("first take: %d, %v", charge, err)
	}
	var refused *render.JobError
	if _, err := storymakerReserve(run, cfg, "openai", text); !errors.As(err, &refused) || refused.Code != render.CodeBudgetExceeded {
		t.Fatalf("second take: %v", err)
	}
	if charge, err := storymakerReserve(run, cfg, "phonon", text); err != nil || charge != 0 {
		t.Fatalf("a local voice is free: %d, %v", charge, err)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run '(?i)TestStorymakerNarrate(Retries|KeepsA|KeepsThe|Charges)' -count=1 -v`
Expected: internal\server\storymaker_narrate_test.go:576:20: undefined: storymakerReserve (the retry tests then fail with calls = 1)

- [ ] **Step 3: Implement**

In `internal/server/storymaker_narrate.go`, replace:

```go
	// written before the call, so a crash during the call cannot lose it.
	provider := target.block.Voice.Provider
	maxDaily := cfg.Storymaker.Narration.MaxDailyChars
	charge := 0
	if !storymaker.LocalVoiceProvider(provider) {
		charge = utf8.RuneCountInString(target.piece.Text)
		if maxJob := cfg.Storymaker.Narration.MaxJobChars; run.sent+charge > maxJob {
			return false, render.Fail(render.CodeBudgetExceeded, "This job would send %d characters to %s; one job may send %d.", run.sent+charge, provider, maxJob)
		}
		if err := run.rt.usage.reserve(charge, maxDaily); err != nil {
			return false, err
		}
		run.sent += charge
	}
	// One provider call. The stored format is the detected one, never the provider's claim.
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
```

with:

```go
	// written before the call, so a crash during the call cannot lose it.
	provider := target.block.Voice.Provider
	maxDaily := cfg.Storymaker.Narration.MaxDailyChars
	charge, err := storymakerReserve(run, cfg, provider, target.piece.Text)
	if err != nil {
		return false, err
	}
	// One provider call. The stored format is the detected one, never the provider's claim.
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
```

In `internal/server/storymaker_narrate.go`, replace:

```go
	if err != nil {
		return false, render.Fail(render.CodeNarrationFailed, "The voice provider %s returned audio that could not be decoded.", provider)
	}
	// The envelope names the words of the piece: a voice that changes later must still find them.
	env.Text = target.piece.Text
	env.Language = target.block.Voice.Language
```

with:

```go
	if err != nil {
		return false, render.Fail(render.CodeNarrationFailed, "The voice provider %s returned audio that could not be decoded.", provider)
	}
	// A faulty take is made once more, only here: a stored file that is verified is never made again. The second take
	// is kept even when it fails too, and its faults stay in the envelope.
	if len(env.Faults) > 0 {
		if data2, ext2, env2, ok := s.storymakerSecondTake(ctx, run, cfg, target); ok {
			data, ext, env = data2, ext2, env2
		}
	}
	// The envelope names the words of the piece: a voice that changes later must still find them.
	env.Text = target.piece.Text
	env.Language = target.block.Voice.Language
```

In `internal/server/storymaker_narrate.go`, replace:

```go
	return true, nil
}

// storymakerNarrateRead reads the story, the audio answer and the piece list of this moment.
func (s *Server) storymakerNarrateRead(ctx context.Context, run *storymakerNarrateRun) (storymaker.Story, storymaker.AudioAnswer, map[string]storymaker.PieceFile, error) {
	// The desktop service can be replaced while a job runs. Requests refresh the store the same way.
```

with:

```go
	return true, nil
}

// storymakerReserve charges the characters of one provider call to the job cap and the daily count. A local voice is
// free. It returns the charge, so the caller can release it when the provider sent nothing.
func storymakerReserve(run *storymakerNarrateRun, cfg *config.Config, provider, text string) (int, error) {
	if storymaker.LocalVoiceProvider(provider) {
		return 0, nil
	}
	charge := utf8.RuneCountInString(text)
	if maxJob := cfg.Storymaker.Narration.MaxJobChars; run.sent+charge > maxJob {
		return 0, render.Fail(render.CodeBudgetExceeded, "This job would send %d characters to %s; one job may send %d.", run.sent+charge, provider, maxJob)
	}
	if err := run.rt.usage.reserve(charge, cfg.Storymaker.Narration.MaxDailyChars); err != nil {
		return 0, err
	}
	run.sent += charge
	return charge, nil
}

// storymakerSecondTake synthesizes a piece once more after a fault, through the same budget. It reports false, and the
// caller keeps the first take, when the budget refuses, the provider fails or the answer does not decode.
func (s *Server) storymakerSecondTake(ctx context.Context, run *storymakerNarrateRun, cfg *config.Config, target *storymakerNarrateTarget) ([]byte, string, storymaker.Envelope, bool) {
	provider := target.block.Voice.Provider
	charge, err := storymakerReserve(run, cfg, provider, target.piece.Text)
	if err != nil {
		return nil, "", storymaker.Envelope{}, false
	}
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
	if err != nil {
		if charge > 0 && len(data) == 0 {
			run.rt.usage.release(charge, cfg.Storymaker.Narration.MaxDailyChars)
		}
		return nil, "", storymaker.Envelope{}, false
	}
	ext, env, err := storymakerDecodePiece(data)
	if err != nil {
		return nil, "", storymaker.Envelope{}, false
	}
	return data, ext, env, true
}

// storymakerNarrateRead reads the story, the audio answer and the piece list of this moment.
func (s *Server) storymakerNarrateRead(ctx context.Context, run *storymakerNarrateRun) (storymaker.Story, storymaker.AudioAnswer, map[string]storymaker.PieceFile, error) {
	// The desktop service can be replaced while a job runs. Requests refresh the store the same way.
```

In `internal/server/storymaker_narrate.go`, replace:

```go
	if env.DurationMS <= 0 {
		return "", storymaker.Envelope{}, errors.New("storymaker: the audio is empty")
	}
	return ext, env, nil
}
```

with:

```go
	if env.DurationMS <= 0 {
		return "", storymaker.Envelope{}, errors.New("storymaker: the audio is empty")
	}
	// The samples are held only for this check; the envelope keeps the verdict.
	env.Faults = storymaker.CheckPiece(decoded, env)
	return ext, env, nil
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run '(?i)TestStorymakerNarrate(Retries|KeepsA|KeepsThe|Charges)' -count=1 -v`
Expected: --- PASS: TestStorymakerNarrateRetriesAFaultyPieceOnce, KeepsASecondTakeThatStillFails, KeepsTheFirstTakeWhenTheRetryFails, ChargesTheRetryToTheBudget; ok aurago/internal/server

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_narrate.go internal/server/storymaker_narrate_test.go
git commit -m "storymaker: make a faulty narration piece once more and keep its faults"
```

### Task E5: measure a too loud bed and a bed gap on the pass-3 stem blocks

**Files:**
- Modify: `internal/storymaker/render/jobs.go`
- Create: `internal/storymaker/render/mixcheck.go`
- Create: `internal/storymaker/render/mixcheck_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/render/mixcheck_test.go`:

```go
package render

import (
	"reflect"
	"testing"
)

// mixCheckRun feeds a mixCheck the way pass 3 does: blocks of mixBlockFrames frames, stereo, each stem a constant per
// frame given by its function of the time in seconds.
func mixCheckRun(frames int64, narration, music func(sec float64) float64) *mixCheck {
	c := newMixCheck(frames)
	narr, mus, sfx := make([]float64, mixBlockFrames*mixChannels), make([]float64, mixBlockFrames*mixChannels), make([]float64, mixBlockFrames*mixChannels)
	for n0 := int64(0); n0 < frames; n0 += mixBlockFrames {
		f := int(min(mixBlockFrames, frames-n0))
		for i := 0; i < f; i++ {
			sec := float64(n0+int64(i)) / MixRate
			for ch := 0; ch < mixChannels; ch++ {
				narr[i*mixChannels+ch], mus[i*mixChannels+ch] = narration(sec), music(sec)
			}
		}
		c.observe(&mixBlock{frames: f, narration: narr[:f*mixChannels], music: mus[:f*mixChannels], sfx: sfx[:f*mixChannels]})
	}
	return c
}

func mixCheckConst(v float64) func(float64) float64 { return func(float64) float64 { return v } }

// mixCheckSpeech speaks (0.1, -20 dBFS) from 1 s to 9 s of a 12 s film, with pauses of 1 s.
func mixCheckSpeech(sec float64) float64 {
	if sec >= 1 && sec < 9 {
		return 0.1
	}
	return 0
}

func TestMixCheckBedTooLoud(t *testing.T) {
	// The bed at 0.05 is 6 dB under the speech, less than the 12 dB that the check demands.
	c := mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0.05))
	warnings, at := c.bedWarnings()
	if !reflect.DeepEqual(warnings, []string{"bed_too_loud"}) {
		t.Fatalf("warnings %v", warnings)
	}
	// The first window of 6 s that is half speech: it starts at 0 s, because 5 s of its 6 s are speech.
	if got := at["bed_too_loud"]; got != 0 {
		t.Errorf("at %v", got)
	}
}

func TestMixCheckBedTooLoudStaysQuietForAGoodBed(t *testing.T) {
	// 0.0125 is 18 dB under the speech, where the mix aims. A loud bed under silence is not judged: the narration
	// is active in under half of every window. Speech from 1 s to 3 s only.
	short := func(sec float64) float64 {
		if sec >= 1 && sec < 3 {
			return 0.1
		}
		return 0
	}
	for name, c := range map[string]*mixCheck{
		"good bed":        mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0.0125)),
		"loud bed, quiet": mixCheckRun(12*MixRate, short, mixCheckConst(0.05)),
		"no bed":          mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0)),
	} {
		if warnings, _ := c.bedWarnings(); len(warnings) != 0 {
			t.Errorf("%s: %v", name, warnings)
		}
	}
}

func TestMixCheckBedGap(t *testing.T) {
	// A steady bed that stops dead at 3 s of a 10 s film.
	stops := func(sec float64) float64 {
		if sec < 3 {
			return 0.05
		}
		return 0
	}
	warnings, at := mixCheckRun(10*MixRate, mixCheckConst(0), stops).bedWarnings()
	if !reflect.DeepEqual(warnings, []string{"bed_gap"}) {
		t.Fatalf("warnings %v", warnings)
	}
	if got := at["bed_gap"]; got < 2.99 || got > 3.01 {
		t.Errorf("at %v", got)
	}
}

func TestMixCheckBedGapIgnoresNormalMusic(t *testing.T) {
	// 0.5 s steady, then 14 dB down, over and over: it never holds steady for 1 s.
	pulsing := func(sec float64) float64 {
		if int(sec*2)%2 == 0 {
			return 0.05
		}
		return 0.01
	}
	// The planned fade-out: steady until 8.5 s, then a stop inside the last 1.5 s of the film.
	fadeOut := func(sec float64) float64 {
		if sec < 9 {
			return 0.05
		}
		return 0
	}
	// A slow fade is no step.
	fade := func(sec float64) float64 { return 0.05 * (1 - sec/10) }
	for name, c := range map[string]*mixCheck{
		"pulsing":   mixCheckRun(10*MixRate, mixCheckConst(0), pulsing),
		"fade out":  mixCheckRun(10*MixRate, mixCheckConst(0), fadeOut),
		"slow fade": mixCheckRun(10*MixRate, mixCheckConst(0), fade),
	} {
		if warnings, _ := c.bedWarnings(); len(warnings) != 0 {
			t.Errorf("%s: %v", name, warnings)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/render -run 'TestMixCheck' -count=1 -v`
Expected: mixcheck_test.go:13:7: undefined: mixCheck (build failed: newMixCheck, observe and bedWarnings do not exist)

- [ ] **Step 3: Implement**

In `internal/storymaker/render/jobs.go`, replace:

```go
	warnSubtitleFailed = "subtitle_failed"
	warnLibraryFailed  = "library_failed"
	warnMusicSilent    = "music_silent"
)

// JobWarning is a problem that did not fail the job.
```

with:

```go
	warnSubtitleFailed = "subtitle_failed"
	warnLibraryFailed  = "library_failed"
	warnMusicSilent    = "music_silent"
	warnBedTooLoud     = "bed_too_loud"
	warnBedGap         = "bed_gap"
)

// JobWarning is a problem that did not fail the job.
```

Create `internal/storymaker/render/mixcheck.go`:

```go
package render

import "math"

// The mix checks measure the pass-3 stem blocks, after all gains, in steps of 10 ms. They only detect: the mix is never
// changed, because the stems must sum to it exactly.
const (
	mixCheckStepFrames = MixRate / 100 // frames in one 10 ms step
	// bed_too_loud: a 6 s window in which the narration speaks at least half the time, and a bed that is less than 12 dB
	// under the narration while it speaks. The mix aims at 18 dB (12 LU plus 6 dB of ducking).
	mixCheckWindowSteps  = 600
	mixCheckBedMarginDB  = 12.0
	mixCheckSpeakingDB   = duckThresholdDB // a step with the narration above this level counts as speaking
	mixCheckMinWindowSec = 1               // a film shorter than this has no window
	// bed_gap: a bed that held steady for 1 s (its 10 ms levels within 3 dB) and then falls by 12 dB in one step.
	mixCheckSteadySteps  = 100
	mixCheckSteadySpread = 3.0
	mixCheckGapDropDB    = 12.0
	mixCheckBedFloorDB   = -90.0 // a bed below this is silence, not music that stopped
	mixCheckSilenceDB    = -120.0
)

// mixCheck collects the mean power of the narration and the music stem per 10 ms step.
type mixCheck struct {
	film             int64
	narration, music []float64
	frames           int     // frames in the open step
	nSum, mSum       float64 // sum of squares in the open step, both channels
}

func newMixCheck(film int64) *mixCheck { return &mixCheck{film: film} }

// observe adds one block of pass 3. The stems are interleaved stereo.
func (c *mixCheck) observe(b *mixBlock) {
	for i := 0; i < b.frames; i++ {
		for ch := 0; ch < mixChannels; ch++ {
			n, m := b.narration[i*mixChannels+ch], b.music[i*mixChannels+ch]
			c.nSum += n * n
			c.mSum += m * m
		}
		if c.frames++; c.frames == mixCheckStepFrames {
			per := float64(c.frames * mixChannels)
			c.narration = append(c.narration, c.nSum/per)
			c.music = append(c.music, c.mSum/per)
			c.frames, c.nSum, c.mSum = 0, 0, 0
		}
	}
}

func powerDB(p float64) float64 {
	if p <= 0 {
		return mixCheckSilenceDB
	}
	return max(10*math.Log10(p), mixCheckSilenceDB)
}

// bedWarnings returns the codes of the bed checks in the order bed_too_loud, bed_gap, and the second at which each
// began.
func (c *mixCheck) bedWarnings() ([]string, map[string]float64) {
	var codes []string
	at := map[string]float64{}
	if sec, ok := c.bedTooLoud(); ok {
		codes = append(codes, warnBedTooLoud)
		at[warnBedTooLoud] = sec
	}
	if sec, ok := c.bedGap(); ok {
		codes = append(codes, warnBedGap)
		at[warnBedGap] = sec
	}
	return codes, at
}

// bedTooLoud slides a window of 6 s (the whole film when it is shorter) over the steps. In a window where the narration
// speaks in at least half the steps, the RMS of the music over those steps is compared to the RMS of the narration.
func (c *mixCheck) bedTooLoud() (float64, bool) {
	steps := len(c.narration)
	window := min(mixCheckWindowSteps, steps)
	if steps < mixCheckMinWindowSec*100 {
		return 0, false
	}
	speaking := math.Pow(10, mixCheckSpeakingDB/10)
	// Prefix sums over the speaking steps: how many, and their narration and music power.
	count, narr, mus := make([]int, steps+1), make([]float64, steps+1), make([]float64, steps+1)
	for i := 0; i < steps; i++ {
		count[i+1], narr[i+1], mus[i+1] = count[i], narr[i], mus[i]
		if c.narration[i] > speaking {
			count[i+1]++
			narr[i+1] += c.narration[i]
			mus[i+1] += c.music[i]
		}
	}
	for s := 0; s+window <= steps; s++ {
		n := count[s+window] - count[s]
		if n == 0 || 2*n < window {
			continue
		}
		bed, voice := mus[s+window]-mus[s], narr[s+window]-narr[s]
		if bed > 0 && powerDB(bed/float64(n))-powerDB(voice/float64(n)) > -mixCheckBedMarginDB {
			return float64(s) / 100, true
		}
	}
	return 0, false
}

// bedGap finds a bed that was steady for 1 s and then fell by 12 dB in one step. The planned fade-out of the last
// musicFadeOutS is left out: the bed ends there on purpose.
func (c *mixCheck) bedGap() (float64, bool) {
	fadeOutFrom := c.film - int64(musicFadeOutS*MixRate)
	level := make([]float64, len(c.music))
	for i, p := range c.music {
		level[i] = powerDB(p)
	}
	for i := mixCheckSteadySteps; i < len(level); i++ {
		if int64(i)*mixCheckStepFrames >= fadeOutFrom {
			break
		}
		if level[i-1] < mixCheckBedFloorDB || level[i-1]-level[i] < mixCheckGapDropDB {
			continue
		}
		lo, hi := level[i-1], level[i-1]
		for _, v := range level[i-mixCheckSteadySteps : i] {
			lo, hi = min(lo, v), max(hi, v)
		}
		if hi-lo < mixCheckSteadySpread {
			return float64(i) / 100, true
		}
	}
	return 0, false
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/render -run 'TestMixCheck' -count=1 -v`
Expected: --- PASS: TestMixCheckBedTooLoud, TestMixCheckBedTooLoudStaysQuietForAGoodBed, TestMixCheckBedGap, TestMixCheckBedGapIgnoresNormalMusic; ok aurago/internal/storymaker/render

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/jobs.go internal/storymaker/render/mixcheck.go internal/storymaker/render/mixcheck_test.go
git commit -m "storymaker: measure a too loud bed and a bed gap on the pass-3 stem blocks"
```

### Task E6: report the mix warnings bed_too_loud, bed_gap and mix_clipped

**Files:**
- Modify: `internal/storymaker/render/jobs.go`
- Modify: `internal/storymaker/render/mix.go`
- Modify: `internal/storymaker/render/mix_test.go`
- Modify: `internal/storymaker/render/mixcheck.go`
- Modify: `internal/storymaker/render/mixcheck_test.go`
- Modify: `internal/storymaker/render/render.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/mix_test.go`, replace:

```go
	if last != 1 {
		t.Errorf("progress ended at %.3f", last)
	}
	if res.Clamps != 0 || res.BedDropped {
		t.Errorf("clamps %d, dropped %v", res.Clamps, res.BedDropped)
	}
	if res.VoiceGainDB["p|v"] != 0 {
		t.Errorf("a single voice got %.3f dB", res.VoiceGainDB["p|v"])
```

with:

```go
	if last != 1 {
		t.Errorf("progress ended at %.3f", last)
	}
	if res.Clamps != 0 || res.BedDropped || len(res.Warnings) != 0 {
		t.Errorf("clamps %d, dropped %v, warnings %v", res.Clamps, res.BedDropped, res.Warnings)
	}
	if res.VoiceGainDB["p|v"] != 0 {
		t.Errorf("a single voice got %.3f dB", res.VoiceGainDB["p|v"])
```

In `internal/storymaker/render/mixcheck_test.go`, replace:

```go
		"slow fade": mixCheckRun(10*MixRate, mixCheckConst(0), fade),
	} {
		if warnings, _ := c.bedWarnings(); len(warnings) != 0 {
			t.Errorf("%s: %v", name, warnings)
		}
	}
}
```

with:

```go
		"slow fade": mixCheckRun(10*MixRate, mixCheckConst(0), fade),
	} {
		if warnings, _ := c.bedWarnings(); len(warnings) != 0 {
			t.Errorf("%s: %v", name, warnings)
		}
	}
}

func TestMixWarningsAreInTheOrderOfTheSpec(t *testing.T) {
	// Speech from 1 s to 9 s over a bed at 6 dB under it that stops dead at 5 s: too loud, then a gap, and a clamp.
	bed := func(sec float64) float64 {
		if sec < 5 {
			return 0.05
		}
		return 0
	}
	c := mixCheckRun(12*MixRate, mixCheckSpeech, bed)
	warnings, at := mixWarnings(c, 3)
	if want := []string{"bed_too_loud", "bed_gap", "mix_clipped"}; !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warnings %v, want %v", warnings, want)
	}
	if at["bed_gap"] < 4.99 || at["bed_gap"] > 5.01 {
		t.Errorf("bed_gap at %v", at["bed_gap"])
	}
}

func TestMixWarningsClampsAloneGiveMixClipped(t *testing.T) {
	c := mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0.0125))
	if warnings, _ := mixWarnings(c, 1); !reflect.DeepEqual(warnings, []string{"mix_clipped"}) {
		t.Errorf("one clamp: %v", warnings)
	}
	if warnings, _ := mixWarnings(c, 0); len(warnings) != 0 {
		t.Errorf("no clamp: %v", warnings)
	}
}

func TestMixWarningMessages(t *testing.T) {
	for code, want := range map[string]string{
		warnBedTooLoud: "The music is loud under the narration (first at 12.5 s), so speech may be hard to follow.",
		warnBedGap:     "The music stops abruptly at 12.5 s.",
		warnMixClipped: "The final mix clipped at least one sample, so it may sound distorted.",
	} {
		if got := mixWarningMessage(code, 12.5); got != want {
			t.Errorf("%s: %q", code, got)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/render -run '(?i)(TestMixCheck|TestMixWarning|TestMixPassesApplyTheGains)' -count=1 -v`
Expected: .\mixcheck_test.go:123:18: undefined: mixWarnings

- [ ] **Step 3: Implement**

In `internal/storymaker/render/jobs.go`, replace:

```go
	warnMusicSilent    = "music_silent"
	warnBedTooLoud     = "bed_too_loud"
	warnBedGap         = "bed_gap"
)

// JobWarning is a problem that did not fail the job.
```

with:

```go
	warnMusicSilent    = "music_silent"
	warnBedTooLoud     = "bed_too_loud"
	warnBedGap         = "bed_gap"
	warnMixClipped     = "mix_clipped"
)

// JobWarning is a problem that did not fail the job.
```

In `internal/storymaker/render/mix.go`, replace:

```go
	StemPeak                                               [3]float64 // narration, music, sfx after the master gain, linear
	Clamps                                                 int64
	BedDropped                                             bool // the bed read silent
}

// mixPass selects what one run over the film computes.
```

with:

```go
	StemPeak                                               [3]float64 // narration, music, sfx after the master gain, linear
	Clamps                                                 int64
	BedDropped                                             bool // the bed read silent
	// Warnings are the codes of the mix checks (bed_too_loud, bed_gap, mix_clipped); WarningAt is the second of the
	// first finding of each bed check.
	Warnings  []string
	WarningAt map[string]float64
}

// mixPass selects what one run over the film computes.
```

In `internal/storymaker/render/mix.go`, replace:

```go
	if len(usableClips(in.SFX, in.Frames)) == 0 {
		stems.SFX = ""
	}
	writer, err := newStemWriter(stems)
	if err != nil {
		return res, err
```

with:

```go
	if len(usableClips(in.SFX, in.Frames)) == 0 {
		stems.SFX = ""
	}
	check := newMixCheck(working.Frames)
	writer, err := newStemWriter(stems)
	if err != nil {
		return res, err
```

In `internal/storymaker/render/mix.go`, replace:

```go
	defer func() { _ = writer.close() }()
	err = runMixPass(ctx, &working, mixPass{bedGainDB: levels.bedGainDB, masterDB: master}, func(b *mixBlock) error {
		prog.step(int64(b.frames))
		return writer.write(b)
	})
	if err != nil {
```

with:

```go
	defer func() { _ = writer.close() }()
	err = runMixPass(ctx, &working, mixPass{bedGainDB: levels.bedGainDB, masterDB: master}, func(b *mixBlock) error {
		prog.step(int64(b.frames))
		check.observe(b)
		return writer.write(b)
	})
	if err != nil {
```

In `internal/storymaker/render/mix.go`, replace:

```go
	res.BedGainDB, res.MasterGainDB = levels.bedGainDB, master
	res.BedDropped = levels.bedDropped
	res.StemPeak, res.Clamps = writer.peak, writer.clamps
	return res, nil
}
```

with:

```go
	res.BedGainDB, res.MasterGainDB = levels.bedGainDB, master
	res.BedDropped = levels.bedDropped
	res.StemPeak, res.Clamps = writer.peak, writer.clamps
	res.Warnings, res.WarningAt = mixWarnings(check, res.Clamps)
	return res, nil
}
```

In `internal/storymaker/render/mixcheck.go`, replace:

```go
package render

import "math"

// The mix checks measure the pass-3 stem blocks, after all gains, in steps of 10 ms. They only detect: the mix is never
// changed, because the stems must sum to it exactly.
const (
	mixCheckStepFrames = MixRate / 100 // frames in one 10 ms step
```

with:

```go
package render

import (
	"fmt"
	"math"
)

// The mix checks measure the pass-3 stem blocks, after all gains, in steps of 10 ms. They only detect: the mix is never
// changed, because the stems must sum to it exactly.
const (
	mixCheckStepFrames = MixRate / 100 // frames in one 10 ms step
```

In `internal/storymaker/render/mixcheck.go`, replace:

```go
			return float64(i) / 100, true
		}
	}
	return 0, false
}
```

with:

```go
			return float64(i) / 100, true
		}
	}
	return 0, false
}

// mixWarnings returns the warning codes of a mix in the order bed_too_loud, bed_gap, mix_clipped, each at most once, and
// the second of the first finding of each bed check.
func mixWarnings(c *mixCheck, clamps int64) ([]string, map[string]float64) {
	codes, at := c.bedWarnings()
	if clamps > 0 {
		codes = append(codes, warnMixClipped)
	}
	return codes, at
}

// mixWarningMessage is the one sentence of a mix warning for the export record. at is the second of the first finding.
func mixWarningMessage(code string, at float64) string {
	switch code {
	case warnBedTooLoud:
		return fmt.Sprintf("The music is loud under the narration (first at %.1f s), so speech may be hard to follow.", at)
	case warnBedGap:
		return fmt.Sprintf("The music stops abruptly at %.1f s.", at)
	case warnMixClipped:
		return "The final mix clipped at least one sample, so it may sound distorted."
	}
	return code
}
```

In `internal/storymaker/render/render.go`, replace:

```go
			if res.BedDropped {
				run.warn(warnMusicSilent, "The music track is silent, so the film has no music.")
			}
		}
	}
```

with:

```go
			if res.BedDropped {
				run.warn(warnMusicSilent, "The music track is silent, so the film has no music.")
			}
			for _, code := range res.Warnings {
				run.warn(code, mixWarningMessage(code, res.WarningAt[code]))
			}
		}
	}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/render -run '(?i)(TestMixCheck|TestMixWarning|TestMixPassesApplyTheGains)' -count=1 -v`
Expected: --- PASS: TestMixWarningsAreInTheOrderOfTheSpec, TestMixWarningsClampsAloneGiveMixClipped, TestMixWarningMessages, TestMixPassesApplyTheGains (clean mix, no warnings); ok aurago/internal/storymaker/render

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/jobs.go internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go internal/storymaker/render/mixcheck.go internal/storymaker/render/mixcheck_test.go internal/storymaker/render/render.go
git commit -m "storymaker: report the mix warnings bed_too_loud, bed_gap and mix_clipped"
```

### Task E7: carry the piece faults on the assembled audio block

**Files:**
- Modify: `internal/storymaker/envelope.go`
- Modify: `internal/storymaker/envelope_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/envelope_test.go`, replace:

```go
		t.Errorf("odd faults %v, %v", got.Faults, err)
	}
}
```

with:

```go
		t.Errorf("odd faults %v, %v", got.Faults, err)
	}
}

func TestAssembleBlockCollectsThePieceFaults(t *testing.T) {
	block := NarrationBlock{ID: "s1:n", Kind: BlockKindNarration, Beat: -1, Speaker: "narrator"}
	dropout := envelopeOfRuns(1000, [2]int{100, 800})
	dropout.Faults = []string{"dropout"}
	both := envelopeOfRuns(500, [2]int{50, 400})
	both.Faults = []string{"clipped", "dropout"}
	got := AssembleBlock(block, []AudioPieceInput{
		{Hash: "a", Ext: "wav", Env: dropout, Join: JoinSentence},
		{Hash: "b", Ext: "wav", Env: both},
	}, 30)
	if !reflect.DeepEqual(got.Faults, []string{"clipped", "dropout"}) {
		t.Errorf("faults %v", got.Faults)
	}
	if clean := AssembleBlock(block, []AudioPieceInput{{Hash: "a", Ext: "wav", Env: envelopeOfRuns(1000, [2]int{100, 800})}}, 30); clean.Faults != nil {
		t.Errorf("a clean block has faults %v", clean.Faults)
	}
	// The page never sees them: the answer of /narration and boot.audio_info stay as they were.
	data, err := json.Marshal(got)
	if err != nil || strings.Contains(string(data), "fault") {
		t.Errorf("block JSON %s, %v", data, err)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run 'TestAssembleBlockCollectsThePieceFaults' -count=1`
Expected: internal\storymaker\envelope_test.go:224:28: got.Faults undefined (type AudioBlock has no field or method Faults)

- [ ] **Step 3: Implement**

In `internal/storymaker/envelope.go`, replace:

```go
	Levels     Levels       `json:"levels"`
	Pieces     []AudioPiece `json:"pieces"`
	SpeechOK   bool         `json:"speech_ok"`
}

func joinPauseMS(join string) int {
```

with:

```go
	Levels     Levels       `json:"levels"`
	Pieces     []AudioPiece `json:"pieces"`
	SpeechOK   bool         `json:"speech_ok"`
	// Faults is the union of the faults of the pieces ("clipped", "dropout", sorted). The page never sees it.
	Faults []string `json:"-"`
}

func joinPauseMS(join string) int {
```

In `internal/storymaker/envelope.go`, replace:

```go
		out.Beat = &beat
	}
	at, pause := 0, 0
	for _, p := range pieces {
		at += pause
		in, end := 0, p.Env.DurationMS
		runs := p.Env.SpeechRuns()
```

with:

```go
		out.Beat = &beat
	}
	at, pause := 0, 0
	var faults []string
	for _, p := range pieces {
		faults = append(faults, p.Env.Faults...)
		at += pause
		in, end := 0, p.Env.DurationMS
		runs := p.Env.SpeechRuns()
```

In `internal/storymaker/envelope.go`, replace:

```go
		pause = joinPauseMS(p.Join)
	}
	out.DurationMS = at
	out.Levels = blockLevels(out, pieces, fps)
	return out
}
```

with:

```go
		pause = joinPauseMS(p.Join)
	}
	out.DurationMS = at
	out.Faults = knownPieceFaults(faults)
	out.Levels = blockLevels(out, pieces, fps)
	return out
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run 'TestAssembleBlockCollectsThePieceFaults' -count=1`
Expected: ok aurago/internal/storymaker (TestAssembleBlockCollectsThePieceFaults passes; faults stay out of the block JSON)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/envelope.go internal/storymaker/envelope_test.go
git commit -m "storymaker: carry the piece faults on the assembled audio block"
```

### Task E8: warn narration_dropout and narration_clipped once each on the export

**Files:**
- Modify: `internal/storymaker/render/jobs.go`
- Modify: `internal/storymaker/render/mixcheck.go`
- Create: `internal/storymaker/render/narrationfaults_test.go`
- Modify: `internal/storymaker/render/render.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/render/narrationfaults_test.go`:

```go
package render

import (
	"context"
	"io"
	"testing"

	"aurago/internal/storymaker"
)

// markPieceFaults rewrites the envelope of every seeded piece with the given faults.
func (fx *runFixture) markPieceFaults(t *testing.T, events []any, faults ...string) {
	t.Helper()
	ctx := context.Background()
	for _, e := range events {
		hash := e.(map[string]any)["ref"].(string)
		rc, _, err := fx.projects.OpenAudio(ctx, fx.projectID, "narration", hash+".env.json")
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		env, err := storymaker.ParseEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		env.Faults = faults
		if err := fx.projects.WriteEnvelope(ctx, fx.projectID, hash, env); err != nil {
			t.Fatal(err)
		}
	}
}

func countWarnings(job Job, code string) int {
	n := 0
	for _, w := range job.Warnings {
		if w.Code == code {
			n++
		}
	}
	return n
}

func TestRunWarnsOnceForEachNarrationFault(t *testing.T) {
	testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	fx.markPieceFaults(t, events, "clipped", "dropout")
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if countWarnings(job, "narration_dropout") != 1 || countWarnings(job, "narration_clipped") != 1 {
		t.Errorf("warnings = %+v", job.Warnings)
	}
}

func TestRunNarrationWithoutFaultsHasNoFaultWarning(t *testing.T) {
	testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded || len(job.Warnings) != 0 {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/render -run 'TestRunWarnsOnceForEachNarrationFault|TestRunNarrationWithoutFaults' -count=1 -v`
Expected: narrationfaults_test.go:58: warnings = []

- [ ] **Step 3: Implement**

In `internal/storymaker/render/jobs.go`, replace:

```go

// Warning codes of a job that still succeeded.
const (
	warnStemFailed     = "stem_failed"
	warnSubtitleFailed = "subtitle_failed"
	warnLibraryFailed  = "library_failed"
	warnMusicSilent    = "music_silent"
	warnBedTooLoud     = "bed_too_loud"
	warnBedGap         = "bed_gap"
	warnMixClipped     = "mix_clipped"
)

// JobWarning is a problem that did not fail the job.
```

with:

```go

// Warning codes of a job that still succeeded.
const (
	warnStemFailed       = "stem_failed"
	warnSubtitleFailed   = "subtitle_failed"
	warnLibraryFailed    = "library_failed"
	warnMusicSilent      = "music_silent"
	warnBedTooLoud       = "bed_too_loud"
	warnBedGap           = "bed_gap"
	warnMixClipped       = "mix_clipped"
	warnNarrationDropout = "narration_dropout"
	warnNarrationClipped = "narration_clipped"
)

// JobWarning is a problem that did not fail the job.
```

In `internal/storymaker/render/mixcheck.go`, replace:

```go
	}
	return code
}
```

with:

```go
	}
	return code
}

// narrationFaults lists the faults of the narration pieces the plan uses, as warning codes in the order
// narration_dropout, narration_clipped, each once.
func narrationFaults(plan *Plan) []string {
	var dropout, clipped bool
	for _, scene := range plan.AudioInfo.Narration {
		for _, block := range scene.Blocks {
			for _, fault := range block.Faults {
				dropout = dropout || fault == "dropout"
				clipped = clipped || fault == "clipped"
			}
		}
	}
	var codes []string
	if dropout {
		codes = append(codes, warnNarrationDropout)
	}
	if clipped {
		codes = append(codes, warnNarrationClipped)
	}
	return codes
}

// narrationFaultMessage is the one sentence of a narration fault warning.
func narrationFaultMessage(code string) string {
	if code == warnNarrationDropout {
		return "A synthesized narration piece has a dropout, a short hole inside speech. Generate the narration again."
	}
	return "A synthesized narration piece is clipped, so the voice may sound distorted. Generate the narration again."
}
```

In `internal/storymaker/render/render.go`, replace:

```go
			for _, code := range res.Warnings {
				run.warn(code, mixWarningMessage(code, res.WarningAt[code]))
			}
		}
	}
```

with:

```go
			for _, code := range res.Warnings {
				run.warn(code, mixWarningMessage(code, res.WarningAt[code]))
			}
			for _, code := range narrationFaults(run.plan) {
				run.warn(code, narrationFaultMessage(code))
			}
		}
	}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/render -run 'TestRunWarnsOnceForEachNarrationFault|TestRunNarrationWithoutFaults' -count=1 -v`
Expected: --- PASS: TestRunWarnsOnceForEachNarrationFault, --- PASS: TestRunNarrationWithoutFaultsHasNoFaultWarning (real FFmpeg)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/jobs.go internal/storymaker/render/mixcheck.go internal/storymaker/render/narrationfaults_test.go internal/storymaker/render/render.go
git commit -m "storymaker: warn narration_dropout and narration_clipped once each on the export"
```

### Task E9: show the narration faults on the agent job record

**Files:**
- Modify: `internal/server/storymaker_agent.go`
- Create: `internal/server/storymaker_agent_faults_test.go`
- Modify: `internal/storymaker/agentjob.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/storymaker_agent_faults_test.go`:

```go
package server

import (
	"context"
	"slices"
	"testing"

	"aurago/internal/tools"
)

func TestStorymakerAgentJobRecordsTheNarrationFaults(t *testing.T) {
	for name, tc := range map[string]struct {
		wav  []byte
		want []string
	}{
		"dropout": {narrateJobDropoutWAV(), []string{"dropout"}},
		"clean":   {narrateJobWAV(), nil},
	} {
		t.Run(name, func(t *testing.T) {
			env := newSMScriptEnv(t, smScriptNativeModel, &smScriptClient{turns: smScriptHappyTurns()})
			env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
				return tc.wav, "wav", nil
			}
			job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
			// A fault never changes the outcome.
			smScriptRequireReady(t, job)
			if !slices.Equal(job.Agent.NarrationFaults, tc.want) {
				t.Errorf("narration_faults = %v, want %v", job.Agent.NarrationFaults, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run 'TestStorymakerAgentJobRecordsTheNarrationFaults' -count=1 -v`
Expected: internal\server\storymaker_agent_faults_test.go:27:31: job.Agent.NarrationFaults undefined (type *storymaker.AgentJobInfo has no field or method NarrationFaults); with the field only: narration_faults = [], want [dropout]

- [ ] **Step 3: Implement**

In `internal/server/storymaker_agent.go`, replace:

```go
	adviceDone bool
	summary    string
	before     *storymaker.Story

	browserReady    bool // the browser for the compile check answered ready
	browserAttempts int  // Ensure calls of this job, at most storymakerBrowserMaxAttempts
```

with:

```go
	adviceDone bool
	summary    string
	before     *storymaker.Story
	// narrationFaults are the faults of the story's stored pieces, read after the narration step.
	narrationFaults []string

	browserReady    bool // the browser for the compile check answered ready
	browserAttempts int  // Ensure calls of this job, at most storymakerBrowserMaxAttempts
```

In `internal/server/storymaker_agent.go`, replace:

```go
	j.update(func(info *storymaker.AgentJobInfo) {
		info.Outcome, info.OpenChecks = outcome, checks
		info.Repairs = j.studio.Repairs()
		info.Phase, info.SceneID = "", ""
		info.ScenesTotal, info.ScenesDone, info.StoryETag = progress.ScenesTotal, progress.ScenesDone, progress.ETag
		info.DurationSeconds = in.Check.TotalSeconds
```

with:

```go
	j.update(func(info *storymaker.AgentJobInfo) {
		info.Outcome, info.OpenChecks = outcome, checks
		info.Repairs = j.studio.Repairs()
		info.NarrationFaults = j.narrationFaults
		info.Phase, info.SceneID = "", ""
		info.ScenesTotal, info.ScenesDone, info.StoryETag = progress.ScenesTotal, progress.ScenesDone, progress.ETag
		info.DurationSeconds = in.Check.TotalSeconds
```

In `internal/server/storymaker_agent.go`, replace:

```go
	return report, nil
}

// readStory reads the saved story. It returns nil when the story cannot be read or decoded.
func (j *storymakerAgentJob) readStory(ctx context.Context) *storymaker.Story {
	data, _, err := j.rt.projects.LoadStory(ctx, j.job.ProjectID)
```

with:

```go
	return report, nil
}

// readNarrationFaults collects the faults of the stored pieces of the story, "clipped" and "dropout", sorted. A story
// that cannot be read has none: the faults are a note on the job card and never fail a job.
func (j *storymakerAgentJob) readNarrationFaults(ctx context.Context) []string {
	story := j.readStory(ctx)
	if story == nil {
		return nil
	}
	storymaker.ApplyDefaults(story)
	storymaker.ApplyPhononVoices(story, j.rt.catalog)
	answer, err := j.rt.projects.AudioAnswerExact(ctx, j.job.ProjectID, story, nil)
	if err != nil {
		return nil
	}
	var all []string
	for _, scene := range answer.Narration {
		for _, block := range scene.Blocks {
			all = append(all, block.Faults...)
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// readStory reads the saved story. It returns nil when the story cannot be read or decoded.
func (j *storymakerAgentJob) readStory(ctx context.Context) *storymaker.Story {
	data, _, err := j.rt.projects.LoadStory(ctx, j.job.ProjectID)
```

In `internal/server/storymaker_agent.go`, replace:

```go
		j.logger.Warn("[Storymaker] The final agent check failed", "job_id", j.job.ID, "error", err)
		j.noteModelError(err)
	}
	return j.publishOutcome(final, storymaker.OutcomeInput{
		Progress:   open,
		Check:      report,
```

with:

```go
		j.logger.Warn("[Storymaker] The final agent check failed", "job_id", j.job.ID, "error", err)
		j.noteModelError(err)
	}
	j.narrationFaults = j.readNarrationFaults(final)
	return j.publishOutcome(final, storymaker.OutcomeInput{
		Progress:   open,
		Check:      report,
```

In `internal/storymaker/agentjob.go`, replace:

```go
	ChangedScenes   []string     `json:"changed_scenes,omitempty"`
	UnchangedScenes []string     `json:"unchanged_scenes,omitempty"`
	RemovedScenes   []string     `json:"removed_scenes,omitempty"`
}

// OutcomeInput is what a finished job knows. Narration is "", narration_unavailable, narration_failed,
```

with:

```go
	ChangedScenes   []string     `json:"changed_scenes,omitempty"`
	UnchangedScenes []string     `json:"unchanged_scenes,omitempty"`
	RemovedScenes   []string     `json:"removed_scenes,omitempty"`
	// NarrationFaults are the faults the synthesis checks found in the story's pieces ("clipped", "dropout", sorted). They
	// are shown on the job card and never change the outcome.
	NarrationFaults []string `json:"narration_faults,omitempty"`
}

// OutcomeInput is what a finished job knows. Narration is "", narration_unavailable, narration_failed,
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run 'TestStorymakerAgentJobRecordsTheNarrationFaults' -count=1 -v`
Expected: --- PASS: TestStorymakerAgentJobRecordsTheNarrationFaults/dropout, /clean; ok aurago/internal/server (all TestStorymakerAgent* pass)

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_agent.go internal/server/storymaker_agent_faults_test.go internal/storymaker/agentjob.go
git commit -m "storymaker: show the narration faults on the agent job record"
```

### Task E10: show the narration faults on the agent job card

**Files:**
- Modify: `ui/js/desktop/apps/storymaker-agent.js`
- Modify: `ui/lang/storymaker/cs.json`
- Modify: `ui/lang/storymaker/da.json`
- Modify: `ui/lang/storymaker/de.json`
- Modify: `ui/lang/storymaker/el.json`
- Modify: `ui/lang/storymaker/en.json`
- Modify: `ui/lang/storymaker/es.json`
- Modify: `ui/lang/storymaker/fr.json`
- Modify: `ui/lang/storymaker/hi.json`
- Modify: `ui/lang/storymaker/it.json`
- Modify: `ui/lang/storymaker/ja.json`
- Modify: `ui/lang/storymaker/nl.json`
- Modify: `ui/lang/storymaker/no.json`
- Modify: `ui/lang/storymaker/pl.json`
- Modify: `ui/lang/storymaker/pt.json`
- Modify: `ui/lang/storymaker/sv.json`
- Modify: `ui/lang/storymaker/zh.json`

- [ ] **Step 1: Make the change**

In `ui/js/desktop/apps/storymaker-agent.js`, replace:

```js
                const ids = job.agent && Array.isArray(job.agent[field]) ? job.agent[field] : [];
                if (ids.length) outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-scenes-list', list: field }, text: t(key, { scenes: sceneLabels(ids) }) }));
            });
            if (job.agent && job.agent.review && job.agent.review.sheet && inst.projectId) {
                outcomeCard.appendChild(el('a', {
                    class: 'stm-agent-review', href: inst.api.reviewSheetUrl(inst.projectId), target: '_blank', rel: 'noopener',
```

with:

```js
                const ids = job.agent && Array.isArray(job.agent[field]) ? job.agent[field] : [];
                if (ids.length) outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-scenes-list', list: field }, text: t(key, { scenes: sceneLabels(ids) }) }));
            });
            if (job.agent && Array.isArray(job.agent.narration_faults) && job.agent.narration_faults.length) {
                outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-narration-faults' }, text: t('storymaker.agent.narration_faults') }));
            }
            if (job.agent && job.agent.review && job.agent.review.sheet && inst.projectId) {
                outcomeCard.appendChild(el('a', {
                    class: 'stm-agent-review', href: inst.api.reviewSheetUrl(inst.projectId), target: '_blank', rel: 'noopener',
```

In `ui/lang/storymaker/cs.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Změněno: {scenes}",
  "storymaker.agent.unchanged_scenes": "Beze změny: {scenes}",
  "storymaker.agent.removed_scenes": "Odebráno: {scenes}",
  "storymaker.agent.restore_confirm": "Obnovit řeč? Změny příběhu od chvíle, kdy se řeč odebrala, se ztratí."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Změněno: {scenes}",
  "storymaker.agent.unchanged_scenes": "Beze změny: {scenes}",
  "storymaker.agent.removed_scenes": "Odebráno: {scenes}",
  "storymaker.agent.restore_confirm": "Obnovit řeč? Změny příběhu od chvíle, kdy se řeč odebrala, se ztratí.",
  "storymaker.agent.narration_faults": "Některé části syntetizované řeči mají výpadky nebo zkreslení. Vytvořte vyprávění znovu a zkuste je nahradit."
}
```

In `ui/lang/storymaker/da.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Ændret: {scenes}",
  "storymaker.agent.unchanged_scenes": "Uændret: {scenes}",
  "storymaker.agent.removed_scenes": "Fjernet: {scenes}",
  "storymaker.agent.restore_confirm": "Gendan tale? Ændringer i historien siden talen blev fjernet, går tabt."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Ændret: {scenes}",
  "storymaker.agent.unchanged_scenes": "Uændret: {scenes}",
  "storymaker.agent.removed_scenes": "Fjernet: {scenes}",
  "storymaker.agent.restore_confirm": "Gendan tale? Ændringer i historien siden talen blev fjernet, går tabt.",
  "storymaker.agent.narration_faults": "Dele af den syntetiserede tale har udfald eller forvrængning. Generer fortællingen igen for at prøve dem på ny."
}
```

In `ui/lang/storymaker/de.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Geändert: {scenes}",
  "storymaker.agent.unchanged_scenes": "Unverändert: {scenes}",
  "storymaker.agent.removed_scenes": "Entfernt: {scenes}",
  "storymaker.agent.restore_confirm": "Sprache wiederherstellen? Änderungen an der Geschichte seit dem Stummschalten gehen verloren."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Geändert: {scenes}",
  "storymaker.agent.unchanged_scenes": "Unverändert: {scenes}",
  "storymaker.agent.removed_scenes": "Entfernt: {scenes}",
  "storymaker.agent.restore_confirm": "Sprache wiederherstellen? Änderungen an der Geschichte seit dem Stummschalten gehen verloren.",
  "storymaker.agent.narration_faults": "In Teilen der erzeugten Sprache gibt es Aussetzer oder Verzerrungen. Erzeuge die Sprache erneut, um sie neu zu versuchen."
}
```

In `ui/lang/storymaker/el.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Τροποποιήθηκαν: {scenes}",
  "storymaker.agent.unchanged_scenes": "Δεν άλλαξαν: {scenes}",
  "storymaker.agent.removed_scenes": "Αφαιρέθηκαν: {scenes}",
  "storymaker.agent.restore_confirm": "Επαναφορά της ομιλίας; Οι αλλαγές στην ιστορία από τότε που αφαιρέθηκε η ομιλία θα χαθούν."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Τροποποιήθηκαν: {scenes}",
  "storymaker.agent.unchanged_scenes": "Δεν άλλαξαν: {scenes}",
  "storymaker.agent.removed_scenes": "Αφαιρέθηκαν: {scenes}",
  "storymaker.agent.restore_confirm": "Επαναφορά της ομιλίας; Οι αλλαγές στην ιστορία από τότε που αφαιρέθηκε η ομιλία θα χαθούν.",
  "storymaker.agent.narration_faults": "Μέρη της συνθετικής ομιλίας έχουν διακοπές ή παραμόρφωση. Δημιουργήστε ξανά την αφήγηση για να τα δοκιμάσετε εκ νέου."
}
```

In `ui/lang/storymaker/en.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Changed: {scenes}",
  "storymaker.agent.unchanged_scenes": "Unchanged: {scenes}",
  "storymaker.agent.removed_scenes": "Removed: {scenes}",
  "storymaker.agent.restore_confirm": "Restore the speech? Changes made to the story since it was made silent are lost."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Changed: {scenes}",
  "storymaker.agent.unchanged_scenes": "Unchanged: {scenes}",
  "storymaker.agent.removed_scenes": "Removed: {scenes}",
  "storymaker.agent.restore_confirm": "Restore the speech? Changes made to the story since it was made silent are lost.",
  "storymaker.agent.narration_faults": "Parts of the synthesized speech have dropouts or distortion. Generate the narration again to retry them."
}
```

In `ui/lang/storymaker/es.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Modificadas: {scenes}",
  "storymaker.agent.unchanged_scenes": "Sin cambios: {scenes}",
  "storymaker.agent.removed_scenes": "Eliminadas: {scenes}",
  "storymaker.agent.restore_confirm": "¿Restaurar el discurso? Se perderán los cambios hechos en la historia desde que se silenció."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Modificadas: {scenes}",
  "storymaker.agent.unchanged_scenes": "Sin cambios: {scenes}",
  "storymaker.agent.removed_scenes": "Eliminadas: {scenes}",
  "storymaker.agent.restore_confirm": "¿Restaurar el discurso? Se perderán los cambios hechos en la historia desde que se silenció.",
  "storymaker.agent.narration_faults": "Partes del habla sintetizada tienen cortes o distorsión. Genera la narración de nuevo para reintentarlas."
}
```

In `ui/lang/storymaker/fr.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Modifiées : {scenes}",
  "storymaker.agent.unchanged_scenes": "Inchangées : {scenes}",
  "storymaker.agent.removed_scenes": "Supprimées : {scenes}",
  "storymaker.agent.restore_confirm": "Rétablir la parole ? Les modifications apportées à l'histoire depuis qu'elle est muette seront perdues."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Modifiées : {scenes}",
  "storymaker.agent.unchanged_scenes": "Inchangées : {scenes}",
  "storymaker.agent.removed_scenes": "Supprimées : {scenes}",
  "storymaker.agent.restore_confirm": "Rétablir la parole ? Les modifications apportées à l'histoire depuis qu'elle est muette seront perdues.",
  "storymaker.agent.narration_faults": "Certains passages de la voix de synthèse présentent des coupures ou de la distorsion. Générez à nouveau la narration pour les refaire."
}
```

In `ui/lang/storymaker/hi.json`, replace:

```json
  "storymaker.agent.changed_scenes": "बदली गईं: {scenes}",
  "storymaker.agent.unchanged_scenes": "बिना बदलाव: {scenes}",
  "storymaker.agent.removed_scenes": "हटाई गईं: {scenes}",
  "storymaker.agent.restore_confirm": "भाषण वापस लाएँ? भाषण हटाए जाने के बाद कहानी में किए गए बदलाव खो जाएँगे।"
}
```

with:

```json
  "storymaker.agent.changed_scenes": "बदली गईं: {scenes}",
  "storymaker.agent.unchanged_scenes": "बिना बदलाव: {scenes}",
  "storymaker.agent.removed_scenes": "हटाई गईं: {scenes}",
  "storymaker.agent.restore_confirm": "भाषण वापस लाएँ? भाषण हटाए जाने के बाद कहानी में किए गए बदलाव खो जाएँगे।",
  "storymaker.agent.narration_faults": "संश्लेषित वाणी के कुछ हिस्सों में रुकावटें या विकृति है। उन्हें फिर से आज़माने के लिए नैरेशन दोबारा बनाएँ।"
}
```

In `ui/lang/storymaker/it.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Modificate: {scenes}",
  "storymaker.agent.unchanged_scenes": "Invariate: {scenes}",
  "storymaker.agent.removed_scenes": "Rimosse: {scenes}",
  "storymaker.agent.restore_confirm": "Ripristinare il parlato? Le modifiche alla storia fatte da quando è stata resa muta andranno perse."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Modificate: {scenes}",
  "storymaker.agent.unchanged_scenes": "Invariate: {scenes}",
  "storymaker.agent.removed_scenes": "Rimosse: {scenes}",
  "storymaker.agent.restore_confirm": "Ripristinare il parlato? Le modifiche alla storia fatte da quando è stata resa muta andranno perse.",
  "storymaker.agent.narration_faults": "Alcune parti del parlato sintetizzato hanno interruzioni o distorsione. Genera di nuovo la narrazione per riprovarle."
}
```

In `ui/lang/storymaker/ja.json`, replace:

```json
  "storymaker.agent.changed_scenes": "変更: {scenes}",
  "storymaker.agent.unchanged_scenes": "変更なし: {scenes}",
  "storymaker.agent.removed_scenes": "削除: {scenes}",
  "storymaker.agent.restore_confirm": "音声を元に戻しますか？ 無音にしてからの物語の変更は失われます。"
}
```

with:

```json
  "storymaker.agent.changed_scenes": "変更: {scenes}",
  "storymaker.agent.unchanged_scenes": "変更なし: {scenes}",
  "storymaker.agent.removed_scenes": "削除: {scenes}",
  "storymaker.agent.restore_confirm": "音声を元に戻しますか？ 無音にしてからの物語の変更は失われます。",
  "storymaker.agent.narration_faults": "合成音声の一部に音切れや歪みがあります。ナレーションを再生成して、やり直してください。"
}
```

In `ui/lang/storymaker/nl.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Gewijzigd: {scenes}",
  "storymaker.agent.unchanged_scenes": "Ongewijzigd: {scenes}",
  "storymaker.agent.removed_scenes": "Verwijderd: {scenes}",
  "storymaker.agent.restore_confirm": "Spraak herstellen? Wijzigingen in het verhaal sinds het stil is gemaakt, gaan verloren."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Gewijzigd: {scenes}",
  "storymaker.agent.unchanged_scenes": "Ongewijzigd: {scenes}",
  "storymaker.agent.removed_scenes": "Verwijderd: {scenes}",
  "storymaker.agent.restore_confirm": "Spraak herstellen? Wijzigingen in het verhaal sinds het stil is gemaakt, gaan verloren.",
  "storymaker.agent.narration_faults": "Delen van de gesynthetiseerde spraak hebben haperingen of vervorming. Genereer de vertelling opnieuw om ze opnieuw te proberen."
}
```

In `ui/lang/storymaker/no.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Endret: {scenes}",
  "storymaker.agent.unchanged_scenes": "Uendret: {scenes}",
  "storymaker.agent.removed_scenes": "Fjernet: {scenes}",
  "storymaker.agent.restore_confirm": "Gjenopprette tale? Endringer i historien siden talen ble fjernet, går tapt."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Endret: {scenes}",
  "storymaker.agent.unchanged_scenes": "Uendret: {scenes}",
  "storymaker.agent.removed_scenes": "Fjernet: {scenes}",
  "storymaker.agent.restore_confirm": "Gjenopprette tale? Endringer i historien siden talen ble fjernet, går tapt.",
  "storymaker.agent.narration_faults": "Deler av den syntetiserte talen har avbrudd eller forvrengning. Generer fortellingen på nytt for å prøve dem igjen."
}
```

In `ui/lang/storymaker/pl.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Zmienione: {scenes}",
  "storymaker.agent.unchanged_scenes": "Bez zmian: {scenes}",
  "storymaker.agent.removed_scenes": "Usunięte: {scenes}",
  "storymaker.agent.restore_confirm": "Przywrócić mowę? Zmiany w historii wprowadzone od wyciszenia zostaną utracone."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Zmienione: {scenes}",
  "storymaker.agent.unchanged_scenes": "Bez zmian: {scenes}",
  "storymaker.agent.removed_scenes": "Usunięte: {scenes}",
  "storymaker.agent.restore_confirm": "Przywrócić mowę? Zmiany w historii wprowadzone od wyciszenia zostaną utracone.",
  "storymaker.agent.narration_faults": "Fragmenty syntetyzowanej mowy mają przerwy lub zniekształcenia. Wygeneruj narrację ponownie, aby spróbować jeszcze raz."
}
```

In `ui/lang/storymaker/pt.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Alteradas: {scenes}",
  "storymaker.agent.unchanged_scenes": "Sem alterações: {scenes}",
  "storymaker.agent.removed_scenes": "Removidas: {scenes}",
  "storymaker.agent.restore_confirm": "Repor a fala? As alterações feitas à história desde que ficou sem fala serão perdidas."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Alteradas: {scenes}",
  "storymaker.agent.unchanged_scenes": "Sem alterações: {scenes}",
  "storymaker.agent.removed_scenes": "Removidas: {scenes}",
  "storymaker.agent.restore_confirm": "Repor a fala? As alterações feitas à história desde que ficou sem fala serão perdidas.",
  "storymaker.agent.narration_faults": "Partes da fala sintetizada têm cortes ou distorção. Gere a narração novamente para tentar de novo."
}
```

In `ui/lang/storymaker/sv.json`, replace:

```json
  "storymaker.agent.changed_scenes": "Ändrade: {scenes}",
  "storymaker.agent.unchanged_scenes": "Oförändrade: {scenes}",
  "storymaker.agent.removed_scenes": "Borttagna: {scenes}",
  "storymaker.agent.restore_confirm": "Återställ tal? Ändringar i berättelsen sedan den gjordes tyst går förlorade."
}
```

with:

```json
  "storymaker.agent.changed_scenes": "Ändrade: {scenes}",
  "storymaker.agent.unchanged_scenes": "Oförändrade: {scenes}",
  "storymaker.agent.removed_scenes": "Borttagna: {scenes}",
  "storymaker.agent.restore_confirm": "Återställ tal? Ändringar i berättelsen sedan den gjordes tyst går förlorade.",
  "storymaker.agent.narration_faults": "Delar av det syntetiserade talet har avbrott eller distorsion. Generera berättelsen igen för att försöka på nytt."
}
```

In `ui/lang/storymaker/zh.json`, replace:

```json
  "storymaker.agent.changed_scenes": "已修改：{scenes}",
  "storymaker.agent.unchanged_scenes": "未修改：{scenes}",
  "storymaker.agent.removed_scenes": "已删除：{scenes}",
  "storymaker.agent.restore_confirm": "恢复语音？自静音以来对故事所做的更改将会丢失。"
}
```

with:

```json
  "storymaker.agent.changed_scenes": "已修改：{scenes}",
  "storymaker.agent.unchanged_scenes": "未修改：{scenes}",
  "storymaker.agent.removed_scenes": "已删除：{scenes}",
  "storymaker.agent.restore_confirm": "恢复语音？自静音以来对故事所做的更改将会丢失。",
  "storymaker.agent.narration_faults": "合成语音的部分内容有中断或失真。请重新生成旁白以重试。"
}
```

- [ ] **Step 2: Run the test and see it pass**

Run: `go test ./ui -run '(?i)storymaker' -count=1`
Expected: ok aurago/ui (TestStorymakerAppUsesOnlyKeysItHas, TestStorymakerLocalesCoverAllSixteenLanguages, TestStorymakerAppTranslationsAreTranslated); npm run build:ui writes no bundle (storymaker-agent.js is not bundled), npm run check:ui passes

- [ ] **Step 3: Commit**

```bash
git add ui/js/desktop/apps/storymaker-agent.js ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "storymaker: show the narration faults on the agent job card"
```

### Task E11: a released take also gives its characters back to the job

**Files:**
- Modify: `internal/server/storymaker_narrate.go`
- Modify: `internal/server/storymaker_narrate_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/server/storymaker_narrate_test.go`, replace:

```go
		t.Fatalf("a local voice is free: %d, %v", charge, err)
	}
}
```

with:

```go
		t.Fatalf("a local voice is free: %d, %v", charge, err)
	}
}

func TestStorymakerNarrateReleaseGivesTheCharactersBackToTheJob(t *testing.T) {
	env := newNarrateUsageEnv(t)
	run := &storymakerNarrateRun{rt: env.rt}
	copied := *env.s.ConfigSnapshot()
	cfg := &copied
	cfg.Storymaker.Narration.MaxJobChars = 1000
	cfg.Storymaker.Narration.MaxDailyChars = 5000
	charge, err := storymakerReserve(run, cfg, "openai", strings.Repeat("a", 400))
	if err != nil || charge != 400 {
		t.Fatalf("reserve: %d, %v", charge, err)
	}
	// The call returned no bytes, so nothing was billed: the job count goes back with the daily count.
	storymakerRelease(run, cfg, charge)
	if run.sent != 0 {
		t.Fatalf("sent = %d after the release", run.sent)
	}
	if _, err := storymakerReserve(run, cfg, "openai", strings.Repeat("b", 700)); err != nil {
		t.Fatalf("a 700 character piece after a released 400: %v", err)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run '(?i)storymaker.*narrat' -count=1 -v`
Expected: storymaker_narrate_test.go: undefined: storymakerRelease (before the helper, run.sent stayed at 400 after the release, so a 700 character piece under a job cap of 1000 was refused)

- [ ] **Step 3: Implement**

In `internal/server/storymaker_narrate.go`, replace:

```go
	// The caps of a cloud voice. The job cap counts what this job already sent. The daily count is
	// written before the call, so a crash during the call cannot lose it.
	provider := target.block.Voice.Provider
	maxDaily := cfg.Storymaker.Narration.MaxDailyChars
	charge, err := storymakerReserve(run, cfg, provider, target.piece.Text)
	if err != nil {
		return false, err
```

with:

```go
	// The caps of a cloud voice. The job cap counts what this job already sent. The daily count is
	// written before the call, so a crash during the call cannot lose it.
	provider := target.block.Voice.Provider
	charge, err := storymakerReserve(run, cfg, provider, target.piece.Text)
	if err != nil {
		return false, err
```

In `internal/server/storymaker_narrate.go`, replace:

```go
		// No bytes: nothing to bill. Once the provider has answered it has billed, so a decode or
		// write failure below keeps the reservation.
		if charge > 0 && len(data) == 0 {
			run.rt.usage.release(charge, maxDaily)
		}
		if ctx.Err() != nil {
			return false, s.storymakerNarrateStopped()
```

with:

```go
		// No bytes: nothing to bill. Once the provider has answered it has billed, so a decode or
		// write failure below keeps the reservation.
		if charge > 0 && len(data) == 0 {
			storymakerRelease(run, cfg, charge)
		}
		if ctx.Err() != nil {
			return false, s.storymakerNarrateStopped()
```

In `internal/server/storymaker_narrate.go`, replace:

```go
	return true, nil
}

// storymakerReserve charges the characters of one provider call to the job cap and the daily count. A local voice is
// free. It returns the charge, so the caller can release it when the provider sent nothing.
func storymakerReserve(run *storymakerNarrateRun, cfg *config.Config, provider, text string) (int, error) {
```

with:

```go
	return true, nil
}

// storymakerRelease undoes a storymakerReserve whose provider call returned no bytes: the daily count and the count of
// this job both go back, so nothing that was never billed stays against the job cap.
func storymakerRelease(run *storymakerNarrateRun, cfg *config.Config, charge int) {
	if charge <= 0 {
		return
	}
	run.rt.usage.release(charge, cfg.Storymaker.Narration.MaxDailyChars)
	run.sent = max(0, run.sent-charge)
}

// storymakerReserve charges the characters of one provider call to the job cap and the daily count. A local voice is
// free. It returns the charge, so the caller can release it when the provider sent nothing.
func storymakerReserve(run *storymakerNarrateRun, cfg *config.Config, provider, text string) (int, error) {
```

In `internal/server/storymaker_narrate.go`, replace:

```go
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
	if err != nil {
		if charge > 0 && len(data) == 0 {
			run.rt.usage.release(charge, cfg.Storymaker.Narration.MaxDailyChars)
		}
		return nil, "", storymaker.Envelope{}, false
	}
```

with:

```go
	data, err := s.storymakerSynth(ctx, cfg, target.block.Voice, target.piece.Text)
	if err != nil {
		if charge > 0 && len(data) == 0 {
			storymakerRelease(run, cfg, charge)
		}
		return nil, "", storymaker.Envelope{}, false
	}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run '(?i)storymaker.*narrat' -count=1 -v`
Expected: --- PASS: TestStorymakerNarrateReleaseGivesTheCharactersBackToTheJob; ok aurago/internal/server

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_narrate.go internal/server/storymaker_narrate_test.go
git commit -m "storymaker: a released take also gives its characters back to the job"
```

