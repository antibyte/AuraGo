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

func TestDockerBindTouchesAuraGoStateOnlyInContainers(t *testing.T) {
	proven := DockerSelfIdentity{Proven: true, StateVolumes: []string{"prod_aurago_data"}, StateBindSources: []string{"/srv/aurago/data"}, StateHostPaths: []string{"/var/lib/docker/volumes/prod_aurago_data/_data"}}
	cases := []struct {
		name        string
		bind        string
		inContainer bool
		self        DockerSelfIdentity
		want        bool
	}{
		{"native installs keep every volume", "aurago_aurago_data:/data", false, DockerSelfIdentity{}, false},
		{"unproven container: shipped volume", "aurago_aurago_data:/data", true, DockerSelfIdentity{}, true},
		{"unproven container: shipped key", "aurago_data:/data:ro", true, DockerSelfIdentity{}, true},
		{"unproven container: unrelated volume", "media:/data", true, DockerSelfIdentity{}, false},
		{"unproven container: workdir stays usable", "aurago_aurago_workdir:/work", true, DockerSelfIdentity{}, false},
		{"proven: exact volume", "prod_aurago_data:/d", true, proven, true},
		{"proven: another stack's aurago_data", "other_aurago_data:/d", true, proven, false},
		{"proven: bind of the data directory", "/srv/aurago/data/vault.bin:/v:ro", true, proven, true},
		{"proven: unrelated bind", "/srv/media:/m", true, proven, false},
		{"anonymous volume", "/data", true, DockerSelfIdentity{}, false},
		{"proven: host path of the data volume", "/var/lib/docker/volumes/prod_aurago_data/_data/vault.bin:/v", true, proven, true},
		{"proven without a data volume: shipped names are not AuraGo's", "aurago_aurago_data:/d", true, DockerSelfIdentity{Proven: true, ComposeProject: "aurago"}, false},
		{"shared namespace: unproven, shipped names stay protected", "x_aurago_data:/d", true, DockerSelfIdentity{ComposeProject: "aurago"}, true},
	}
	for _, tc := range cases {
		if got := DockerBindTouchesAuraGoState(tc.bind, tc.inContainer, tc.self); got != tc.want {
			t.Errorf("%s: DockerBindTouchesAuraGoState(%q) = %v, want %v", tc.name, tc.bind, got, tc.want)
		}
	}
}

func TestDockerComposeModelVolumeNamesListsMountsAndDeclarations(t *testing.T) {
	model := DockerComposeModel{
		Services: map[string]DockerComposeService{"thief": {Image: "alpine", Volumes: []DockerComposeMount{{Type: "volume", Source: "loot", Target: "/l"}, composeBind("/srv/x", "/x", false)}}},
		Volumes:  map[string]DockerComposeNamedVolume{"loot": {Name: "aurago_aurago_data"}, "spare": {Name: "stack_spare"}},
	}
	got := DockerComposeModelVolumeNames(model)
	want := []string{"loot", "aurago_aurago_data", "loot", "aurago_aurago_data", "spare", "stack_spare"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DockerComposeModelVolumeNames() = %q, want %q", got, want)
	}
}
