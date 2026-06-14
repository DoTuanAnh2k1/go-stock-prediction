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
        when(userService.createUser("admin", "newuser", "pass", "user")).thenReturn(
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
}
