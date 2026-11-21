package gamemaker

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"strings"
)

const zzfxRuntimeJS = `// AuraGo bundled procedural Web Audio SFX helper (ZzFX-compatible, MIT)
let sharedAudioCtx = null;

export function getAudioContext() {
  if (typeof window === 'undefined') return null;
  const AudioCtx = window.AudioContext || window.webkitAudioContext;
  if (!AudioCtx) return null;
  if (!sharedAudioCtx || sharedAudioCtx.state === 'closed') {
    sharedAudioCtx = new AudioCtx();
  }
  if (sharedAudioCtx.state === 'suspended') {
    sharedAudioCtx.resume().catch(() => {});
  }
  return sharedAudioCtx;
}

export const sfxPresets = {
  jump:      [0.6, 0, 260, 0.01, 0.08, 0.14, 0, 1.4, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0.7, 0.03],
  pickup:    [0.5, 0, 520, 0.01, 0.06, 0.12, 0, 1.2, 0, 0, 240, 0.06, 0, 0, 0, 0, 0, 0.8, 0.02],
  coin:      [0.5, 0, 880, 0.01, 0.04, 0.16, 0, 1.0, 0, 0, 440, 0.05, 0, 0, 0, 0, 0, 0.8, 0.01],
  laser:     [0.5, 0, 720, 0.01, 0.05, 0.15, 2, 1.6, -14, 0, 0, 0, 0, 0.1, 0, 0, 0, 0.7, 0.02],
  shoot:     [0.6, 0, 420, 0.01, 0.04, 0.11, 3, 1.8, -12, 0, 0, 0, 0, 0.3, 0, 0, 0, 0.7, 0.01],
  explosion: [0.8, 0, 110, 0.01, 0.12, 0.42, 4, 2.2, -3, 0, 0, 0, 0, 1.2, 0, 0.2, 0.04, 0.6, 0.05],
  hit:       [0.7, 0, 180, 0.01, 0.03, 0.10, 3, 2.4, -8, 0, 0, 0, 0, 0.6, 0, 0.1, 0, 0.7, 0.01],
  damage:    [0.7, 0, 140, 0.01, 0.06, 0.18, 2, 2.0, -6, 0, -40, 0.06, 0, 0.4, 0, 0, 0, 0.65, 0.02],
  powerup:   [0.6, 0, 330, 0.02, 0.14, 0.26, 1, 1.3, 4, 0, 180, 0.08, 0.08, 0, 0, 0, 0, 0.8, 0.04],
  win:       [0.6, 0, 440, 0.02, 0.16, 0.34, 0, 1.2, 2, 0, 220, 0.10, 0.12, 0, 0, 0, 0, 0.85, 0.05],
  lose:      [0.6, 0, 280, 0.02, 0.14, 0.36, 2, 1.5, -4, 0, -90, 0.12, 0.14, 0.1, 0, 0, 0, 0.7, 0.04],
  click:     [0.4, 0, 620, 0.005, 0.015, 0.04, 1, 1.2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.7, 0.01],
  step:      [0.35, 0, 120, 0.005, 0.02, 0.06, 4, 1.8, -4, 0, 0, 0, 0, 0.8, 0, 0, 0, 0.6, 0.01],
  bounce:    [0.55, 0, 220, 0.01, 0.07, 0.16, 0, 1.1, 12, -0.5, 0, 0, 0, 0, 0, 0, 0, 0.8, 0.02],
  alarm:     [0.5, 0, 640, 0.01, 0.12, 0.18, 2, 1.4, 0, 0, 180, 0.08, 0.16, 0, 0, 0, 0, 0.75, 0.02],
};

export function zzfx(
  volume = 1, randomness = 0.05, frequency = 220, attack = 0, sustain = 0, release = 0.1,
  shape = 0, shapeCurve = 1, slide = 0, deltaSlide = 0, pitchJump = 0, pitchJumpTime = 0,
  repeatTime = 0, noise = 0, modulation = 0, bitCrush = 0, delay = 0, sustainVolume = 1,
  decay = 0, tremolo = 0
) {
  const ctx = getAudioContext();
  if (!ctx) return null;
  const sampleRate = ctx.sampleRate || 44100;
  const PI2 = Math.PI * 2;
  const sign = (v) => (v > 0 ? 1 : -1);
  let startSlide = slide * 500 * PI2 / sampleRate / sampleRate;
  let deltaSlideStep = deltaSlide * 500 * PI2 / sampleRate / sampleRate / sampleRate;
  let startFrequency = frequency * (1 + randomness * 2 * Math.random() - randomness) * PI2 / sampleRate;
  const pitchJumpFreq = pitchJump * PI2 / sampleRate;
  const pitchJumpSamples = pitchJumpTime * sampleRate | 0;
  const repeatSamples = repeatTime * sampleRate | 0;
  const attackSamples = Math.max(1, attack * sampleRate | 0);
  const decaySamples = decay * sampleRate | 0;
  const sustainSamples = sustain * sampleRate | 0;
  const releaseSamples = release * sampleRate | 0;
  const delaySamples = delay * sampleRate | 0;
  const length = Math.max(1, attackSamples + decaySamples + sustainSamples + releaseSamples + delaySamples);
  const b = new Float32Array(length);
  let t = 0, tm = 0, i = 0, j = 1, r = 0, c = 0, s = 0, f = startFrequency;
  for (; i < length; b[i++] = s * volume) {
    if (++c >= (bitCrush * 100 | 0)) {
      c = 0;
      s = shape ? (shape > 1 ? (shape > 2 ? (shape > 3 ? Math.sin((t % PI2) ** 3) : Math.max(Math.min(Math.tan(t), 1), -1)) : 1 - (2 * t / PI2 % 2 + 2) % 2) : 1 - 4 * Math.abs(Math.round(t / PI2) - t / PI2)) : Math.sin(t);
      s = (repeatSamples ? 1 - tremolo + tremolo * Math.sin(PI2 * i / repeatSamples) : 1) *
        sign(s) * Math.pow(Math.abs(s), shapeCurve) *
        (i < attackSamples ? i / attackSamples : (i < attackSamples + decaySamples ? 1 - ((i - attackSamples) / decaySamples) * (1 - sustainVolume) : (i < attackSamples + decaySamples + sustainSamples ? sustainVolume : (i < length - delaySamples ? (length - i - delaySamples) / releaseSamples * sustainVolume : 0))));
      s = delaySamples ? s / 2 + (delaySamples > i ? 0 : (i < length - delaySamples ? 1 : (length - i) / delaySamples) * b[i - delaySamples | 0] / 2) : s;
    }
    f += startSlide += deltaSlideStep;
    t += f * (1 + noise * (Math.random() * 2 - 1));
    tm += modulation * PI2 / sampleRate;
    t += modulation ? Math.sin(tm) * 0.04 : 0;
    if (pitchJumpSamples && ++j > pitchJumpSamples) {
      f += pitchJumpFreq;
      startFrequency += pitchJumpFreq;
      j = 0;
    }
    if (repeatSamples && !(++r % repeatSamples)) {
      f = startFrequency;
      startSlide = slide * 500 * PI2 / sampleRate / sampleRate;
      j = 1;
    }
  }
  try {
    const buffer = ctx.createBuffer(1, length, sampleRate);
    buffer.getChannelData(0).set(b);
    const source = ctx.createBufferSource();
    source.buffer = buffer;
    source.connect(ctx.destination);
    source.start();
    return source;
  } catch {
    return null;
  }
}

export function playSfx(nameOrParams, volume = 0.35) {
  const params = typeof nameOrParams === 'string'
    ? (sfxPresets[nameOrParams] || sfxPresets.pickup)
    : (Array.isArray(nameOrParams) ? nameOrParams : sfxPresets.pickup);
  const copy = params.slice();
  copy[0] = (typeof copy[0] === 'number' ? copy[0] : 0.5) * volume;
  return zzfx(...copy);
}
`

