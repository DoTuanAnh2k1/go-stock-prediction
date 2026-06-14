package vn.gostock.auth.seeder;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Component;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;

@Component
@RequiredArgsConstructor
@Slf4j
public class SuperAdminSeeder implements ApplicationRunner {

    private static final String SUPER_ADMIN_USERNAME = "chon";
    private static final String SUPER_ADMIN_PASSWORD = "Ch1nch2n@";

    private final UserRepository userRepository;
    private final PasswordEncoder passwordEncoder;

    @Override
    public void run(ApplicationArguments args) {
        boolean exists = userRepository.existsByUsernameAndRoleAndDeletedAtIsNull(
            SUPER_ADMIN_USERNAME, "super_admin");
        if (!exists) {
            User superAdmin = User.builder()
                .username(SUPER_ADMIN_USERNAME)
                .passwordHash(passwordEncoder.encode(SUPER_ADMIN_PASSWORD))
                .role("super_admin")
                .createdAt(LocalDateTime.now())
                .updatedAt(LocalDateTime.now())
                .build();
            userRepository.save(superAdmin);
            log.info("Super admin '{}' seeded successfully", SUPER_ADMIN_USERNAME);
        }
    }
}
