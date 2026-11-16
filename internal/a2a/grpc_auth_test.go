package a2a

import (
	"context"
	"net"
	"testing"
	"time"

	"aurago/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCAuthenticatesUnaryAndStreaming(t *testing.T) {
	for _, mode := range []string{"key", "bearer", "unconfigured", "empty"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.A2A.Auth.APIKeyEnabled = mode == "key" || mode == "empty"
			cfg.A2A.Auth.BearerEnabled = mode == "bearer"
			if mode != "empty" {
				cfg.A2A.Auth.APIKey = "fixture-key"
			}
			cfg.A2A.Auth.BearerSecret = "fixture-token"
			server := newAuthenticatedGRPCServer(cfg)
			healthpb.RegisterHealthServer(server, health.NewServer())
			listener := bufconn.Listen(1 << 20)
			go server.Serve(listener)
			t.Cleanup(func() { server.Stop(); listener.Close() })
			client, err := grpc.NewClient("passthrough:///fixture", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			for _, cred := range []struct {
				name, key, value string
				valid            bool
			}{
				{"missing", "", "", false}, {"empty", "x-api-key", "", false}, {"wrong", "x-api-key", "wrong", false},
				{"api-key", "x-api-key", "fixture-key", mode == "key"}, {"bearer", "authorization", "Bearer fixture-token", mode == "bearer"},
			} {
				t.Run(cred.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					if cred.key != "" {
						ctx = metadata.AppendToOutgoingContext(ctx, cred.key, cred.value)
					}
					api := healthpb.NewHealthClient(client)
					_, unaryErr := api.Check(ctx, &healthpb.HealthCheckRequest{})
					stream, streamErr := api.Watch(ctx, &healthpb.HealthCheckRequest{})
					if streamErr == nil {
						_, streamErr = stream.Recv()
					}
					for _, err := range []error{unaryErr, streamErr} {
						if cred.valid && err != nil {
							t.Error(err)
						}
						if !cred.valid && status.Code(err) != codes.Unauthenticated {
							t.Errorf("expected Unauthenticated, got %v", err)
						}
					}
				})
			}
		})
	}
}
