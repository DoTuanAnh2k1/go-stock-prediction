package vn.gostock.auth.grpc;

import io.grpc.stub.StreamObserver;
import lombok.RequiredArgsConstructor;
import net.devh.boot.grpc.server.service.GrpcService;
import vn.gostock.auth.entity.MarketGroup;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.proto.*;
import vn.gostock.auth.service.JwtService;
import vn.gostock.auth.service.MarketGroupService;
import vn.gostock.auth.service.UserService;
import java.util.List;

@GrpcService
@RequiredArgsConstructor
public class AuthGrpcServiceImpl extends AuthServiceGrpc.AuthServiceImplBase {

    private final UserService userService;
    private final MarketGroupService marketGroupService;
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
                req.getUsername(), req.getPassword(), req.getRole());
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
    public void updateUserRole(UpdateRoleRequest req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.updateRole(req.getCaller().getCallerRole(),
                req.getTargetId(), req.getNewRole());
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

    // ── Converters ────────────────────────────────────────────────────────

    private UserResponse toUserResponse(User u) {
        return UserResponse.newBuilder()
            .setId(u.getId())
            .setUsername(u.getUsername())
            .setRole(u.getRole())
            .setCreatedAt(u.getCreatedAt() != null ? u.getCreatedAt().toString() : "")
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
}
