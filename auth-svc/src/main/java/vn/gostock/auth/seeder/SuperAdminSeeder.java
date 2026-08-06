package vn.gostock.auth.seeder;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Component;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;
import java.util.Set;

/**
 * Seeds the single super_admin from env — NEVER hardcoded. Credentials arrive via
 * SUPER_ADMIN_USERNAME / SUPER_ADMIN_PASSWORD (Secret at deploy time). Fail-fast on a
 * blank or well-known-weak password so a cluster can never boot with a guessable owner.
 */
@Component
@RequiredArgsConstructor
@Slf4j
public class SuperAdminSeeder implements ApplicationRunner {

    /** Passwords we refuse to seed — forces a real secret to be supplied. */
    private static final Set<String> WEAK_PASSWORDS = Set.of(
        "123", "admin", "admin123", "password", "changeme", "change-me",
        "change-me-in-dev", "change-me-in-production");

    @Value("${superadmin.username:}")
    private String superAdminUsername;

    @Value("${superadmin.password:}")
    private String superAdminPassword;

    private final UserRepository userRepository;
    private final PasswordEncoder passwordEncoder;

    @Override
    public void run(ApplicationArguments args) {
        final String username = superAdminUsername == null ? "" : superAdminUsername.trim();
        final String password = superAdminPassword == null ? "" : superAdminPassword;

        if (username.isEmpty() || password.isEmpty()) {
            throw new IllegalStateException(
                "super_admin seed refused: set SUPER_ADMIN_USERNAME and SUPER_ADMIN_PASSWORD "
                + "(no defaults are shipped).");
        }
        if (WEAK_PASSWORDS.contains(password.toLowerCase())) {
            throw new IllegalStateException(
                "super_admin seed refused: SUPER_ADMIN_PASSWORD is a well-known weak value; "
                + "supply a strong secret.");
        }

        boolean exists = userRepository.existsByUsernameAndRoleAndDeletedAtIsNull(
            username, "super_admin");
        if (!exists) {
            User superAdmin = User.builder()
                .username(username)
                .passwordHash(passwordEncoder.encode(password))
                .role("super_admin")
                .createdAt(LocalDateTime.now())
                .updatedAt(LocalDateTime.now())
                .build();
            userRepository.save(superAdmin);
            log.info("Super admin '{}' seeded successfully", username);   // never log the password
        }
    }
}