type sfxPresetSpec struct {
	name   string
	params []float64
}

var sfxPresetTable = []sfxPresetSpec{
	{name: "jump", params: []float64{0.6, 0, 260, 0.01, 0.08, 0.14, 0, 1.4, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0.7, 0.03}},
	{name: "pickup", params: []float64{0.5, 0, 520, 0.01, 0.06, 0.12, 0, 1.2, 0, 0, 240, 0.06, 0, 0, 0, 0, 0, 0.8, 0.02}},
	{name: "coin", params: []float64{0.5, 0, 880, 0.01, 0.04, 0.16, 0, 1.0, 0, 0, 440, 0.05, 0, 0, 0, 0, 0, 0.8, 0.01}},
	{name: "laser", params: []float64{0.5, 0, 720, 0.01, 0.05, 0.15, 2, 1.6, -14, 0, 0, 0, 0, 0.1, 0, 0, 0, 0.7, 0.02}},
	{name: "shoot", params: []float64{0.6, 0, 420, 0.01, 0.04, 0.11, 3, 1.8, -12, 0, 0, 0, 0, 0.3, 0, 0, 0, 0.7, 0.01}},
	{name: "explosion", params: []float64{0.8, 0, 110, 0.01, 0.12, 0.42, 4, 2.2, -3, 0, 0, 0, 0, 1.2, 0, 0.2, 0.04, 0.6, 0.05}},
	{name: "hit", params: []float64{0.7, 0, 180, 0.01, 0.03, 0.10, 3, 2.4, -8, 0, 0, 0, 0, 0.6, 0, 0.1, 0, 0.7, 0.01}},
	{name: "damage", params: []float64{0.7, 0, 140, 0.01, 0.06, 0.18, 2, 2.0, -6, 0, -40, 0.06, 0, 0.4, 0, 0, 0, 0.65, 0.02}},
	{name: "powerup", params: []float64{0.6, 0, 330, 0.02, 0.14, 0.26, 1, 1.3, 4, 0, 180, 0.08, 0.08, 0, 0, 0, 0, 0.8, 0.04}},
	{name: "win", params: []float64{0.6, 0, 440, 0.02, 0.16, 0.34, 0, 1.2, 2, 0, 220, 0.10, 0.12, 0, 0, 0, 0, 0.85, 0.05}},
	{name: "lose", params: []float64{0.6, 0, 280, 0.02, 0.14, 0.36, 2, 1.5, -4, 0, -90, 0.12, 0.14, 0.1, 0, 0, 0, 0.7, 0.04}},
	{name: "click", params: []float64{0.4, 0, 620, 0.005, 0.015, 0.04, 1, 1.2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.7, 0.01}},
	{name: "step", params: []float64{0.35, 0, 120, 0.005, 0.02, 0.06, 4, 1.8, -4, 0, 0, 0, 0, 0.8, 0, 0, 0, 0.6, 0.01}},
	{name: "bounce", params: []float64{0.55, 0, 220, 0.01, 0.07, 0.16, 0, 1.1, 12, -0.5, 0, 0, 0, 0, 0, 0, 0, 0.8, 0.02}},
	{name: "alarm", params: []float64{0.5, 0, 640, 0.01, 0.12, 0.18, 2, 1.4, 0, 0, 180, 0.08, 0.16, 0, 0, 0, 0, 0.75, 0.02}},
}

