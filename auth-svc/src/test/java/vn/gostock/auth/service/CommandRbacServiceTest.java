package vn.gostock.auth.service;

import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;
import vn.gostock.auth.entity.CliHandler;
import vn.gostock.auth.entity.Command;
import vn.gostock.auth.entity.CommandGroup;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.repository.CliHandlerRepository;
import vn.gostock.auth.repository.CommandGroupRepository;
import vn.gostock.auth.repository.CommandRepository;
import vn.gostock.auth.repository.UserCommandGroupRepository;
import vn.gostock.auth.repository.UserRepository;

import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class CommandRbacServiceTest {

    @Mock CliHandlerRepository cliHandlerRepository;
    @Mock CommandRepository commandRepository;
    @Mock CommandGroupRepository commandGroupRepository;
    @Mock UserCommandGroupRepository userCommandGroupRepository;
    @Mock UserRepository userRepository;

    @InjectMocks CommandRbacService service;

    @BeforeEach
    void setUp() {
        ReflectionTestUtils.setField(service, "internalSecret", "topsecret");
    }

    // ── Command CRUD ──────────────────────────────────────────────────────

    @Test
    void createCommand_adminCaller_persistsCommand() {
        when(commandRepository.existsByName("c1")).thenReturn(false);
        when(cliHandlerRepository.existsById("market.latest")).thenReturn(true);
        when(commandRepository.save(any(Command.class)))
            .thenAnswer(inv -> inv.getArgument(0));

        Command c = service.createCommand("admin", "c1", "desc",
            "market.latest", "{\"market\":\"gold\"}", true);

        assertEquals("c1", c.getName());
        assertEquals("market.latest", c.getHandlerKey());
        assertEquals("{\"market\":\"gold\"}", c.getArgs());
        assertTrue(c.getEnabled());
        verify(commandRepository).save(any(Command.class));
    }

    @Test
    void createCommand_nonAdmin_throwsPermissionDenied() {
        assertThrows(StatusRuntimeException.class,
            () -> service.createCommand("user", "c1", "d", "market.latest", "{}", true));
        verify(commandRepository, never()).save(any());
    }

    @Test
    void createCommand_duplicateName_throwsAlreadyExists() {
        when(commandRepository.existsByName("dup")).thenReturn(true);
        assertThrows(StatusRuntimeException.class,
            () -> service.createCommand("admin", "dup", "d", "market.latest", "{}", true));
    }

    @Test
    void createCommand_unknownHandler_throwsInvalidArgument() {
        when(commandRepository.existsByName("c1")).thenReturn(false);
        when(cliHandlerRepository.existsById("nope")).thenReturn(false);
        assertThrows(StatusRuntimeException.class,
            () -> service.createCommand("admin", "c1", "d", "nope", "{}", true));
    }

    @Test
    void updateCommand_fullReplace_updatesAllFields() {
        Command existing = Command.builder()
            .id(5L).name("old").description("oldd").handlerKey("market.latest")
            .args("{}").enabled(true).build();
        when(commandRepository.findById(5L)).thenReturn(Optional.of(existing));
        when(commandRepository.existsByName("new")).thenReturn(false);
        when(cliHandlerRepository.existsById("market.prices")).thenReturn(true);
        when(commandRepository.save(any(Command.class)))
            .thenAnswer(inv -> inv.getArgument(0));

        Command updated = service.updateCommand("super_admin", 5L, "new", "newd",
            "market.prices", "{\"limit\":10}", false);

        assertEquals("new", updated.getName());
        assertEquals("newd", updated.getDescription());
        assertEquals("market.prices", updated.getHandlerKey());
        assertEquals("{\"limit\":10}", updated.getArgs());
        assertFalse(updated.getEnabled());
    }

    @Test
    void deleteCommand_notFound_throwsNotFound() {
        when(commandRepository.findById(99L)).thenReturn(Optional.empty());
        assertThrows(StatusRuntimeException.class,
            () -> service.deleteCommand("admin", 99L));
    }

    // ── Group membership / SetGroupCommands ───────────────────────────────

    @Test
    void setGroupCommands_replacesMembership() {
        CommandGroup group = CommandGroup.builder().id(1L).name("g").build();
        group.getCommandIds().add(7L); // pre-existing membership that must be replaced
        when(commandRepository.existsById(10L)).thenReturn(true);
        when(commandRepository.existsById(20L)).thenReturn(true);
        when(commandGroupRepository.findById(1L)).thenReturn(Optional.of(group));
        when(commandGroupRepository.save(any(CommandGroup.class)))
            .thenAnswer(inv -> inv.getArgument(0));

        service.setGroupCommands("admin", 1L, List.of(10L, 20L));

        assertEquals(List.of(10L, 20L), group.getCommandIds());
    }

    @Test
    void setGroupCommands_invalidCommandId_throwsInvalidArgument() {
        when(commandRepository.existsById(404L)).thenReturn(false);
        assertThrows(StatusRuntimeException.class,
            () -> service.setGroupCommands("admin", 1L, List.of(404L)));
    }

    @Test
    void addUserToGroup_superAdminTarget_throwsPermissionDenied() {
        when(userRepository.findByIdAndDeletedAtIsNull(2L))
            .thenReturn(Optional.of(User.builder().id(2L).role("super_admin").build()));
        assertThrows(StatusRuntimeException.class,
            () -> service.addUserToGroup("admin", 2L, 1L));
    }

    // ── GetUserCommands — three role cases ────────────────────────────────

    @Test
    void getUserCommands_superAdmin_returnsAllEnabled() {
        when(userRepository.findByIdAndDeletedAtIsNull(1L))
            .thenReturn(Optional.of(User.builder().id(1L).role("super_admin").build()));
        when(commandRepository.findAllEnabledWithEnabledHandler())
            .thenReturn(List.of(Command.builder().id(1L).name("a").build(),
                                Command.builder().id(2L).name("b").build()));

        List<Command> result = service.getUserCommands(1L);

        assertEquals(2, result.size());
        verify(commandRepository).findAllEnabledWithEnabledHandler();
        verify(commandRepository, never()).findEnabledCommandsByUserId(anyLong());
    }

    @Test
    void getUserCommands_admin_returnsAllEnabled() {
        when(userRepository.findByIdAndDeletedAtIsNull(3L))
            .thenReturn(Optional.of(User.builder().id(3L).role("admin").build()));
        when(commandRepository.findAllEnabledWithEnabledHandler())
            .thenReturn(List.of(Command.builder().id(1L).name("a").build()));

        List<Command> result = service.getUserCommands(3L);

        assertEquals(1, result.size());
        verify(commandRepository).findAllEnabledWithEnabledHandler();
    }

    @Test
    void getUserCommands_userWithGroups_returnsUnion() {
        when(userRepository.findByIdAndDeletedAtIsNull(4L))
            .thenReturn(Optional.of(User.builder().id(4L).role("user").build()));
        when(commandRepository.findEnabledCommandsByUserId(4L))
            .thenReturn(List.of(Command.builder().id(10L).name("x").build(),
                                Command.builder().id(11L).name("y").build()));

        List<Command> result = service.getUserCommands(4L);

        assertEquals(2, result.size());
        verify(commandRepository).findEnabledCommandsByUserId(4L);
        verify(commandRepository, never()).findAllEnabledWithEnabledHandler();
    }

    @Test
    void getUserCommands_userNoGroups_returnsEmpty() {
        when(userRepository.findByIdAndDeletedAtIsNull(5L))
            .thenReturn(Optional.of(User.builder().id(5L).role("user").build()));
        when(commandRepository.findEnabledCommandsByUserId(5L)).thenReturn(List.of());

        List<Command> result = service.getUserCommands(5L);

        assertTrue(result.isEmpty());
    }

    @Test
    void getUserCommands_userNotFound_throwsNotFound() {
        when(userRepository.findByIdAndDeletedAtIsNull(6L)).thenReturn(Optional.empty());
        assertThrows(StatusRuntimeException.class, () -> service.getUserCommands(6L));
    }

    // ── UpsertHandlers — secret enforcement ───────────────────────────────

    @Test
    void upsertHandlers_validSecret_upsertsEach() {
        CliHandler h = CliHandler.builder()
            .handlerKey("market.latest").displayName("Latest").verb("get")
            .resource("market").argSchema("[\"market\"]").enabled(true).build();
        when(cliHandlerRepository.findById("market.latest")).thenReturn(Optional.empty());
        when(cliHandlerRepository.save(any(CliHandler.class)))
            .thenAnswer(inv -> inv.getArgument(0));

        service.upsertHandlers("topsecret", List.of(h));

        verify(cliHandlerRepository).save(any(CliHandler.class));
    }

    @Test
    void upsertHandlers_wrongSecret_throwsPermissionDenied() {
        assertThrows(StatusRuntimeException.class,
            () -> service.upsertHandlers("wrong", List.of()));
        verify(cliHandlerRepository, never()).save(any());
    }

    @Test
    void upsertHandlers_emptyConfiguredSecret_rejectsEvenMatchingEmpty() {
        ReflectionTestUtils.setField(service, "internalSecret", "");
        assertThrows(StatusRuntimeException.class,
            () -> service.upsertHandlers("", List.of()));
        verify(cliHandlerRepository, never()).save(any());
    }
}
