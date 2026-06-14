package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.User;
import java.util.List;
import java.util.Optional;

public interface UserRepository extends JpaRepository<User, Long> {
    Optional<User> findByUsernameAndDeletedAtIsNull(String username);
    Optional<User> findByIdAndDeletedAtIsNull(Long id);
    List<User> findAllByDeletedAtIsNull();
    boolean existsByUsernameAndDeletedAtIsNull(String username);
    boolean existsByUsernameAndRoleAndDeletedAtIsNull(String username, String role);
}
