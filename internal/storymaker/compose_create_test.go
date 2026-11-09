package storymaker

import (
	"context"
	"testing"
)

func TestCreateWithOptionsSetsComposeOnlyWhenAsked(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := newTestStore(t)

	_, plain, _, err := ps.Create(ctx, "Editor film", "16:9", 0)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Style.Compose != "" {
		t.Errorf("the editor path must not set compose, got %q", plain.Style.Compose)
	}

	project, v2, _, err := ps.CreateWithOptions(ctx, "Agent film", "16:9", 0, nil, CreateOptions{Compose: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if v2.Style.Compose != "v2" {
		t.Fatalf("compose = %q, want v2", v2.Style.Compose)
	}
	raw, _, err := ps.LoadStory(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustDecode(t, string(raw))
	if saved.Style.Compose != "v2" {
		t.Fatalf("the saved story lost compose: %q", saved.Style.Compose)
	}
}