func selectSFXPreset(prompt string) sfxPresetSpec {
	lower := strings.ToLower(prompt)
	keywords := []struct {
		preset string
		terms  []string
	}{
		{"bounce", []string{"bounce", "boing", "rebound", "trampoline"}},
		{"jump", []string{"jump", "hop", "leap", "spring"}},
		{"coin", []string{"coin", "chime", "ring", "gold"}},
		{"pickup", []string{"pickup", "collect", "gem", "star", "orb", "item", "loot", "treasure", "key"}},
		{"laser", []string{"laser", "pew", "blaster", "ray", "beam", "zap"}},
		{"shoot", []string{"shoot", "shot", "fire", "gun", "bullet", "cannon", "bow", "arrow"}},
		{"explosion", []string{"explo", "boom", "blast", "bomb", "crash", "burst"}},
		{"damage", []string{"damage", "hurt", "oof", "injur"}},
		{"hit", []string{"hit", "punch", "slap", "strike", "smack", "slash", "sword", "impact"}},
		{"powerup", []string{"power", "upgrade", "level", "heal", "buff", "shield", "boost"}},
		{"win", []string{"win", "victory", "goal", "fanfare", "success", "clear", "complete"}},
		{"lose", []string{"lose", "game over", "gameover", "fail", "death", "die", "defeat"}},
		{"click", []string{"click", "button", "select", "menu", "hover", "confirm", "ui", "tap"}},
		{"step", []string{"step", "foot", "walk", "run"}},
		{"alarm", []string{"alarm", "alert", "siren", "warn", "danger", "beep"}},
	}
	chosen := "pickup"
	for _, entry := range keywords {
		matched := false
		for _, term := range entry.terms {
			if strings.Contains(lower, term) {
				matched = true
				break
			}
		}
		if matched {
			chosen = entry.preset
			break
		}
	}
	for _, spec := range sfxPresetTable {
		if spec.name == chosen {
			out := make([]float64, len(spec.params))
			copy(out, spec.params)
			return sfxPresetSpec{name: spec.name, params: out}
		}
	}
	out := make([]float64, len(sfxPresetTable[1].params))
	copy(out, sfxPresetTable[1].params)
	return sfxPresetSpec{name: "pickup", params: out}
}

