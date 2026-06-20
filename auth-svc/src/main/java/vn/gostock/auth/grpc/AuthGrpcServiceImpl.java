package vn.gostock.auth.grpc;

import io.grpc.stub.StreamObserver;
import lombok.RequiredArgsConstructor;
import net.devh.boot.grpc.server.service.GrpcService;
import vn.gostock.auth.entity.MarketGroup;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.proto.*;
import vn.gostock.auth.service.CommandRbacService;
import vn.gostock.auth.service.JwtService;
import vn.gostock.auth.service.MarketGroupService;
import vn.gostock.auth.service.UserService;
import java.util.List;

@GrpcService
@RequiredArgsConstructor
public class AuthGrpcServiceImpl extends AuthServiceGrpc.AuthServiceImplBase {

    private final UserService userService;
    private final MarketGroupService marketGroupService;
    private final CommandRbacService commandRbacService;
    private final JwtService jwtService;

    // ── Auth ──────────────────────────────────────────────────────────────

    @Override
    public void login(LoginRequest req, StreamObserver<LoginResponse> obs) {
        try {
            User user = userService.authenticate(req.getUsername(), req.getPassword());
            List<String> markets = userService.getAccessibleMarkets(user);
            String token = jwtService.generateToken(user, markets);
            obs.onNext(LoginResponse.newBuilder()
                .setToken(token).setUsername(user.getUsername()).setRole(user.getRole())
                .build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void getMe(CallerMeta req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.listUsers().stream()
                .filter(u -> u.getId().equals(req.getCallerId()))
                .findFirst()
                .orElseThrow();
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void changePassword(ChangePassRequest req, StreamObserver<Empty> obs) {
        try {
            userService.changePassword(req.getCaller().getCallerId(),
                req.getOldPassword(), req.getNewPassword());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── User management ───────────────────────────────────────────────────

    @Override
    public void listUsers(CallerMeta req, StreamObserver<ListUsersResponse> obs) {
        try {
            List<UserResponse> users = userService.listUsers().stream()
                .map(this::toUserResponse).toList();
            obs.onNext(ListUsersResponse.newBuilder().addAllUsers(users).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void createUser(CreateUserRequest req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.createUser(req.getCaller().getCallerRole(),
                req.getUsername(), req.getPassword(), req.getRole(),
                req.getFullName(), req.getEmail(), req.getPhone());
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void deleteUser(DeleteUserRequest req, StreamObserver<Empty> obs) {
        try {
            userService.deleteUser(req.getCaller().getCallerRole(), req.getTargetId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void resetPassword(ResetPasswordRequest req, StreamObserver<Empty> obs) {
        try {
            userService.resetPassword(req.getCaller().getCallerRole(),
                req.getTargetId(), req.getNewPassword());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateUserRole(UpdateRoleRequest req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.updateRole(req.getCaller().getCallerRole(),
                req.getTargetId(), req.getNewRole());
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateUser(UpdateUserRequest req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.updateUser(req.getCaller().getCallerRole(),
                req.getTargetId(), req.getFullName(), req.getEmail(),
                req.getPhone(), req.getRole());
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Market groups ─────────────────────────────────────────────────────

    @Override
    public void listMarketGroups(CallerMeta req, StreamObserver<ListGroupsResponse> obs) {
        try {
            List<MarketGroupResponse> groups = marketGroupService.listGroups().stream()
                .map(this::toGroupResponse).toList();
            obs.onNext(ListGroupsResponse.newBuilder().addAllGroups(groups).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void createMarketGroup(CreateGroupRequest req, StreamObserver<MarketGroupResponse> obs) {
        try {
            MarketGroup g = marketGroupService.createGroup(
                req.getCaller().getCallerRole(), req.getName(), req.getDescription());
            obs.onNext(toGroupResponse(g));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateMarketGroup(UpdateGroupRequest req, StreamObserver<MarketGroupResponse> obs) {
        try {
            MarketGroup g = marketGroupService.updateGroup(
                req.getCaller().getCallerRole(), req.getGroupId(),
                req.getName(), req.getDescription());
            obs.onNext(toGroupResponse(g));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void deleteMarketGroup(DeleteGroupRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.deleteGroup(req.getCaller().getCallerRole(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void setGroupMarkets(SetGroupMarketsRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.setGroupMarkets(req.getCaller().getCallerRole(),
                req.getGroupId(), req.getMarketKeysList());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void listGroupUsers(GroupRequest req, StreamObserver<ListUsersResponse> obs) {
        try {
            List<UserResponse> users = marketGroupService
                .listGroupUsers(req.getCaller().getCallerRole(), req.getGroupId())
                .stream().map(this::toUserResponse).toList();
            obs.onNext(ListUsersResponse.newBuilder().addAllUsers(users).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void addUserToGroup(UserGroupRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.addUserToGroup(req.getCaller().getCallerRole(),
                req.getUserId(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void removeUserFromGroup(UserGroupRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.removeUserFromGroup(req.getCaller().getCallerRole(),
                req.getUserId(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void getUserMarketGroups(UserRequest req, StreamObserver<ListGroupsResponse> obs) {
        try {
            List<MarketGroupResponse> groups = marketGroupService
                .getUserMarketGroups(req.getUserId())
                .stream().map(this::toGroupResponse).toList();
            obs.onNext(ListGroupsResponse.newBuilder().addAllGroups(groups).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Command RBAC: handler catalog ─────────────────────────────────────

    @Override
    public void upsertHandlers(UpsertHandlersRequest req, StreamObserver<Empty> obs) {
        try {
            List<vn.gostock.auth.entity.CliHandler> handlers = req.getHandlersList().stream()
                .map(this::toHandlerEntity).toList();
            commandRbacService.upsertHandlers(req.getSecret(), handlers);
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void listHandlers(CallerMeta req, StreamObserver<ListHandlersResponse> obs) {
        try {
            List<CliHandler> handlers = commandRbacService.listHandlers().stream()
                .map(this::toHandlerResponse).toList();
            obs.onNext(ListHandlersResponse.newBuilder().addAllHandlers(handlers).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Command RBAC: commands ────────────────────────────────────────────

    @Override
    public void listCommands(CallerMeta req, StreamObserver<ListCommandsResponse> obs) {
        try {
            List<Command> commands = commandRbacService.listCommands().stream()
                .map(this::toCommandProto).toList();
            obs.onNext(ListCommandsResponse.newBuilder().addAllCommands(commands).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void createCommand(CreateCommandRequest req, StreamObserver<CommandResponse> obs) {
        try {
            vn.gostock.auth.entity.Command c = commandRbacService.createCommand(
                req.getCaller().getCallerRole(), req.getName(), req.getDescription(),
                req.getHandlerKey(), req.getArgs(), req.getEnabled());
            obs.onNext(CommandResponse.newBuilder().setCommand(toCommandProto(c)).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateCommand(UpdateCommandRequest req, StreamObserver<CommandResponse> obs) {
        try {
            vn.gostock.auth.entity.Command c = commandRbacService.updateCommand(
                req.getCaller().getCallerRole(), req.getCommandId(), req.getName(),
                req.getDescription(), req.getHandlerKey(), req.getArgs(), req.getEnabled());
            obs.onNext(CommandResponse.newBuilder().setCommand(toCommandProto(c)).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void deleteCommand(DeleteCommandRequest req, StreamObserver<Empty> obs) {
        try {
            commandRbacService.deleteCommand(req.getCaller().getCallerRole(), req.getCommandId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Command RBAC: command groups ──────────────────────────────────────

    @Override
    public void listCommandGroups(CallerMeta req, StreamObserver<ListCommandGroupsResponse> obs) {
        try {
            List<CommandGroupResponse> groups = commandRbacService.listGroups().stream()
                .map(this::toCommandGroupResponse).toList();
            obs.onNext(ListCommandGroupsResponse.newBuilder().addAllGroups(groups).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void createCommandGroup(CreateCmdGroupRequest req, StreamObserver<CommandGroupResponse> obs) {
        try {
            vn.gostock.auth.entity.CommandGroup g = commandRbacService.createGroup(
                req.getCaller().getCallerRole(), req.getName(), req.getDescription());
            obs.onNext(toCommandGroupResponse(g));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateCommandGroup(UpdateCmdGroupRequest req, StreamObserver<CommandGroupResponse> obs) {
        try {
            vn.gostock.auth.entity.CommandGroup g = commandRbacService.updateGroup(
                req.getCaller().getCallerRole(), req.getGroupId(),
                req.getName(), req.getDescription());
            obs.onNext(toCommandGroupResponse(g));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void deleteCommandGroup(DeleteCmdGroupRequest req, StreamObserver<Empty> obs) {
        try {
            commandRbacService.deleteGroup(req.getCaller().getCallerRole(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void setGroupCommands(SetGroupCommandsRequest req, StreamObserver<Empty> obs) {
        try {
            commandRbacService.setGroupCommands(req.getCaller().getCallerRole(),
                req.getGroupId(), req.getCommandIdsList());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void listCmdGroupUsers(CmdGroupRequest req, StreamObserver<ListUsersResponse> obs) {
        try {
            List<UserResponse> users = commandRbacService
                .listGroupUsers(req.getCaller().getCallerRole(), req.getGroupId())
                .stream().map(this::toUserResponse).toList();
            obs.onNext(ListUsersResponse.newBuilder().addAllUsers(users).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void addUserToCmdGroup(UserCmdGroupRequest req, StreamObserver<Empty> obs) {
        try {
            commandRbacService.addUserToGroup(req.getCaller().getCallerRole(),
                req.getUserId(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void removeUserFromCmdGroup(UserCmdGroupRequest req, StreamObserver<Empty> obs) {
        try {
            commandRbacService.removeUserFromGroup(req.getCaller().getCallerRole(),
                req.getUserId(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Command RBAC: enforcement ─────────────────────────────────────────

    @Override
    public void getUserCommands(UserRequest req, StreamObserver<ListCommandsResponse> obs) {
        try {
            List<Command> commands = commandRbacService.getUserCommands(req.getUserId())
                .stream().map(this::toCommandProto).toList();
            obs.onNext(ListCommandsResponse.newBuilder().addAllCommands(commands).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Converters ────────────────────────────────────────────────────────

    private UserResponse toUserResponse(User u) {
        return UserResponse.newBuilder()
            .setId(u.getId())
            .setUsername(u.getUsername())
            .setRole(u.getRole())
            .setCreatedAt(u.getCreatedAt() != null ? u.getCreatedAt().toString() : "")
            .setFullName(u.getFullName() != null ? u.getFullName() : "")
            .setEmail(u.getEmail() != null ? u.getEmail() : "")
            .setPhone(u.getPhone() != null ? u.getPhone() : "")
            .build();
    }

    private MarketGroupResponse toGroupResponse(MarketGroup g) {
        return MarketGroupResponse.newBuilder()
            .setId(g.getId())
            .setName(g.getName())
            .setDescription(g.getDescription() != null ? g.getDescription() : "")
            .addAllMarketKeys(g.getMarketKeys())
            .setCreatedAt(g.getCreatedAt() != null ? g.getCreatedAt().toString() : "")
            .setUpdatedAt(g.getUpdatedAt() != null ? g.getUpdatedAt().toString() : "")
            .build();
    }

    // proto CliHandler -> entity CliHandler
    private vn.gostock.auth.entity.CliHandler toHandlerEntity(CliHandler h) {
        return vn.gostock.auth.entity.CliHandler.builder()
            .handlerKey(h.getHandlerKey())
            .displayName(h.getDisplayName())
            .verb(h.getVerb())
            .resource(h.getResource())
            .argSchema(h.getArgSchema() != null && !h.getArgSchema().isEmpty() ? h.getArgSchema() : "[]")
            .enabled(h.getEnabled())
            .build();
    }

    private CliHandler toHandlerResponse(vn.gostock.auth.entity.CliHandler h) {
        return CliHandler.newBuilder()
            .setHandlerKey(h.getHandlerKey())
            .setDisplayName(h.getDisplayName() != null ? h.getDisplayName() : "")
            .setVerb(h.getVerb() != null ? h.getVerb() : "")
            .setResource(h.getResource() != null ? h.getResource() : "")
            .setArgSchema(h.getArgSchema() != null ? h.getArgSchema() : "[]")
            .setEnabled(h.getEnabled() != null ? h.getEnabled() : false)
            .build();
    }

    private Command toCommandProto(vn.gostock.auth.entity.Command c) {
        return Command.newBuilder()
            .setId(c.getId())
            .setName(c.getName())
            .setDescription(c.getDescription() != null ? c.getDescription() : "")
            .setHandlerKey(c.getHandlerKey() != null ? c.getHandlerKey() : "")
            .setArgs(c.getArgs() != null ? c.getArgs() : "{}")
            .setEnabled(c.getEnabled() != null ? c.getEnabled() : false)
            .setCreatedAt(c.getCreatedAt() != null ? c.getCreatedAt().toString() : "")
            .setUpdatedAt(c.getUpdatedAt() != null ? c.getUpdatedAt().toString() : "")
            .build();
    }

    private CommandGroupResponse toCommandGroupResponse(vn.gostock.auth.entity.CommandGroup g) {
        return CommandGroupResponse.newBuilder()
            .setId(g.getId())
            .setName(g.getName())
            .setDescription(g.getDescription() != null ? g.getDescription() : "")
            .addAllCommandIds(g.getCommandIds())
            .setCreatedAt(g.getCreatedAt() != null ? g.getCreatedAt().toString() : "")
            .setUpdatedAt(g.getUpdatedAt() != null ? g.getUpdatedAt().toString() : "")
            .build();
    }
}
