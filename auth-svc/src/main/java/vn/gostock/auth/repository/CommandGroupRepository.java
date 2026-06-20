package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.CommandGroup;
import java.util.Optional;

public interface CommandGroupRepository extends JpaRepository<CommandGroup, Long> {
    boolean existsByName(String name);
    Optional<CommandGroup> findByName(String name);
}
