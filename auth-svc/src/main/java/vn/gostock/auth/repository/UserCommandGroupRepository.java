package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.UserCommandGroup;
import vn.gostock.auth.entity.UserCommandGroupId;
import java.util.List;

public interface UserCommandGroupRepository extends JpaRepository<UserCommandGroup, UserCommandGroupId> {
    List<UserCommandGroup> findByUserId(Long userId);
    List<UserCommandGroup> findByGroupId(Long groupId);
    void deleteByUserIdAndGroupId(Long userId, Long groupId);
}
