package proxy

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"aurago/internal/config"
)

const (
	// imageName is the official Caddy image, tagged for the proxy. It runs
	// every configuration without rate limiting.
	imageName = "aurago-proxy:latest"

	// The official Caddy image has no rate limiting. With rate limiting
	// enabled AuraGo builds rateLimitImageName once: the pinned Caddy release
	// plus the pinned caddy-ratelimit module (Go module checksums verified).
	caddyBuildVersion      = "2.11.6"
	rateLimitModule        = "github.com/mholt/caddy-ratelimit"
	rateLimitModuleVersion = "v0.1.1-0.20260612195517-5625512f24f6"
	rateLimitImageName     = "aurago-proxy:ratelimit-" + caddyBuildVersion + "-5625512f24f6"

	rateLimitBuildTimeout = 30 * time.Minute
)

// proxyImage returns the image the configuration needs.
func proxyImage(cfg *config.Config) string {
	if cfg.SecurityProxy.RateLimiting.Enabled {
		return rateLimitImageName
	}
	return imageName
}

func rateLimitDockerfile() []byte {
	return []byte("FROM caddy:" + caddyBuildVersion + "-builder AS builder\n" +
		"RUN xcaddy build --with " + rateLimitModule + "@" + rateLimitModuleVersion + "\n\n" +
		"FROM caddy:" + caddyBuildVersion + "\n" +
		"COPY --from=builder /usr/bin/caddy /usr/bin/caddy\n")
}

// ensureImage makes the image the configuration needs available and returns
// its name. Without rate limiting it pulls caddy:latest once; with rate
// limiting it builds the module image once.
func (m *Manager) ensureImage(cfg *config.Config) (string, error) {
	dockerCfg := dockerConfigFor(cfg)
	image := proxyImage(cfg)
	_, code, _ := m.engine.request(dockerCfg, "GET", "/images/"+url.QueryEscape(image)+"/json", "")
	if code == 200 {
		return image, nil // image already exists
	}

	if image == rateLimitImageName {
		m.log().Info("Building the Caddy image with the rate limit module for the security proxy; the first build takes a few minutes",
			"image", image, "module", rateLimitModule+"@"+rateLimitModuleVersion)
		ctx, cancel := context.WithTimeout(context.Background(), rateLimitBuildTimeout)
		defer cancel()
		if err := m.engine.build(ctx, dockerCfg, image, "Dockerfile", rateLimitDockerfile(), nil, m.log()); err != nil {
			return "", fmt.Errorf("%w: %v", ErrRateLimitImageUnavailable, err)
		}
		m.log().Info("Security proxy image ready", "image", image)
		return image, nil
	}

	m.log().Info("Pulling Caddy image for security proxy...")
	// Pull official caddy image and tag it as our image
	_, pullCode, pullErr := m.engine.request(dockerCfg, "POST", "/images/create?fromImage=caddy&tag=latest", "")
	if pullErr != nil {
		return "", fmt.Errorf("pull caddy image: %w", pullErr)
	}
	if pullCode != 200 {
		return "", fmt.Errorf("pull caddy image: HTTP %d", pullCode)
	}

	// Tag as our image name
	_, tagCode, tagErr := m.engine.request(dockerCfg, "POST", "/images/caddy:latest/tag?repo=aurago-proxy&tag=latest", "")
	if tagErr != nil {
		return "", fmt.Errorf("tag image: %w", tagErr)
	}
	if tagCode != 201 {
		return "", fmt.Errorf("tag image: HTTP %d", tagCode)
	}

	m.log().Info("Security proxy image ready", "image", image)
	return image, nil
}
