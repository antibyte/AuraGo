package a2a

import (
	"aurago/internal/config"
	"context"
	"net"

	"github.com/a2aproject/a2a-go/v2/a2agrpc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// startGRPCTransport starts the gRPC transport for the A2A server.
func (s *Server) startGRPCTransport(ctx context.Context, lis net.Listener) error {
	grpcSrv := newAuthenticatedGRPCServer(s.cfg)
	a2aGRPC := a2agrpc.NewHandler(s.handler)
	a2aGRPC.RegisterWith(grpcSrv)

	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
	}()

	return grpcSrv.Serve(lis)
}

func newAuthenticatedGRPCServer(cfg *config.Config) *grpc.Server {
	authenticate := func(ctx context.Context) error {
		md, _ := metadata.FromIncomingContext(ctx)
		value := func(key string) string {
			values := md.Get(key)
			if len(values) == 1 {
				return values[0]
			}
			return ""
		}
		if !validCredentials(cfg, value("x-api-key"), value("authorization")) {
			return status.Error(codes.Unauthenticated, "Unauthorized")
		}
		return nil
	}
	return grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if err := authenticate(ctx); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if err := authenticate(stream.Context()); err != nil {
				return err
			}
			return handler(srv, stream)
		}),
	)
}
