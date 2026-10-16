package videostudio

import "testing"

func TestFrameRateOptionsAreExactlyTheAcceptedRates(t *testing.T) {
	for _, option := range FrameRateOptions() {
		project := folderTestProject()
		project.FPS, project.NTSC = option.FPS, option.NTSC
		if err := Validate(project); err != nil {
			t.Fatalf("listed rate %+v rejected: %v", option, err)
		}
	}
	for _, fps := range []int{23, 26, 48} {
		project := folderTestProject()
		project.FPS = fps
		if err := Validate(project); err == nil {
			t.Fatalf("%d fps accepted, but it is not in FrameRateOptions", fps)
		}
	}
	for _, fps := range []int{25, 50} {
		project := folderTestProject()
		project.FPS, project.NTSC = fps, true
		if err := Validate(project); err == nil {
			t.Fatalf("NTSC %d fps accepted, but FrameRateOptions has no NTSC variant of it", fps)
		}
	}
	options := FrameRateOptions()
	options[0].FPS = 999
	if FrameRateOptions()[0].FPS == 999 {
		t.Fatal("FrameRateOptions returned its own table instead of a copy")
	}
}

func TestBackfillDurationUsIsTheOnlyLegacyDerivation(t *testing.T) {
	legacy := BackfillDurationUs(Asset{Kind: AssetVideo, DurationFrames: 60})
	if legacy.DurationUs != 2_000_000 {
		t.Fatalf("legacy 60 frames at 30 fps = %d us, want 2000000", legacy.DurationUs)
	}
	if image := BackfillDurationUs(Asset{Kind: AssetImage, DurationFrames: 150}); image.DurationUs != 0 {
		t.Fatalf("images must not get a duration: %d", image.DurationUs)
	}
	if kept := BackfillDurationUs(Asset{Kind: AssetAudio, DurationFrames: 60, DurationUs: 1_500_000}); kept.DurationUs != 1_500_000 {
		t.Fatalf("an existing duration must stay: %d", kept.DurationUs)
	}
}
