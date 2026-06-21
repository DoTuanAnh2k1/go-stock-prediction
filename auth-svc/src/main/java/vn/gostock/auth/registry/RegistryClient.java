package vn.gostock.auth.registry;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import jakarta.annotation.PreDestroy;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import vn.gostock.registry.proto.HeartbeatRequest;
import vn.gostock.registry.proto.HeartbeatResponse;
import vn.gostock.registry.proto.RegisterRequest;
import vn.gostock.registry.proto.RegisterResponse;
import vn.gostock.registry.proto.RegistryGrpc;

@Component
public class RegistryClient {

    private static final Logger log = LoggerFactory.getLogger(RegistryClient.class);

    private static final String SERVICE_NAME = "auth-svc";
    private static final int TTL_SECONDS = 30;

    @Value("${service-mgt.enabled:false}")
    private boolean enabled;

    @Value("${service-mgt.target:service-mgt:8121}")
    private String target;

    @Value("${grpc.server.port:8120}")
    private int advertisedPort;

    private ManagedChannel channel;
    private RegistryGrpc.RegistryBlockingStub stub;
    private volatile String instanceId;

    @EventListener(ApplicationReadyEvent.class)
    public void onReady() {
        if (!enabled) {
            log.info("registry disabled, skipping registration");
            return;
        }
        channel = ManagedChannelBuilder.forTarget(target).usePlaintext().build();
        stub = RegistryGrpc.newBlockingStub(channel);
        log.info("registry client enabled target={} advertise={}:{}", target, SERVICE_NAME, advertisedPort);
        register();
    }

    private void register() {
        try {
            RegisterResponse resp = stub.register(RegisterRequest.newBuilder()
                .setServiceName(SERVICE_NAME)
                .setAddress(SERVICE_NAME)
                .setPort(advertisedPort)
                .setTtlSeconds(TTL_SECONDS)
                .build());
            instanceId = resp.getInstanceId();
            log.info("registered with service-mgt instance_id={} lease_ttl={}s",
                instanceId, resp.getLeaseTtlSeconds());
        } catch (Exception e) {
            log.warn("registration with service-mgt failed, will retry on heartbeat: {}", e.getMessage());
        }
    }

    @Scheduled(fixedDelay = 10000)
    public void heartbeat() {
        if (!enabled || stub == null) {
            return;
        }
        if (instanceId == null) {
            register();
            return;
        }
        try {
            HeartbeatResponse resp = stub.heartbeat(HeartbeatRequest.newBuilder()
                .setInstanceId(instanceId)
                .build());
            if (!resp.getOk()) {
                log.warn("heartbeat returned not-ok, re-registering instance_id={}", instanceId);
                instanceId = null;
                register();
            }
        } catch (StatusRuntimeException e) {
            if (e.getStatus().getCode() == Status.Code.NOT_FOUND) {
                log.warn("heartbeat instance not found, re-registering instance_id={}", instanceId);
                instanceId = null;
                register();
            } else {
                log.warn("heartbeat failed: {}", e.getMessage());
            }
        } catch (Exception e) {
            log.warn("heartbeat failed: {}", e.getMessage());
        }
    }

    @PreDestroy
    public void onShutdown() {
        if (!enabled) {
            return;
        }
        try {
            if (stub != null && instanceId != null) {
                stub.deregister(vn.gostock.registry.proto.DeregisterRequest.newBuilder()
                    .setInstanceId(instanceId)
                    .build());
                log.info("deregistered from service-mgt instance_id={}", instanceId);
            }
        } catch (Exception e) {
            log.warn("deregister failed: {}", e.getMessage());
        } finally {
            if (channel != null) {
                channel.shutdownNow();
            }
        }
    }
}
