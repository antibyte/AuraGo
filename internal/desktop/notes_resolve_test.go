package desktop

import "testing"

func TestNoteLookupFixture(t *testing.T) {
	fixture := loadNotesFixture(t)
	lookup := newNoteLookup(fixture.Resolve.Notes, fixture.Resolve.Aliases, fixture.Resolve.Attachments)
	for _, tc := range fixture.Resolve.Cases {
		kind := tc.Kind
		if kind == "" {
			kind = "wiki"
		}
		if got := lookup.Resolve(tc.From, kind, tc.Value, tc.Embed); got != tc.Path {
			t.Errorf("%s %q from %s: got %q want %q", kind, tc.Value, tc.From, got, tc.Path)
		}
	}
}

func TestNoteLookupLinkText(t *testing.T) {
	fixture := loadNotesFixture(t)
	lookup := newNoteLookup(fixture.Resolve.Notes, fixture.Resolve.Aliases, fixture.Resolve.Attachments)
	for path, want := range map[string]string{
		"Documents/Notes/sub/Other.md":          "Other",
		"Documents/Notes/Archiv/2025/Plan.md":   "Archiv/2025/Plan",
		"Documents/Notes/Projekte/Ideen.md":     "Projekte/Ideen",
		"Documents/Notes/Ideen.md":              "Ideen",
		"Documents/Notes/.attachments/bild.png": "bild.png",
	} {
		if got := lookup.LinkText(path); got != want {
			t.Errorf("%s: got %q want %q", path, got, want)
		}
	}
}
