package tools

import (
	"context"
	"reflect"
	"testing"
)

func TestDockerSelfIdentityForUsesTheBoundResolver(t *testing.T) {
	SetDockerSelfIdentityResolver(nil)
	if got := DockerSelfIdentityFor(context.Background(), DockerConfig{}); !reflect.DeepEqual(got, DockerSelfIdentity{}) {
		t.Fatalf("identity without a resolver = %+v, want zero", got)
	}
	var seenHost string
	SetDockerSelfIdentityResolver(func(_ context.Context, cfg DockerConfig) DockerSelfIdentity {
		seenHost = cfg.Host
		return DockerSelfIdentity{ComposeProject: "aurago"}
	})
	t.Cleanup(func() { SetDockerSelfIdentityResolver(nil) })
	got := DockerSelfIdentityFor(context.Background(), DockerConfig{Host: "tcp://docker-proxy:2375"})
	if got.ComposeProject != "aurago" || seenHost != "tcp://docker-proxy:2375" {
		t.Fatalf("identity = %+v for host %q", got, seenHost)
	}
	for project, want := range map[string]bool{"aurago": true, "AuraGo": true, " aurago ": true, "aurago-dev": false, "": false} {
		if got.OwnsComposeProject(project) != want {
			t.Fatalf("OwnsComposeProject(%q) = %v, want %v", project, !want, want)
		}
	}
	if (DockerSelfIdentity{}).OwnsComposeProject("") {
		t.Fatal("the zero identity owns the empty project")
	}
}
