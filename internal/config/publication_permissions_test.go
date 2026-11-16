package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestPublicationGrantsDefaultOffAndRoundTrip(t *testing.T) {
	var legacy Config
	if err := yaml.Unmarshal([]byte("onedrive:\n  enabled: true\nhuggingface:\n  enabled: true\n  allow_writes: true\npaperless_ngx:\n  enabled: true\n"), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.OneDrive.AllowPublicShare || legacy.HuggingFace.AllowPublish || legacy.PaperlessNGX.AllowDelete {
		t.Fatal("legacy config gained new grants")
	}
	legacy.OneDrive.AllowPublicShare = true
	legacy.HuggingFace.AllowPublish = true
	legacy.PaperlessNGX.AllowDelete = true
	raw, err := yaml.Marshal(&legacy)
	if err != nil {
		t.Fatal(err)
	}
	var restored Config
	if err := yaml.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.OneDrive.AllowPublicShare || !restored.HuggingFace.AllowPublish || !restored.PaperlessNGX.AllowDelete {
		t.Fatal("explicit grants lost during roundtrip")
	}
}
