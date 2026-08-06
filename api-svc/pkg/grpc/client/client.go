package client

import (
	"context"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/reqid"
	"go-stock-prediction/pkg/telemetry"
	pb "go-stock-prediction/proto/prediction"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	conn             *grpc.ClientConn
	predictionClient pb.PredictionServiceClient
)

// reqidUnaryInterceptor propagates the correlation ID from the context into
// outbound gRPC unary call metadata as "x-request-id".
func reqidUnaryInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if id, ok := reqid.FromContext(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, reqid.MetaKey, id)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// reqidStreamInterceptor propagates the correlation ID from the context into
// outbound gRPC streaming call metadata as "x-request-id".
func reqidStreamInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if id, ok := reqid.FromContext(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, reqid.MetaKey, id)
	}
	return streamer(ctx, desc, cc, method, opts...)
}

// Init creates a gRPC connection to the given target and initialises the
// singleton PredictionServiceClient. It calls logger.Logger.Fatalf on any
// connection error, which terminates the process.
func Init(target string) {
	logger.Logger.Infof("gRPC client: connecting to prediction service at %s", target)

	c, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Injects the current trace context into outbound RPC metadata so the
		// span chains into the Python prediction service.
		telemetry.GRPCClientDialOption(),
		// Propagates the X-Request-ID correlation ID into outbound gRPC metadata.
		grpc.WithChainUnaryInterceptor(reqidUnaryInterceptor),
		grpc.WithChainStreamInterceptor(reqidStreamInterceptor),
	)
	if err != nil {
		logger.Logger.Fatalf("gRPC client: failed to connect to %s: %v", target, err)
		return
	}

	conn = c
	predictionClient = pb.NewPredictionServiceClient(conn)

	logger.Logger.Infof("gRPC client: connected to prediction service at %s", target)
}

// GetClient returns the singleton PredictionServiceClient.
// Init must be called before GetClient.
func GetClient() pb.PredictionServiceClient {
	return predictionClient
}

// Close closes the underlying gRPC connection. It should be called during
// graceful shutdown.
func Close() {
	if conn != nil {
		if err := conn.Close(); err != nil {
			logger.Logger.Errorf("gRPC client: error closing connection: %v", err)
			return
		}
		logger.Logger.Infof("gRPC client: connection closed")
	}
}
