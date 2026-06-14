package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.MarketGroup;
import java.util.Optional;

public interface MarketGroupRepository extends JpaRepository<MarketGroup, Long> {
    boolean existsByName(String name);
    Optional<MarketGroup> findByName(String name);
}
