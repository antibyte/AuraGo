package config

import "testing"

func TestAgnesIsKnownProviderType(t *testing.T) {
	if !knownProviderTypes["agnes"] {
		t.Fatal("agnes should be a known provider type")
	}
}
