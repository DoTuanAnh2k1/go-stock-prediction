package authclient

import (
	"context"

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

// ===========================================================================
// Command RBAC wrappers — thin pass-throughs to the singleton AuthServiceClient.
// Mirror the market-group RPC surface for commands / command groups / handlers.
// ===========================================================================

// UpsertHandlers upserts the CLI handler catalog (internal, secret-protected).
func UpsertHandlers(ctx context.Context, in *authpb.UpsertHandlersRequest) (*authpb.Empty, error) {
	return authClient.UpsertHandlers(ctx, in)
}

// ListHandlers lists the CLI handler catalog.
func ListHandlers(ctx context.Context, in *authpb.CallerMeta) (*authpb.ListHandlersResponse, error) {
	return authClient.ListHandlers(ctx, in)
}

// ListCommands lists all commands.
func ListCommands(ctx context.Context, in *authpb.CallerMeta) (*authpb.ListCommandsResponse, error) {
	return authClient.ListCommands(ctx, in)
}

// CreateCommand creates a new command.
func CreateCommand(ctx context.Context, in *authpb.CreateCommandRequest) (*authpb.CommandResponse, error) {
	return authClient.CreateCommand(ctx, in)
}

// UpdateCommand updates an existing command.
func UpdateCommand(ctx context.Context, in *authpb.UpdateCommandRequest) (*authpb.CommandResponse, error) {
	return authClient.UpdateCommand(ctx, in)
}

// DeleteCommand deletes a command.
func DeleteCommand(ctx context.Context, in *authpb.DeleteCommandRequest) (*authpb.Empty, error) {
	return authClient.DeleteCommand(ctx, in)
}

// ListCommandGroups lists all command groups.
func ListCommandGroups(ctx context.Context, in *authpb.CallerMeta) (*authpb.ListCommandGroupsResponse, error) {
	return authClient.ListCommandGroups(ctx, in)
}

// CreateCommandGroup creates a new command group.
func CreateCommandGroup(ctx context.Context, in *authpb.CreateCmdGroupRequest) (*authpb.CommandGroupResponse, error) {
	return authClient.CreateCommandGroup(ctx, in)
}

// UpdateCommandGroup updates an existing command group.
func UpdateCommandGroup(ctx context.Context, in *authpb.UpdateCmdGroupRequest) (*authpb.CommandGroupResponse, error) {
	return authClient.UpdateCommandGroup(ctx, in)
}

// DeleteCommandGroup deletes a command group.
func DeleteCommandGroup(ctx context.Context, in *authpb.DeleteCmdGroupRequest) (*authpb.Empty, error) {
	return authClient.DeleteCommandGroup(ctx, in)
}

// SetGroupCommands replaces the set of commands in a group.
func SetGroupCommands(ctx context.Context, in *authpb.SetGroupCommandsRequest) (*authpb.Empty, error) {
	return authClient.SetGroupCommands(ctx, in)
}

// ListCmdGroupUsers lists users in a command group.
func ListCmdGroupUsers(ctx context.Context, in *authpb.CmdGroupRequest) (*authpb.ListUsersResponse, error) {
	return authClient.ListCmdGroupUsers(ctx, in)
}

// AddUserToCmdGroup adds a user to a command group.
func AddUserToCmdGroup(ctx context.Context, in *authpb.UserCmdGroupRequest) (*authpb.Empty, error) {
	return authClient.AddUserToCmdGroup(ctx, in)
}

// RemoveUserFromCmdGroup removes a user from a command group.
func RemoveUserFromCmdGroup(ctx context.Context, in *authpb.UserCmdGroupRequest) (*authpb.Empty, error) {
	return authClient.RemoveUserFromCmdGroup(ctx, in)
}

// GetUserCommands returns the commands a user is allowed to run.
func GetUserCommands(ctx context.Context, in *authpb.UserRequest) (*authpb.ListCommandsResponse, error) {
	return authClient.GetUserCommands(ctx, in)
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
