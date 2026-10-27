package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The loader, the shell routing, the window sizing and the generated main bundle know the Docker Manager app, and its
// eleven modules load in the order they depend on each other. Behaviour is covered by scripts/test-docker-manager-ui.mjs
// and TestDesktopDockerManagerBrowser.
func TestDockerManagerWiring(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entry := regexp.MustCompile(`'docker-manager': \{ styles: appStyles\('/css/desktop-app-docker-manager\.css'\), scripts: \[([^\]]*)\] \}`).FindStringSubmatch(loader)
	if entry == nil {
		t.Fatal("module-loader.js must register the docker-manager assets")
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`'/js/desktop/apps/([a-z-]+\.js)'`).FindAllStringSubmatch(entry[1], -1) {
		scripts = append(scripts, match[1])
	}
	want := []string{
		"docker-manager-model.js", "docker-manager-stack-model.js", "docker-manager-store.js", "docker-manager-views.js", "docker-manager-stacks.js", "docker-manager-editor.js",
		"docker-manager-ops.js", "docker-manager-palette.js", "docker-manager-dialogs.js", "docker-manager-newstack.js", "docker-manager.js",
	}
	if strings.Join(scripts, ",") != strings.Join(want, ",") {
		t.Fatalf("docker-manager scripts = %v, want %v", scripts, want)
	}
	if !strings.Contains(loader, "'docker-manager': ['docker_manager']") {
		t.Fatal("module-loader.js must load the docker_manager translation section for the app")
	}

	for file, markers := range map[string][]string{
		"js/desktop/core/menus-and-routing.js":    {"appId === 'docker-manager' && window.DockerManagerApp", "window.DockerManagerApp.render(contentEl(id), id,"},
		"js/desktop/core/desktop-foundation.js":   {"'docker-manager': 'DockerManagerApp'"},
		"js/desktop/core/window-shell-runtime.js": {"'docker-manager': { width: 1180, height: 760 }", "appId === 'docker-manager') return { width: 440, height: 400 }"},
		"js/desktop/bundles/main.bundle.js":       {"appId === 'docker-manager' && window.DockerManagerApp", "'docker-manager': 'DockerManagerApp'", "'docker-manager': { width: 1180, height: 760 }"},
	} {
		source := readDesktopAssetText(t, file)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Errorf("%s is missing %q (run npm run build:ui after editing the core files)", file, marker)
			}
		}
	}
}
