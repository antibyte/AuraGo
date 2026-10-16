package videostudio

import (
	"reflect"
	"testing"
)

func TestUpgradeVersionTwoToThree(t *testing.T) {
	if CurrentVersion != 3 {
		t.Fatalf("CurrentVersion = %d, want 3", CurrentVersion)
	}
	v2 := validProject()
	v2.Version = 2
	v2.Assets[0].DurationUs = 10_000_000
	v2.Tracks[0].Clips[0].Label = "teal"
	want := v2
	got, err := Upgrade(v2)
	if err != nil || got.Version != 3 {
		t.Fatalf("Upgrade(v2) = version %d, %v; want 3", got.Version, err)
	}
	if v2.Version != 2 {
		t.Fatal("Upgrade modified its input")
	}
	got.Version = 2
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the 2 -> 3 step changed data:\n got  %+v\n want %+v", got, want)
	}
	got.Version = 3
	if err := Validate(got); err != nil {
		t.Fatalf("upgraded project invalid: %v", err)
	}
	if again, err := Upgrade(got); err != nil || again.Version != 3 {
		t.Fatalf("Upgrade(v3) = %d, %v", again.Version, err)
	}
}

func TestUpgradeVersionOneReachesThree(t *testing.T) {
	v1 := validProject()
	v1.Version = 1
	got, err := Upgrade(v1)
	if err != nil || got.Version != 3 || got.Assets[0].DurationUs != 10_000_000 {
		t.Fatalf("Upgrade(v1) = version %d, duration %d us, %v; want 3 with the 30 fps backfill", got.Version, got.Assets[0].DurationUs, err)
	}
	if v1.Version != 1 || v1.Assets[0].DurationUs != 0 {
		t.Fatal("Upgrade modified its input")
	}
	if err := Validate(got); err != nil {
		t.Fatalf("upgraded project invalid: %v", err)
	}
	ntsc := v1
	ntsc.NTSC = true
	if _, err := Upgrade(ntsc); err == nil {
		t.Fatal("an NTSC version 1 project was upgraded")
	}
}

func TestValidateRejectsVersionTwo(t *testing.T) {
	for _, version := range []int{0, 1, 2, 4} {
		project := validProject()
		project.Version = version
		if err := Validate(project); err == nil {
			t.Fatalf("Validate accepted version %d", version)
		}
	}
	for _, version := range []int{-1, 0, 4} {
		if _, err := Upgrade(Project{Version: version}); err == nil {
			t.Fatalf("Upgrade accepted version %d", version)
		}
	}
	project := validProject()
	project.Version = 3
	if err := Validate(project); err != nil {
		t.Fatalf("Validate rejected version 3: %v", err)
	}
}
