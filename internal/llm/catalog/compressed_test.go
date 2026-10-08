package catalog

import (
	"bytes"
	"os"
	"testing"
)

func TestCompressedCatalogMatchesSource(t *testing.T) {
	for _, name := range []string{"ohmypi_models.json", "ohmypi_providers.json"} {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := readCompressedCatalog(name + ".gz")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("compressed catalog differs from source; regenerate with sync_ohmypi_catalog.go")
			}
			if _, err := bundledFS.ReadFile(name); err == nil {
				t.Fatal("uncompressed source must not be embedded alongside gzip")
			}
		})
	}
}
