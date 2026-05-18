package main

import "testing"

func TestIsRunningAsDarwinServiceUsesLaunchdParent(t *testing.T) {
	if !isRunningAsDarwinService(1) {
		t.Fatal("launchd parent pid should indicate service mode")
	}
	if isRunningAsDarwinService(123) {
		t.Fatal("non-launchd parent pid should not indicate service mode")
	}
}
