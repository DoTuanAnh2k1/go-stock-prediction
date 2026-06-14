package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import vn.gostock.auth.entity.UserMarketGroup;
import vn.gostock.auth.entity.UserMarketGroupId;
import java.util.List;

public interface UserMarketGroupRepository extends JpaRepository<UserMarketGroup, UserMarketGroupId> {
    List<UserMarketGroup> findByUserId(Long userId);
    List<UserMarketGroup> findByGroupId(Long groupId);
    void deleteByUserIdAndGroupId(Long userId, Long groupId);

    @Query("""
        SELECT DISTINCT mgm.market_key
        FROM user_market_groups umg
        JOIN market_group_markets mgm ON mgm.group_id = umg.group_id
        WHERE umg.user_id = :userId
        """, nativeQuery = true)
    List<String> findMarketKeysByUserId(Long userId);
}
