package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestNetworkInitializationFlags(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		want     bool
		wantHost string
	}{
		{"absent retains HTTPS", nil, true, "127.0.0.1"},
		{"explicit opt out", []string{"-https=false"}, false, "127.0.0.1"},
		{"independent host", []string{"-https=true", "-host=::1", "-https-port=8443", "-http-port=0", "-cert-mode=selfsigned"}, true, "::1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			data := []byte("server:\n  host: 127.0.0.1\n  https:\n    enabled: true\n    https_port: 8443\n    http_port: 0\n")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			fs := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			var opts networkInitFlags
			opts.register(fs)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if err := opts.apply(fs, cfg, path); err != nil {
				t.Fatal(err)
			}
			loaded, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Server.HTTPS.Enabled != tc.want || loaded.Server.Host != tc.wantHost || loaded.Server.HTTPS.HTTPPort != 0 || loaded.Server.HTTPS.HTTPSPort != 8443 {
				t.Fatalf("wrong network settings: %+v", loaded.Server)
			}
			if tc.args == nil {
				got, _ := os.ReadFile(path)
				if string(got) != string(data) {
					t.Fatal("absent flags rewrote config")
				}
			}
		})
	}
}

func TestNetworkInitializationReportsWriteFailure(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var opts networkInitFlags
	opts.register(fs)
	_ = fs.Parse([]string{"-https=true"})
	cfg := &config.Config{}
	if err := opts.apply(fs, cfg, t.TempDir()); err == nil {
		t.Fatal("directory destination must fail")
	}
	if cfg.Server.HTTPS.Enabled {
		t.Fatal("failed save changed the live config")
	}
}
