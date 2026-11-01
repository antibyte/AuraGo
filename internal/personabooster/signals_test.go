package personabooster

import (
	"math"
	"slices"
	"testing"
	"time"
)

func TestPriorityTable(t *testing.T) {
	for trigger, want := range map[string]int{
		CueDistress: 100, CueWantsCalm: 100, SignalTextStop: 100,
		CueProvocation: 70, CueCriticism: 70, SignalTextSwearing: 70, SignalTextShouting: 70,
		SignalMoodFrustratedStreak: 50, SignalMoodPlayfulHigh: 50, SignalMoodConcernedLoad: 50, SignalMoodCuriousCalm: 50,
		CuePraise: 40, CueBanter: 40, CueDeepTopic: 40, CueConspiracyBait: 40, CueRulesPressure: 40,
		SignalNight: 40, SignalLongAbsence: 40, SignalLongSession: 40,
		TriggerSpontaneous: 20, "signal:unknown": 0,
	} {
		if got := Priority(trigger); got != want {
			t.Errorf("Priority(%q) = %d, want %d", trigger, got, want)
		}
	}
}

func TestDetectTextSignals(t *testing.T) {
	cases := []struct {
		msg  string
		want []string
	}{
		{"WARUM GEHT DAS NICHT ENDLICH", []string{SignalTextShouting}},
		{"ok!!!", []string{SignalTextShouting}},
		{"NASA API", nil},
		{"Verdammt, schon wieder kaputt", []string{SignalTextSwearing}},
		{"this is fucking broken", []string{SignalTextSwearing}},
		{"Scheißding stürzt ab", []string{SignalTextSwearing}},
		{"Mistral is a model", nil},
		{"Hör auf damit", []string{SignalTextStop}},
		{"Hör auf mich, das ist wichtig", nil},
		{"please calm down", []string{SignalTextStop}},
		{"lass das!", []string{SignalTextStop}},
		{"lass das Programm laufen", nil},
		{"lass das zu", nil},
		{"how do I stop it from crashing", nil},
		{"it would be nice to have tests", nil},
		{"", nil},
	}
	for _, tc := range cases {
		if got := DetectTextSignals(tc.msg); !slices.Equal(got, tc.want) {
			t.Errorf("DetectTextSignals(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

func TestNormalizeCue(t *testing.T) {
	cases := []struct {
		raw        string
		confidence float64
		want       string
		ok         bool
	}{
		{"criticism", 0.8, CueCriticism, true},
		{" Cue:Praise ", 0.5, CuePraise, true},
		{"criticism", 0.49, "", false},
		{"none", 0.9, "", false},
		{"tantrum", 0.9, "", false},
		{"criticism", math.NaN(), "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeCue(tc.raw, tc.confidence)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NormalizeCue(%q, %v) = %q, %v", tc.raw, tc.confidence, got, ok)
		}
	}
}

func TestMoodAndTimeSignals(t *testing.T) {
	cases := []struct {
		in   MoodInput
		want []string
	}{
		{MoodInput{Mood: "frustrated", Friction: 0.6}, []string{SignalMoodFrustratedStreak}},
		{MoodInput{Mood: "frustrated", ConsecutiveToolErrors: 2}, []string{SignalMoodFrustratedStreak}},
		{MoodInput{Mood: "frustrated", Friction: 0.2, ConsecutiveToolErrors: 1}, nil},
		{MoodInput{Mood: "creative", Arousal: 0.7}, []string{SignalMoodPlayfulHigh}},
		{MoodInput{Mood: "cautious", Load: 0.5}, []string{SignalMoodConcernedLoad}},
		{MoodInput{Mood: "analytical", Arousal: 0.3}, []string{SignalMoodCuriousCalm}},
		{MoodInput{Mood: "curious", Arousal: 0.5}, nil},
	}
	for _, tc := range cases {
		if got := MoodSignals(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("MoodSignals(%+v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	night := time.Date(2026, 11, 1, 3, 0, 0, 0, time.Local)
	if got := TimeSignals(night, 30, true); !slices.Equal(got, []string{SignalNight, SignalLongAbsence, SignalLongSession}) {
		t.Errorf("TimeSignals(night) = %v", got)
	}
	if got := TimeSignals(night.Add(2*time.Hour), 23.9, false); len(got) != 0 {
		t.Errorf("TimeSignals(05:00) = %v, want none", got)
	}
}
