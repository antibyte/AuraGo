package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
)

func (s *Server) reconcileSpaceAgentHTTPSProxy() {
	cfg := s.currentSpaceAgentConfig()
	if !cfg.SpaceAgent.Enabled || !cfg.SpaceAgent.HTTPSEnabled {
		if err := s.stopSpaceAgentHTTPSProxy(); err != nil {
			s.Logger.Warn("[SpaceAgent] HTTPS proxy stop failed", "error", err)
		}
		return
	}
	if err := s.startSpaceAgentHTTPSProxy(&cfg); err != nil {
		s.Logger.Warn("[SpaceAgent] HTTPS proxy start failed", "error", err)
	}
}

func (s *Server) startSpaceAgentHTTPSProxy(cfg *config.Config) error {
	if s == nil || cfg == nil {
		return fmt.Errorf("server and config are required")
	}
	if s.spaceAgentHTTPS != nil {
		if err := s.stopSpaceAgentHTTPSProxy(); err != nil {
			return err
		}
	}
	port := cfg.SpaceAgent.HTTPSPort
	if port <= 0 {
		port = 3101
	}
	bindHost := strings.TrimSpace(cfg.SpaceAgent.Host)
	if bindHost == "" {
		bindHost = "0.0.0.0"
	}
	addr := net.JoinHostPort(bindHost, strconv.Itoa(port))

	handler, target, err := spaceAgentReverseProxyHandler(cfg, s.Logger)
	if err != nil {
		return err
	}

	certDir := filepath.Join(cfg.Directories.DataDir, "certs")
	certFile := filepath.Join(certDir, "space-agent.crt")
	keyFile := filepath.Join(certDir, "space-agent.key")
	if err := ensureSelfSignedCert(certFile, keyFile, "", s.Logger); err != nil {
		return fmt.Errorf("prepare Space Agent HTTPS certificate: %w", err)
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load Space Agent HTTPS certificate: %w", err)
	}
	tlsCfg := secureTLSConfig()
	tlsCfg.Certificates = []tls.Certificate{cert}

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		TLSConfig:    tlsCfg,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen Space Agent HTTPS proxy on %s: %w", addr, err)
	}
	s.spaceAgentHTTPS = srv
	go func() {
		s.Logger.Info("[SpaceAgent] HTTPS proxy started", "addr", addr, "target", target.String())
		if err := srv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
			s.Logger.Warn("[SpaceAgent] HTTPS proxy stopped", "error", err)
		}
	}()
	return nil
}

func (s *Server) stopSpaceAgentHTTPSProxy() error {
	if s == nil || s.spaceAgentHTTPS == nil {
		return nil
	}
	srv := s.spaceAgentHTTPS
	s.spaceAgentHTTPS = nil
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown Space Agent HTTPS proxy: %w", err)
	}
	return nil
}

func spaceAgentReverseProxyHandler(cfg *config.Config, logger interface {
	Warn(msg string, args ...any)
}) (http.Handler, *url.URL, error) {
	port := cfg.SpaceAgent.Port
	if port <= 0 {
		port = 3100
	}
	target, err := url.Parse("http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid Space Agent proxy target: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		if logger != nil {
			logger.Warn("[SpaceAgent] HTTPS reverse proxy failed", "error", proxyErr)
		}
		http.Error(w, "Space Agent backend unavailable", http.StatusBadGateway)
	}
	return proxy, target, nil
}
