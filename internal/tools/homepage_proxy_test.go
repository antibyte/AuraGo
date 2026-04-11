package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProxyRoutesEmpty(t *testing.T) {
	dir := t.TempDir()
	routes := loadProxyRoutes(dir)
	if routes != nil {
		t.Fatalf("expected nil for missing file, got %v", routes)
	}
}

func TestSaveAndLoadProxyRoutes(t *testing.T) {
	dir := t.TempDir()
	expected := []ProxyRoute{
		{Path: "/phaser-demo", Port: 3000},
		{Path: "/api", Port: 8080, StripPrefix: true},
	}
	if err := saveProxyRoutes(dir, expected); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded := loadProxyRoutes(dir)
	if len(loaded) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(loaded))
	}
	if loaded[0].Path != "/phaser-demo" || loaded[0].Port != 3000 {
		t.Fatalf("route 0 mismatch: %+v", loaded[0])
	}
	if loaded[1].Path != "/api" || loaded[1].Port != 8080 || !loaded[1].StripPrefix {
		t.Fatalf("route 1 mismatch: %+v", loaded[1])
	}
}

func TestRegisterProxyRouteAddsNew(t *testing.T) {
	dir := t.TempDir()
	if err := RegisterProxyRoute(dir, ProxyRoute{Path: "/app1", Port: 3000}); err != nil {
		t.Fatalf("register: %v", err)
	}
	routes := loadProxyRoutes(dir)
	if len(routes) != 1 || routes[0].Path != "/app1" {
		t.Fatalf("expected 1 route /app1, got %+v", routes)
	}
}

func TestRegisterProxyRouteReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := RegisterProxyRoute(dir, ProxyRoute{Path: "/app", Port: 3000}); err != nil {
		t.Fatalf("register 1: %v", err)
	}
	if err := RegisterProxyRoute(dir, ProxyRoute{Path: "/app", Port: 4000}); err != nil {
		t.Fatalf("register 2: %v", err)
	}
	routes := loadProxyRoutes(dir)
	if len(routes) != 1 {
		t.Fatalf("expected 1 route (replaced), got %d", len(routes))
	}
	if routes[0].Port != 4000 {
		t.Fatalf("expected port 4000, got %d", routes[0].Port)
	}
}

func TestRemoveProxyRoute(t *testing.T) {
	dir := t.TempDir()
	RegisterProxyRoute(dir, ProxyRoute{Path: "/keep", Port: 3000})
	RegisterProxyRoute(dir, ProxyRoute{Path: "/remove", Port: 4000})
	if err := RemoveProxyRoute(dir, "/remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	routes := loadProxyRoutes(dir)
	if len(routes) != 1 || routes[0].Path != "/keep" {
		t.Fatalf("expected only /keep, got %+v", routes)
	}
}

func TestBuildCaddyfileWithProxiesNoRoutes(t *testing.T) {
	result := buildCaddyfileWithProxies("", 8080, nil)
	if !strings.Contains(result, "file_server") {
		t.Fatal("expected file_server in Caddyfile")
	}
	if !strings.Contains(result, "try_files") {
		t.Fatal("expected try_files in Caddyfile")
	}
	if strings.Contains(result, "reverse_proxy") {
		t.Fatal("did not expect reverse_proxy without routes")
	}
}

func TestBuildCaddyfileWithProxiesWithRoutes(t *testing.T) {
	routes := []ProxyRoute{
		{Path: "/phaser-demo", Port: 3000},
		{Path: "/api", Port: 8080, StripPrefix: true},
	}
	result := buildCaddyfileWithProxies("", 8080, routes)
	if !strings.Contains(result, "handle /phaser-demo*") {
		t.Fatal("expected handle /phaser-demo*")
	}
	if !strings.Contains(result, "reverse_proxy aurago-homepage:3000") {
		t.Fatal("expected reverse_proxy to dev container on port 3000")
	}
	if !strings.Contains(result, "handle_path /api*") {
		t.Fatal("expected handle_path /api* for strip_prefix route")
	}
	if !strings.Contains(result, "reverse_proxy aurago-homepage:8080") {
		t.Fatal("expected reverse_proxy to dev container on port 8080")
	}
	if !strings.Contains(result, "file_server") {
		t.Fatal("expected file_server as default handler")
	}
}

func TestBuildCaddyfileWithProxiesDomain(t *testing.T) {
	routes := []ProxyRoute{{Path: "/app", Port: 3000}}
	result := buildCaddyfileWithProxies("example.com", 443, routes)
	if !strings.HasPrefix(result, "example.com") {
		t.Fatal("expected Caddyfile to start with domain")
	}
}

func TestBuildCaddyfileWithProxiesPathSuffix(t *testing.T) {
	routes := []ProxyRoute{
		{Path: "/trailing/", Port: 3000},
		{Path: "/wildcard*", Port: 4000},
	}
	result := buildCaddyfileWithProxies("", 80, routes)
	if !strings.Contains(result, "handle /trailing/*") {
		t.Fatalf("expected /trailing/*, got:\n%s", result)
	}
	if !strings.Contains(result, "handle /wildcard*") {
		t.Fatalf("expected /wildcard* preserved, got:\n%s", result)
	}
}

func TestProxyRoutesFilePersistence(t *testing.T) {
	dir := t.TempDir()
	p := proxyRoutesPath(dir)
	expected := filepath.Join(dir, ".aurago-proxy-routes.json")
	if p != expected {
		t.Fatalf("expected path %q, got %q", expected, p)
	}

	RegisterProxyRoute(dir, ProxyRoute{Path: "/test", Port: 5000})

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "/test") {
		t.Fatalf("file should contain /test, got: %s", string(data))
	}
}
