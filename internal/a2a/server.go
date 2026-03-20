package a2a

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"aurago/internal/config"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// Server manages the A2A protocol server lifecycle.
type Server struct {
	cfg       *config.Config
	logger    *slog.Logger
	executor  *Executor
	taskStore *TaskStore
	card      *a2a.AgentCard
	handler   a2asrv.RequestHandler

	grpcListener net.Listener
	mu           sync.Mutex
	running      bool
}

// NewServer creates a new A2A server.
func NewServer(cfg *config.Config, logger *slog.Logger, deps *ExecutorDeps) *Server {
	executor := NewExecutor(deps)
	taskStore := NewTaskStore()
	card := BuildAgentCard(cfg)

	capabilities := &a2a.AgentCapabilities{
		Streaming:         cfg.A2A.Server.Streaming,
		PushNotifications: cfg.A2A.Server.PushNotifications,
	}

	handler := a2asrv.NewHandler(executor,
		a2asrv.WithCapabilityChecks(capabilities),
		a2asrv.WithTaskStore(taskStore),
		a2asrv.WithLogger(logger.With("component", "a2a-handler")),
	)

	return &Server{
		cfg:       cfg,
		logger:    logger,
		executor:  executor,
		taskStore: taskStore,
		card:      card,
		handler:   handler,
	}
}

// RegisterRoutes registers A2A protocol routes on the given HTTP mux.
// This is used when A2A shares the main server port (port=0).
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	basePath := s.cfg.A2A.Server.BasePath

	// Agent Card endpoint (always public)
	mux.Handle("/.well-known/agent-card.json", a2asrv.NewStaticAgentCardHandler(s.card))

	// Protocol bindings — wrapped in auth middleware
	if s.cfg.A2A.Server.Bindings.REST {
		restHandler := a2asrv.NewRESTHandler(s.handler)
		mux.Handle(basePath+"/", AuthMiddleware(s.cfg, http.StripPrefix(basePath, restHandler)))
		s.logger.Info("A2A REST binding registered", "path", basePath)
	}

	if s.cfg.A2A.Server.Bindings.JSONRPC {
		jsonrpcHandler := a2asrv.NewJSONRPCHandler(s.handler)
		mux.Handle(basePath+"/jsonrpc", AuthMiddleware(s.cfg, jsonrpcHandler))
		s.logger.Info("A2A JSON-RPC binding registered", "path", basePath+"/jsonrpc")
	}
}

// StartDedicatedServer starts a dedicated HTTP server on a separate port for A2A.
// Returns after the server starts listening. Stopped via context cancellation.
func (s *Server) StartDedicatedServer(ctx context.Context) error {
	port := s.cfg.A2A.Server.Port
	if port == 0 {
		return nil // shared port mode, routes registered via RegisterRoutes
	}

	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	host := s.cfg.Server.Host
	if host == "" {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	// Warn when the A2A port is bound to all interfaces without authentication.
	// In this state any internet client can reach the agent-to-agent API.
	if (host == "0.0.0.0" || host == "") &&
		!s.cfg.A2A.Auth.APIKeyEnabled && !s.cfg.A2A.Auth.BearerEnabled {
		s.logger.Warn("[A2A] Dedicated server is listening on all interfaces without authentication — "+
			"set a2a.auth.api_key_enabled or a2a.auth.bearer_enabled in config",
			"addr", addr)
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // SSE requires no write timeout
		IdleTimeout:  120 * time.Second,
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	s.logger.Info("A2A dedicated server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("a2a server: %w", err)
	}
	return nil
}

// StartGRPCServer starts a gRPC server for the A2A gRPC binding.
// This requires the a2agrpc package which brings in gRPC dependencies.
func (s *Server) StartGRPCServer(ctx context.Context) error {
	if !s.cfg.A2A.Server.Bindings.GRPC {
		return nil
	}

	port := s.cfg.A2A.Server.Bindings.GRPCPort
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("a2a grpc listen: %w", err)
	}

	s.mu.Lock()
	s.grpcListener = lis
	s.mu.Unlock()

	s.logger.Info("A2A gRPC server starting", "addr", addr)

	// gRPC server setup is handled in grpc.go (build-tag guarded for optional gRPC support)
	return s.startGRPCTransport(ctx, lis)
}

// StartCleanup starts the background task store cleanup.
func (s *Server) StartCleanup(ctx context.Context) {
	s.taskStore.StartCleanupLoop(ctx, 5*time.Minute, 1*time.Hour)
}

// Status returns the current server status.
func (s *Server) Status() ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ServerStatus{
		Enabled:     s.cfg.A2A.Server.Enabled,
		Running:     s.running,
		Port:        s.cfg.A2A.Server.Port,
		BasePath:    s.cfg.A2A.Server.BasePath,
		TaskCount:   s.taskStore.Count(),
		ActiveTasks: s.taskStore.ActiveCount(),
		Bindings: BindingStatus{
			REST:    s.cfg.A2A.Server.Bindings.REST,
			JSONRPC: s.cfg.A2A.Server.Bindings.JSONRPC,
			GRPC:    s.cfg.A2A.Server.Bindings.GRPC,
		},
	}
}

// Card returns the current agent card.
func (s *Server) Card() *a2a.AgentCard {
	return s.card
}

// ServerStatus represents the A2A server status.
type ServerStatus struct {
	Enabled     bool          `json:"enabled"`
	Running     bool          `json:"running"`
	Port        int           `json:"port"`
	BasePath    string        `json:"base_path"`
	TaskCount   int           `json:"task_count"`
	ActiveTasks int           `json:"active_tasks"`
	Bindings    BindingStatus `json:"bindings"`
}

// BindingStatus represents which protocol bindings are active.
type BindingStatus struct {
	REST    bool `json:"rest"`
	JSONRPC bool `json:"json_rpc"`
	GRPC    bool `json:"grpc"`
}
