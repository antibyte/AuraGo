package main

import (
	"bytes"
	"testing"

	"aurago/internal/tools"
)

func TestPrintHomepageDockerfileArgument(t *testing.T) {
	var out bytes.Buffer
	if !printHomepageDockerfile([]string{"aurago", "--print-homepage-dockerfile"}, &out) {
		t.Fatal("--print-homepage-dockerfile was not handled")
	}
	if out.String() != tools.HomepageDockerfile() {
		t.Fatalf("printed %q, want the Homepage Dockerfile", out.String())
	}
	out.Reset()
	if printHomepageDockerfile([]string{"aurago"}, &out) || printHomepageDockerfile([]string{"aurago", "-debug"}, &out) || out.Len() != 0 {
		t.Fatal("other arguments must not print the Dockerfile")
	}
}
