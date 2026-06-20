package vn.gostock.auth.service;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import vn.gostock.auth.entity.CliHandler;
import vn.gostock.auth.entity.Command;
import vn.gostock.auth.entity.CommandGroup;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.entity.UserCommandGroup;
import vn.gostock.auth.entity.UserCommandGroupId;
import vn.gostock.auth.repository.CliHandlerRepository;
import vn.gostock.auth.repository.CommandGroupRepository;
import vn.gostock.auth.repository.CommandRepository;
import vn.gostock.auth.repository.UserCommandGroupRepository;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;
import java.util.List;

@Service
@RequiredArgsConstructor
public class CommandRbacService {

    // Shared secret cli-svc must present to upsert the handler catalog.
    @Value("${INTERNAL_SECRET:}")
    private String internalSecret;

    private final CliHandlerRepository cliHandlerRepository;
    private final CommandRepository commandRepository;
    private final CommandGroupRepository commandGroupRepository;
    private final UserCommandGroupRepository userCommandGroupRepository;
    private final UserRepository userRepository;

    // ── Handler catalog ───────────────────────────────────────────────────

    @Transactional
    public void upsertHandlers(String secret, List<CliHandler> handlers) {
        if (internalSecret == null || internalSecret.isEmpty() || !internalSecret.equals(secret)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("invalid internal secret"));
        }
        for (CliHandler incoming : handlers) {
            CliHandler handler = cliHandlerRepository.findById(incoming.getHandlerKey())
                .orElseGet(CliHandler::new);
            handler.setHandlerKey(incoming.getHandlerKey());
            handler.setDisplayName(incoming.getDisplayName());
            handler.setVerb(incoming.getVerb());
            handler.setResource(incoming.getResource());
            handler.setArgSchema(emptyToDefault(incoming.getArgSchema(), "[]"));
            handler.setEnabled(incoming.getEnabled() == null ? Boolean.TRUE : incoming.getEnabled());
            cliHandlerRepository.save(handler);
        }
    }

    public List<CliHandler> listHandlers() {
        return cliHandlerRepository.findAll();
    }

    // ── Commands ──────────────────────────────────────────────────────────

    public List<Command> listCommands() {
        return commandRepository.findAll();
    }

    @Transactional
    public Command createCommand(String callerRole, String name, String description,
                                 String handlerKey, String args, boolean enabled) {
        requireAdminOrSuperAdmin(callerRole);
        if (commandRepository.existsByName(name)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("command name already exists"));
        }
        requireHandlerExists(handlerKey);
        Command command = Command.builder()
            .name(name)
            .description(description)
            .handlerKey(handlerKey)
            .args(emptyToDefault(args, "{}"))
            .enabled(enabled)
            .createdAt(LocalDateTime.now())
            .updatedAt(LocalDateTime.now())
            .build();
        return commandRepository.save(command);
    }

    @Transactional
    public Command updateCommand(String callerRole, long commandId, String name, String description,
                                 String handlerKey, String args, boolean enabled) {
        requireAdminOrSuperAdmin(callerRole);
        Command command = findCommandOrThrow(commandId);
        if (!command.getName().equals(name) && commandRepository.existsByName(name)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("command name already exists"));
        }
        requireHandlerExists(handlerKey);
        // Full replace of fields.
        command.setName(name);
        command.setDescription(description);
        command.setHandlerKey(handlerKey);
        command.setArgs(emptyToDefault(args, "{}"));
        command.setEnabled(enabled);
        command.setUpdatedAt(LocalDateTime.now());
        return commandRepository.save(command);
    }

    @Transactional
    public void deleteCommand(String callerRole, long commandId) {
        requireAdminOrSuperAdmin(callerRole);
        Command command = findCommandOrThrow(commandId);
        commandRepository.delete(command);
    }

    // ── Command groups ────────────────────────────────────────────────────

    public List<CommandGroup> listGroups() {
        return commandGroupRepository.findAll();
    }

    @Transactional
    public CommandGroup createGroup(String callerRole, String name, String description) {
        requireAdminOrSuperAdmin(callerRole);
        if (commandGroupRepository.existsByName(name)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("group name already exists"));
        }
        CommandGroup group = CommandGroup.builder()
            .name(name)
            .description(description)
            .createdAt(LocalDateTime.now())
            .updatedAt(LocalDateTime.now())
            .build();
        return commandGroupRepository.save(group);
    }

    @Transactional
    public CommandGroup updateGroup(String callerRole, long groupId, String name, String description) {
        requireAdminOrSuperAdmin(callerRole);
        CommandGroup group = findGroupOrThrow(groupId);
        if (!group.getName().equals(name) && commandGroupRepository.existsByName(name)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("group name already exists"));
        }
        group.setName(name);
        group.setDescription(description);
        group.setUpdatedAt(LocalDateTime.now());
        return commandGroupRepository.save(group);
    }

    @Transactional
    public void deleteGroup(String callerRole, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        CommandGroup group = findGroupOrThrow(groupId);
        commandGroupRepository.delete(group);
    }

    @Transactional
    public void setGroupCommands(String callerRole, long groupId, List<Long> commandIds) {
        requireAdminOrSuperAdmin(callerRole);
        for (Long commandId : commandIds) {
            if (!commandRepository.existsById(commandId)) {
                throw new StatusRuntimeException(
                    Status.INVALID_ARGUMENT.withDescription("invalid command id: " + commandId));
            }
        }
        CommandGroup group = findGroupOrThrow(groupId);
        // Full replace of membership.
        group.getCommandIds().clear();
        group.getCommandIds().addAll(commandIds);
        group.setUpdatedAt(LocalDateTime.now());
        commandGroupRepository.save(group);
    }

    public List<User> listGroupUsers(String callerRole, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        findGroupOrThrow(groupId);
        List<UserCommandGroup> memberships = userCommandGroupRepository.findByGroupId(groupId);
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
                Status.PERMISSION_DENIED.withDescription("cannot assign super_admin to command groups"));
        }
        findGroupOrThrow(groupId);
        UserCommandGroup membership = new UserCommandGroup(userId, groupId);
        if (!userCommandGroupRepository.existsById(new UserCommandGroupId(userId, groupId))) {
            userCommandGroupRepository.save(membership);
        }
    }

    @Transactional
    public void removeUserFromGroup(String callerRole, long userId, long groupId) {
        requireAdminOrSuperAdmin(callerRole);
        userCommandGroupRepository.deleteByUserIdAndGroupId(userId, groupId);
    }

    // ── Enforcement ───────────────────────────────────────────────────────

    /**
     * Commands a user is allowed to execute.
     *  - super_admin / admin → all enabled commands (whose handler is also enabled).
     *  - user (or any other role) → UNION of enabled commands across the command groups
     *    the user belongs to. User with no groups → empty list.
     */
    public List<Command> getUserCommands(long userId) {
        User user = userRepository.findByIdAndDeletedAtIsNull(userId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(user.getRole()) || "admin".equals(user.getRole())) {
            return commandRepository.findAllEnabledWithEnabledHandler();
        }
        return commandRepository.findEnabledCommandsByUserId(userId);
    }

    // ── Helpers ───────────────────────────────────────────────────────────

    private Command findCommandOrThrow(long commandId) {
        return commandRepository.findById(commandId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("command not found")));
    }

    private CommandGroup findGroupOrThrow(long groupId) {
        return commandGroupRepository.findById(groupId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("command group not found")));
    }

    private void requireHandlerExists(String handlerKey) {
        if (!cliHandlerRepository.existsById(handlerKey)) {
            throw new StatusRuntimeException(
                Status.INVALID_ARGUMENT.withDescription("unknown handler_key: " + handlerKey));
        }
    }

    private void requireAdminOrSuperAdmin(String role) {
        if (!"admin".equals(role) && !"super_admin".equals(role)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("admin access required"));
        }
    }

    private static String emptyToDefault(String s, String def) {
        return (s == null || s.isEmpty()) ? def : s;
    }
}