// SynthesizeSFX deterministically renders a mono 16-bit PCM WAV sound effect
// matching the prompt and returns the encoded WAV bytes, matched preset name,
// and corresponding ZzFX parameter vector for optional runtime playback.
func SynthesizeSFX(prompt string, durationSeconds float64, seed int64) ([]byte, string, []float64) {
	spec := selectSFXPreset(prompt)
	if seed == 0 {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(prompt))))
		seed = int64(h.Sum64() & 0x7fffffff)
		if seed == 0 {
			seed = 1
		}
	}
	rngState := uint64(seed) | 1
	nextRand := func() float64 {
		rngState ^= rngState << 13
		rngState ^= rngState >> 7
		rngState ^= rngState << 17
		return float64(rngState&0xffffff)/float64(0xffffff)*2 - 1
	}

	params := spec.params
	// Subtle deterministic pitch variation so repeated prompts or distinct seeds feel alive.
	params[2] = math.Round(params[2]*(1+0.08*nextRand())*10) / 10
	if durationSeconds > 0 {
		if durationSeconds < 0.06 {
			durationSeconds = 0.06
		}
		if durationSeconds > 3.0 {
			durationSeconds = 3.0
		}
		natural := params[3] + params[4] + params[5] + params[18]
		if natural > 0 {
			scale := durationSeconds / natural
			params[3] = math.Round(params[3]*scale*1000) / 1000
			params[4] = math.Round(params[4]*scale*1000) / 1000
			params[5] = math.Round(params[5]*scale*1000) / 1000
			params[18] = math.Round(params[18]*scale*1000) / 1000
		}
	}

	const sampleRate = 22050
	const pi2 = math.Pi * 2
	volume := params[0]
	freq := params[2]
	attack := params[3]
	sustain := params[4]
	release := params[5]
	shape := int(params[6])
	shapeCurve := params[7]
	slide := params[8]
	deltaSlide := params[9]
	pitchJump := params[10]
	pitchJumpTime := params[11]
	repeatTime := params[12]
	noiseAmt := params[13]
	modulation := params[14]
	bitCrush := params[15]
	delaySec := params[16]
	sustainVolume := params[17]
	decay := params[18]

	startSlide := slide * 500 * pi2 / sampleRate / sampleRate
	deltaSlideStep := deltaSlide * 500 * pi2 / sampleRate / sampleRate / sampleRate
	startFreq := freq * pi2 / sampleRate
	pitchJumpFreq := pitchJump * pi2 / sampleRate
	pitchJumpSamples := int(pitchJumpTime * sampleRate)
	repeatSamples := int(repeatTime * sampleRate)
	attackSamples := max(1, int(attack*sampleRate))
	decaySamples := int(decay * sampleRate)
	sustainSamples := int(sustain * sampleRate)
	releaseSamples := max(1, int(release*sampleRate))
	delaySamples := int(delaySec * sampleRate)
	totalSamples := max(1, attackSamples+decaySamples+sustainSamples+releaseSamples+delaySamples)

	buf := make([]float64, totalSamples)
	var t, tm, sample float64
	f := startFreq
	j, r, c := 1, 0, 0
	crushStep := int(bitCrush * 100)

	for i := 0; i < totalSamples; i++ {
		if c >= crushStep {
			c = 0
			switch {
			case shape == 0:
				sample = math.Sin(t)
			case shape == 1:
				sample = 1 - 4*math.Abs(math.Round(t/pi2)-t/pi2)
			case shape == 2:
				mod := math.Mod(2*t/pi2, 2)
				if mod < 0 {
					mod += 2
				}
				sample = 1 - mod
			case shape == 3:
				tanVal := math.Tan(t)
				if tanVal > 1 {
					tanVal = 1
				} else if tanVal < -1 {
					tanVal = -1
				}
				sample = tanVal
			default:
				rem := math.Mod(t, pi2)
				sample = math.Sin(rem * rem * rem)
			}
			sign := 1.0
			if sample < 0 {
				sign = -1.0
			}
			var env float64
			switch {
			case i < attackSamples:
				env = float64(i) / float64(attackSamples)
			case i < attackSamples+decaySamples:
				env = 1 - (float64(i-attackSamples)/float64(decaySamples))*(1-sustainVolume)
			case i < attackSamples+decaySamples+sustainSamples:
				env = sustainVolume
			case i < totalSamples-delaySamples:
				env = float64(totalSamples-i-delaySamples) / float64(releaseSamples) * sustainVolume
			default:
				env = 0
			}
			sample = sign * math.Pow(math.Abs(sample), shapeCurve) * env
			if delaySamples > 0 {
				delayed := 0.0
				if i >= delaySamples {
					tail := 1.0
					if i >= totalSamples-delaySamples {
						tail = float64(totalSamples-i) / float64(delaySamples)
					}
					delayed = tail * buf[i-delaySamples] / 2
				}
				sample = sample/2 + delayed
			}
		}
		c++
		buf[i] = sample * volume
		startSlide += deltaSlideStep
		f += startSlide
		t += f * (1 + noiseAmt*nextRand())
		tm += modulation * pi2 / sampleRate
		if modulation != 0 {
			t += math.Sin(tm) * 0.04
		}
		j++
		if pitchJumpSamples > 0 && j > pitchJumpSamples {
			f += pitchJumpFreq
			startFreq += pitchJumpFreq
			j = 0
		}
		r++
		if repeatSamples > 0 && r%repeatSamples == 0 {
			f = startFreq
			startSlide = slide * 500 * pi2 / sampleRate / sampleRate
			j = 1
		}
	}

	dataSize := totalSamples * 2
	wav := make([]byte, 44+dataSize)
	copy(wav[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(36+dataSize))
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(wav[22:24], 1) // Mono
	binary.LittleEndian.PutUint32(wav[24:28], sampleRate)
	binary.LittleEndian.PutUint32(wav[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(wav[32:34], 2)
	binary.LittleEndian.PutUint16(wav[34:36], 16)
	copy(wav[36:40], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(dataSize))
	for i, v := range buf {
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		binary.LittleEndian.PutUint16(wav[44+i*2:], uint16(int16(v*32767)))
	}
	return wav, spec.name, params
}
