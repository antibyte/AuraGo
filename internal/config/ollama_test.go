package config

import "testing"

func TestOllamaMemoryAndPorts(t *testing.T) {
	for _, bad := range []string{"8gb", "g", "-1", "9223372036854775808", "9223372036854775807g", "18446744073709551617"} {
		if _, ok := ParseOllamaMemoryLimit(bad); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
	if n, ok := ParseOllamaMemoryLimit("8G"); !ok || n != 8<<30 {
		t.Fatal(n, ok)
	}
	var c Config
	c.Ollama.ManagedInstance.Enabled = true
	c.Embeddings.LocalOllama.Enabled = true
	c.Ollama.ManagedInstance.ContainerPort = 11435
	if ValidateOllamaSettings(c) == nil {
		t.Fatal("accepted colliding publications")
	}
	c.Runtime.IsDocker = true
	if err := ValidateOllamaSettings(c); err != nil {
		t.Fatal("separate container namespaces:", err)
	}
	c.Ollama.ManagedInstance.ContainerPort = 65536
	if ValidateOllamaSettings(c) == nil {
		t.Fatal("accepted invalid port")
	}
}
