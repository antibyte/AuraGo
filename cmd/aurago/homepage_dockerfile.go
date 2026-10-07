package main

import (
	"io"

	"aurago/internal/tools"
)

// printHomepageDockerfile handles `aurago --print-homepage-dockerfile`: it
// writes the Dockerfile of the Homepage dev image, so the image can be built
// on the Docker host when the Engine endpoint refuses builds (a socket proxy
// with BUILD=0):
//
//	docker exec aurago /app/aurago --print-homepage-dockerfile | docker build -t aurago-homepage:latest -
func printHomepageDockerfile(args []string, w io.Writer) bool {
	if len(args) < 2 || args[1] != "--print-homepage-dockerfile" {
		return false
	}
	_, _ = io.WriteString(w, tools.HomepageDockerfile())
	return true
}
