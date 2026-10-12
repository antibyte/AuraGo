package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCloudFileManagerVisibilityRoundTrip(t *testing.T) {
	var cfg Config
	if cfg.WebDAV.ShowInFileManager || cfg.Koofr.ShowInFileManager {
		t.Fatal("cloud locations must default off")
	}
	if err := yaml.Unmarshal([]byte("webdav:\n  enabled: true\n  show_in_filemanager: true\nkoofr:\n  enabled: true\n  show_in_filemanager: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded Config
	if err = yaml.Unmarshal(data, &reloaded); err != nil {
		t.Fatal(err)
	}
	if !reloaded.WebDAV.ShowInFileManager || !reloaded.Koofr.ShowInFileManager {
		t.Fatal("visibility lost on save")
	}
}
