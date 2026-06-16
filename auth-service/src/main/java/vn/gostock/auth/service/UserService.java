package vn.gostock.auth.service;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import lombok.RequiredArgsConstructor;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.repository.UserMarketGroupRepository;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;
import java.util.List;

@Service
@RequiredArgsConstructor
public class UserService {

    private static final List<String> ALL_MARKETS = List.of("GOLD", "NASDAQ", "CRYPTO", "SP500");
    private static final List<String> VALID_ROLES  = List.of("super_admin", "admin", "user");

    private final UserRepository userRepository;
    private final UserMarketGroupRepository userMarketGroupRepository;
    private final PasswordEncoder passwordEncoder;

    public User authenticate(String username, String password) {
        User user = userRepository.findByUsernameAndDeletedAtIsNull(username)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.UNAUTHENTICATED.withDescription("invalid credentials")));
        if (!passwordEncoder.matches(password, user.getPasswordHash())) {
            throw new StatusRuntimeException(
                Status.UNAUTHENTICATED.withDescription("invalid credentials"));
        }
        return user;
    }

    public List<String> getAccessibleMarkets(User user) {
        if ("super_admin".equals(user.getRole())) {
            return ALL_MARKETS;
        }
        List<String> markets = userMarketGroupRepository.findMarketKeysByUserId(user.getId());
        return markets.isEmpty() ? List.of() : markets;
    }

    public List<User> listUsers() {
        return userRepository.findAllByDeletedAtIsNull();
    }

    @Transactional
    public User createUser(String callerRole, String username, String password, String role,
                           String fullName, String email, String phone) {
        requireAdminOrSuperAdmin(callerRole);
        String effectiveRole = VALID_ROLES.contains(role) ? role : "user";
        if ("super_admin".equals(effectiveRole) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("only super_admin can create super_admin"));
        }
        if (userRepository.existsByUsernameAndDeletedAtIsNull(username)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("username already taken"));
        }
        User user = User.builder()
            .username(username)
            .passwordHash(passwordEncoder.encode(password))
            .role(effectiveRole)
            .fullName(emptyToNull(fullName))
            .email(emptyToNull(email))
            .phone(emptyToNull(phone))
            .createdAt(LocalDateTime.now())
            .updatedAt(LocalDateTime.now())
            .build();
        return userRepository.save(user);
    }

    @Transactional
    public void resetPassword(String callerRole, long targetId, String newPassword) {
        requireAdminOrSuperAdmin(callerRole);
        User target = userRepository.findByIdAndDeletedAtIsNull(targetId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(target.getRole())) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot reset super_admin password"));
        }
        if ("admin".equals(callerRole) && !"user".equals(target.getRole())) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("admin can only reset user passwords"));
        }
        if (newPassword == null || newPassword.length() < 6) {
            throw new StatusRuntimeException(
                Status.INVALID_ARGUMENT.withDescription("new password must be at least 6 characters"));
        }
        target.setPasswordHash(passwordEncoder.encode(newPassword));
        target.setUpdatedAt(LocalDateTime.now());
        userRepository.save(target);
    }

    @Transactional
    public void deleteUser(String callerRole, long targetId) {
        requireAdminOrSuperAdmin(callerRole);
        User target = userRepository.findByIdAndDeletedAtIsNull(targetId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(target.getRole()) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot delete super_admin"));
        }
        target.setDeletedAt(LocalDateTime.now());
        userRepository.save(target);
    }

    @Transactional
    public User updateRole(String callerRole, long targetId, String newRole) {
        requireAdminOrSuperAdmin(callerRole);
        if (!VALID_ROLES.contains(newRole)) {
            throw new StatusRuntimeException(
                Status.INVALID_ARGUMENT.withDescription("invalid role: " + newRole));
        }
        User target = userRepository.findByIdAndDeletedAtIsNull(targetId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(target.getRole()) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot modify super_admin"));
        }
        if ("super_admin".equals(newRole) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("only super_admin can assign super_admin role"));
        }
        target.setRole(newRole);
        target.setUpdatedAt(LocalDateTime.now());
        return userRepository.save(target);
    }

    @Transactional
    public User updateUser(String callerRole, long targetId, String fullName, String email,
                           String phone, String role) {
        requireAdminOrSuperAdmin(callerRole);
        User target = userRepository.findByIdAndDeletedAtIsNull(targetId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(target.getRole()) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot modify super_admin"));
        }
        if (role != null && !role.isBlank()) {
            if (!VALID_ROLES.contains(role)) {
                throw new StatusRuntimeException(
                    Status.INVALID_ARGUMENT.withDescription("invalid role: " + role));
            }
            if ("super_admin".equals(role) && !"super_admin".equals(callerRole)) {
                throw new StatusRuntimeException(
                    Status.PERMISSION_DENIED.withDescription("only super_admin can assign super_admin role"));
            }
            target.setRole(role);
        }
        target.setFullName(emptyToNull(fullName));
        target.setEmail(emptyToNull(email));
        target.setPhone(emptyToNull(phone));
        target.setUpdatedAt(LocalDateTime.now());
        return userRepository.save(target);
    }

    @Transactional
    public void changePassword(long callerId, String oldPassword, String newPassword) {
        User user = userRepository.findByIdAndDeletedAtIsNull(callerId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if (!passwordEncoder.matches(oldPassword, user.getPasswordHash())) {
            throw new StatusRuntimeException(
                Status.UNAUTHENTICATED.withDescription("current password is incorrect"));
        }
        if (newPassword.length() < 12) {
            throw new StatusRuntimeException(
                Status.INVALID_ARGUMENT.withDescription("new password must be at least 12 characters"));
        }
        user.setPasswordHash(passwordEncoder.encode(newPassword));
        user.setUpdatedAt(LocalDateTime.now());
        userRepository.save(user);
    }

    private static String emptyToNull(String s) {
        return (s == null || s.isEmpty()) ? null : s;
    }

    private void requireAdminOrSuperAdmin(String role) {
        if (!"admin".equals(role) && !"super_admin".equals(role)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("admin access required"));
        }
    }
}
