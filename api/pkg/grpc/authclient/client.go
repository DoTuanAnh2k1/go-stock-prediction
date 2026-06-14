package authclient

import (
	"go-stock-prediction/pkg/logger"
	authpb "go-stock-prediction/proto/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	conn       *grpc.ClientConn
	authClient authpb.AuthServiceClient
)

// Init creates a gRPC connection to the Java Auth Service at the given target
// and initialises the singleton AuthServiceClient.
func Init(target string) {
	logger.Logger.Infof("authclient: connecting to auth service at %s", target)
	c, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Logger.Fatalf("authclient: failed to connect to %s: %v", target, err)
		return
	}
	conn = c
	authClient = authpb.NewAuthServiceClient(conn)
	logger.Logger.Infof("authclient: connected to auth service at %s", target)
}

// GetClient returns the singleton AuthServiceClient.
// Init must be called before GetClient.
func GetClient() authpb.AuthServiceClient {
	return authClient
}

// Close closes the underlying gRPC connection. Should be called during graceful shutdown.
func Close() {
	if conn != nil {
		if err := conn.Close(); err != nil {
			logger.Logger.Errorf("authclient: error closing connection: %v", err)
			return
		}
		logger.Logger.Infof("authclient: connection closed")
	}
}
