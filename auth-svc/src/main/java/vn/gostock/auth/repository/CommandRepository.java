package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import vn.gostock.auth.entity.Command;
import java.util.List;

public interface CommandRepository extends JpaRepository<Command, Long> {
    boolean existsByName(String name);

    // All enabled commands whose handler is also enabled — used for super_admin / admin
    // (they may run every command in the catalog).
    @Query(value = "SELECT c.* FROM commands c "
        + "JOIN cli_handlers h ON h.handler_key = c.handler_key "
        + "WHERE c.enabled = true AND h.enabled = true "
        + "ORDER BY c.id", nativeQuery = true)
    List<Command> findAllEnabledWithEnabledHandler();

    // UNION of enabled commands across every command_group the user belongs to,
    // filtered so both the command and its handler are enabled.
    @Query(value = "SELECT DISTINCT c.* FROM commands c "
        + "JOIN cli_handlers h ON h.handler_key = c.handler_key "
        + "JOIN command_group_commands cgc ON cgc.command_id = c.id "
        + "JOIN user_command_groups ucg ON ucg.group_id = cgc.group_id "
        + "WHERE ucg.user_id = :userId AND c.enabled = true AND h.enabled = true "
        + "ORDER BY c.id", nativeQuery = true)
    List<Command> findEnabledCommandsByUserId(@Param("userId") Long userId);
}
