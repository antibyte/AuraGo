package deployer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPhononGraniteBundleImageKeysRoundTripAndValidate(t *testing.T) {
	const prefix = "ghcr.io/antibyte/"
	digest := "@sha256:" + strings.Repeat("a", 64)
	want := map[string]string{
		"phonon":                prefix + "s2s-tts-phonon" + digest,
		"granite_speech_cpu":    prefix + "s2s-asr-granite-speech-cpu" + digest,
		"granite_speech_cuda":   prefix + "s2s-asr-granite-speech-cuda" + digest,
		"granite_speech_vulkan": prefix + "s2s-asr-granite-speech-vulkan" + digest,
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var images ImageSet
	if err := json.Unmarshal(raw, &images); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := json.Marshal(images)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(roundTrip, &got); err != nil {
		t.Fatal(err)
	}
	for key, value := range got {
		if value == "" {
			delete(got, key)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("image keys lost: got %v, want %v", got, want)
	}
	for key, ref := range want {
		t.Run(key, func(t *testing.T) {
			if imageByKey(images, key) != ref {
				t.Fatal("image key was not resolved")
			}
			manifest := validV2Manifest()
			manifest.Images.Phonon = images.Phonon
			manifest.Images.GraniteSpeechCPU = images.GraniteSpeechCPU
			manifest.Images.GraniteSpeechCUDA = images.GraniteSpeechCUDA
			manifest.Images.GraniteSpeechVulkan = images.GraniteSpeechVulkan
			manifest.Runtimes[0].ImageKey = key
			manifest.Runtimes[0].Image = ref
			if err := validateManifest(manifest); err != nil {
				t.Fatalf("new image rejected: %v", err)
			}
			manifest.Runtimes[0].Image = strings.Replace(ref, strings.Repeat("a", 64), strings.Repeat("b", 64), 1)
			if err := validateManifest(manifest); err == nil {
				t.Fatal("mismatching runtime digest accepted")
			}
		})
	}
	if len(uniqueImages(images)) != len(want) {
		t.Fatal("new images omitted from validation")
	}
}
