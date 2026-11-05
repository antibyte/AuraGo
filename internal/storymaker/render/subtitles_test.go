package render

import "testing"

func subtitleTestCaptions() []reportCaption {
	return []reportCaption{
		{StartMS: 500, EndMS: 2600, Lines: []string{"a --> b", "<b>&</b>"}, SceneID: "s1", BlockID: "s1:n"},
		// 61001 ms = 60000 + 1000 + 1. 3723004 ms = 3600000 + 120000 + 3000 + 4.
		{StartMS: 61001, EndMS: 3723004, Lines: []string{"  zwei\t\t  Worte\x01 ", "a\xc2\xa0b"}, SceneID: "s1", BlockID: "s1:n"},
		{StartMS: 3723004, EndMS: 3723500, Lines: []string{" \x01 "}, SceneID: "s1", BlockID: "s1:n"},
	}
}

// SubRip defines no entities. & stays as it is, and < > become U+2039 and U+203A, so no player reads a tag or an arrow.
func TestSubtitlesSRTKeepsAmpersandsAndBreaksTags(t *testing.T) {
	const lt, gt = "\xe2\x80\xb9", "\xe2\x80\xba"
	want := "1\n00:00:00,500 --> 00:00:02,600\na --" + gt + " b\n" + lt + "b" + gt + "&" + lt + "/b" + gt + "\n\n" +
		"2\n00:01:01,001 --> 01:02:03,004\nzwei Worte\na b\n\n"
	if got := string(WriteSRT(subtitleTestCaptions())); got != want {
		t.Errorf("SRT:\n%q\nwant:\n%q", got, want)
	}
}

func TestSubtitlesVTTBreaksArrowsAndEscapes(t *testing.T) {
	want := "WEBVTT\n\n00:00:00.500 --> 00:00:02.600\na --&gt; b\n&lt;b&gt;&amp;&lt;/b&gt;\n\n" +
		"00:01:01.001 --> 01:02:03.004\nzwei Worte\na b\n\n"
	if got := string(WriteVTT(subtitleTestCaptions())); got != want {
		t.Errorf("VTT:\n%q\nwant:\n%q", got, want)
	}
}
