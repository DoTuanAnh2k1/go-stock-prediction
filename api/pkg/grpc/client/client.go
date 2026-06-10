package client

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	conn            *grpc.ClientConn
	predictionClient pb.PredictionServiceClient
)

// Init creates a gRPC connection to the given target and initialises the
// singleton PredictionServiceClient. It calls logger.Logger.Fatalf on any
// connection error, which terminates the process.
func Init(target string) {
	logger.Logger.Infof("gRPC client: connecting to prediction service at %s", target)

	c, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
