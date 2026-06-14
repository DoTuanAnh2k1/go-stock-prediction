package vn.gostock.auth.service;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import vn.gostock.auth.entity.MarketGroup;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.entity.UserMarketGroup;
import vn.gostock.auth.repository.MarketGroupRepository;
import vn.gostock.auth.repository.UserMarketGroupRepository;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Set;

@Service
@RequiredArgsConstructor
public class MarketGroupService {

    private static final Set<String> VALID_MARKET_KEYS = Set.of("GOLD", "NASDAQ", "CRYPTO", "SP500");

    private final MarketGroupRepository marketGroupRepository;
    private final UserMarketGroupRepository userMarketGroupRepository;
    private final UserRepository userRepository;

    public List<MarketGroup> listGroups() {
        return marketGroupRepository.findAll();
    }

    @Transactional
    public MarketGroup createGroup(String callerRole, String name, String description) {
        requireAdminOrSuperAdmin(callerRole);
        if (marketGroupRepository.existsByName(name)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("group name already exists"));
        }
        MarketGroup group = MarketGroup.builder()
            .name(name)
            .description(description)
            .createdAt(LocalDateTime.now())
            .updatedAt(LocalDateTime.now())
            .build();
        return marketGroupRepository.save(group);
    }

    @Transactional
    public MarketGroup updateGroup(String callerRole, long groupId, String name, String description) {
        requireAdminOrSuperAdmin(callerRole);
        MarketGroup group = findGroupOrThrow(groupId);
        if (!group.getName().equals(name) && marketGroupRepository.existsByName(name)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("group name already exists"));
        }
        group.setName(name);
        group.setDescription(description);
        group.setUpdatedAt(LocalDateTime.now());
        return marketGroupRepository.save(group);
    }

    @Transactional
    public void deleteGroup(String callerRole, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        MarketGroup group = findGroupOrThrow(groupId);
        marketGroupRepository.delete(group);
    }

    @Transactional
    public void setGroupMarkets(String callerRole, long groupId, List<String> marketKeys) {
        requireAdminOrSuperAdmin(callerRole);
        for (String key : marketKeys) {
            if (!VALID_MARKET_KEYS.contains(key)) {
                throw new StatusRuntimeException(
                    Status.INVALID_ARGUMENT.withDescription("invalid market key: " + key));
            }
        }
        MarketGroup group = findGroupOrThrow(groupId);
        group.getMarketKeys().clear();
        group.getMarketKeys().addAll(marketKeys);
        group.setUpdatedAt(LocalDateTime.now());
        marketGroupRepository.save(group);
    }

    public List<User> listGroupUsers(String callerRole, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        findGroupOrThrow(groupId);
        List<UserMarketGroup> memberships = userMarketGroupRepository.findByGroupId(groupId);
        return memberships.stream()
            .map(m -> userRepository.findByIdAndDeletedAtIsNull(m.getUserId()))
            .filter(opt -> opt.isPresent())
            .map(opt -> opt.get())
            .toList();
    }

    @Transactional
    public void addUserToGroup(String callerRole, long userId, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        User user = userRepository.findByIdAndDeletedAtIsNull(userId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(user.getRole())) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot assign super_admin to market groups"));
        }
        findGroupOrThrow(groupId);
        UserMarketGroup membership = new UserMarketGroup(userId, groupId);
        if (!userMarketGroupRepository.existsById(new vn.gostock.auth.entity.UserMarketGroupId(userId, groupId))) {
            userMarketGroupRepository.save(membership);
        }
    }

    @Transactional
    public void removeUserFromGroup(String callerRole, long userId, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        userMarketGroupRepository.deleteByUserIdAndGroupId(userId, groupId);
    }

    public List<MarketGroup> getUserMarketGroups(long userId) {
        List<UserMarketGroup> memberships = userMarketGroupRepository.findByUserId(userId);
        return memberships.stream()
            .map(m -> marketGroupRepository.findById(m.getGroupId()))
            .filter(opt -> opt.isPresent())
            .map(opt -> opt.get())
            .toList();
    }

    private MarketGroup findGroupOrThrow(long groupId) {
        return marketGroupRepository.findById(groupId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("market group not found")));
    }

    private void requireAdminOrSuperAdmin(String role) {
        if (!"admin".equals(role) && !"super_admin".equals(role)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("admin access required"));
        }
    }
}
