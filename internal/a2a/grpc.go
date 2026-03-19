package a2a

import (
	"context"
	"net"

	"github.com/a2aproject/a2a-go/v2/a2agrpc/v1"
	"google.golang.org/grpc"
)

// startGRPCTransport starts the gRPC transport for the A2A server.
func (s *Server) startGRPCTransport(ctx context.Context, lis net.Listener) error {
	grpcSrv := grpc.NewServer()
	a2aGRPC := a2agrpc.NewHandler(s.handler)
	a2aGRPC.RegisterWith(grpcSrv)

	go func() {
		<-ctx.Done()
		grpcSrv.GracefulStop()
	}()

	return grpcSrv.Serve(lis)
}
