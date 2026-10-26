package tools

import "testing"

func TestParseYouTubeVideoURLAcceptsSharedShapes(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantID    string
		wantStart int
	}{
		{name: "country host", raw: "https://www.youtube.de/watch?v=dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "co country host", raw: "https://youtube.co.uk/shorts/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "com country host", raw: "https://www.youtube.com.br/watch?v=dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "subdomain", raw: "https://music.youtube.com/watch?v=dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "legacy v", raw: "https://www.youtube.com/v/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "legacy e", raw: "https://www.youtube.com/e/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "attribution link", raw: "https://www.youtube.com/attribution_link?a=1&u=/watch%3Fv%3DdQw4w9WgXcQ%26t%3D12s", wantID: "dQw4w9WgXcQ", wantStart: 12},
		{name: "http rebuilt", raw: "http://youtu.be/dQw4w9WgXcQ", wantID: "dQw4w9WgXcQ"},
		{name: "start at cap", raw: "https://youtu.be/dQw4w9WgXcQ?t=172800", wantID: "dQw4w9WgXcQ", wantStart: 172800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseYouTubeVideoURL(tt.raw)
			if err != nil {
				t.Fatalf("ParseYouTubeVideoURL() error = %v", err)
			}
			if ref.VideoID != tt.wantID || ref.StartSeconds != tt.wantStart {
				t.Fatalf("ref = %+v, want id %q start %d", ref, tt.wantID, tt.wantStart)
			}
			if ref.EmbedURL != "https://www.youtube-nocookie.com/embed/"+tt.wantID && tt.wantStart == 0 {
				t.Fatalf("embed = %q", ref.EmbedURL)
			}
			if tt.wantStart > 0 && ref.EmbedURL != "https://www.youtube-nocookie.com/embed/"+tt.wantID+"?start="+itoa(tt.wantStart) {
				t.Fatalf("embed = %q", ref.EmbedURL)
			}
		})
	}
}

func TestParseYouTubeVideoURLRejectsSharedShapes(t *testing.T) {
	for _, raw := range []string{
		"https://user:secret@www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/clip/Ugkxabcdefghijklmnopqrstuv",
		"https://www.youtube.com/playlist?list=PL1234567890",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=172801",
		"https://youtu.be/dQw4w9WgXcQ?t=49h",
		"javascript:alert(1)//youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/attribution_link?u=/attribution_link%3Fu%3D/watch%3Fv%3DdQw4w9WgXcQ",
		"https://notyoutube.com/watch?v=dQw4w9WgXcQ",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseYouTubeVideoURL(raw); err == nil {
				t.Fatalf("ParseYouTubeVideoURL(%q) succeeded, want error", raw)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
