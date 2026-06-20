package vn.gostock.auth.grpc;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.*;
import org.mockito.junit.jupiter.MockitoExtension;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.proto.*;
import vn.gostock.auth.service.CommandRbacService;
import vn.gostock.auth.service.JwtService;
import vn.gostock.auth.service.MarketGroupService;
import vn.gostock.auth.service.UserService;
import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class AuthGrpcServiceImplTest {

    @Mock UserService userService;
    @Mock MarketGroupService marketGroupService;
    @Mock CommandRbacService commandRbacService;
    @Mock JwtService jwtService;

    @InjectMocks AuthGrpcServiceImpl grpcService;

    private User mockUser;

    @BeforeEach
    void setUp() {
        mockUser = User.builder()
            .id(1L).username("testuser").role("user")
            .build();
    }

    @Test
    void login_validCredentials_returnsToken() {
        when(userService.authenticate("testuser", "pass123")).thenReturn(mockUser);
        when(userService.getAccessibleMarkets(mockUser)).thenReturn(List.of("GOLD"));
        when(jwtService.generateToken(mockUser, List.of("GOLD"))).thenReturn("jwt-token");

        StreamObserver<LoginResponse> obs = mock(StreamObserver.class);
        grpcService.login(LoginRequest.newBuilder()
            .setUsername("testuser").setPassword("pass123").build(), obs);

        ArgumentCaptor<LoginResponse> captor = ArgumentCaptor.forClass(LoginResponse.class);
        verify(obs).onNext(captor.capture());
        verify(obs).onCompleted();
        assert captor.getValue().getToken().equals("jwt-token");
    }

    @Test
    void login_invalidCredentials_propagatesUnauthenticated() {
        when(userService.authenticate(anyString(), anyString()))
            .thenThrow(new StatusRuntimeException(Status.UNAUTHENTICATED));

        StreamObserver<LoginResponse> obs = mock(StreamObserver.class);
        grpcService.login(LoginRequest.newBuilder()
            .setUsername("bad").setPassword("bad").build(), obs);

        verify(obs).onError(any(StatusRuntimeException.class));
        verify(obs, never()).onCompleted();
    }

    @Test
    void createUser_adminCaller_createsUser() {
        when(userService.createUser("admin", "newuser", "pass", "user", "", "", "")).thenReturn(
            User.builder().id(2L).username("newuser").role("user").build());

        StreamObserver<UserResponse> obs = mock(StreamObserver.class);
        grpcService.createUser(CreateUserRequest.newBuilder()
            .setCaller(CallerMeta.newBuilder().setCallerId(1L).setCallerRole("admin").build())
            .setUsername("newuser").setPassword("pass").setRole("user").build(), obs);

        ArgumentCaptor<UserResponse> captor = ArgumentCaptor.forClass(UserResponse.class);
        verify(obs).onNext(captor.capture());
        assert captor.getValue().getUsername().equals("newuser");
    }

    @Test
    void deleteUser_adminCannotDeleteSuperAdmin_propagatesPermissionDenied() {
        doThrow(new StatusRuntimeException(Status.PERMISSION_DENIED))
            .when(userService).deleteUser("admin", 99L);

        StreamObserver<Empty> obs = mock(StreamObserver.class);
        grpcService.deleteUser(DeleteUserRequest.newBuilder()
            .setCaller(CallerMeta.newBuilder().setCallerRole("admin").build())
            .setTargetId(99L).build(), obs);

        verify(obs).onError(any(StatusRuntimeException.class));
    }

    // ── Command RBAC delegation ───────────────────────────────────────────

    @Test
    void createCommand_adminCaller_returnsCommand() {
        when(commandRbacService.createCommand("admin", "c1", "d", "market.latest", "{}", true))
            .thenReturn(vn.gostock.auth.entity.Command.builder()
                .id(7L).name("c1").handlerKey("market.latest").args("{}").enabled(true).build());

        StreamObserver<CommandResponse> obs = mock(StreamObserver.class);
        grpcService.createCommand(CreateCommandRequest.newBuilder()
            .setCaller(CallerMeta.newBuilder().setCallerRole("admin").build())
            .setName("c1").setDescription("d").setHandlerKey("market.latest")
            .setArgs("{}").setEnabled(true).build(), obs);

        ArgumentCaptor<CommandResponse> captor = ArgumentCaptor.forClass(CommandResponse.class);
        verify(obs).onNext(captor.capture());
        verify(obs).onCompleted();
        assert captor.getValue().getCommand().getName().equals("c1");
    }

    @Test
    void getUserCommands_delegatesAndMaps() {
        when(commandRbacService.getUserCommands(4L)).thenReturn(List.of(
            vn.gostock.auth.entity.Command.builder().id(1L).name("a").handlerKey("h").build()));

        StreamObserver<ListCommandsResponse> obs = mock(StreamObserver.class);
        grpcService.getUserCommands(UserRequest.newBuilder().setUserId(4L).build(), obs);

        ArgumentCaptor<ListCommandsResponse> captor = ArgumentCaptor.forClass(ListCommandsResponse.class);
        verify(obs).onNext(captor.capture());
        assert captor.getValue().getCommandsCount() == 1;
    }

    @Test
    void upsertHandlers_invalidSecret_propagatesError() {
        doThrow(new StatusRuntimeException(Status.PERMISSION_DENIED))
            .when(commandRbacService).upsertHandlers(anyString(), anyList());

        StreamObserver<Empty> obs = mock(StreamObserver.class);
        grpcService.upsertHandlers(UpsertHandlersRequest.newBuilder()
            .setSecret("wrong").build(), obs);

        verify(obs).onError(any(StatusRuntimeException.class));
        verify(obs, never()).onCompleted();
    }
}
