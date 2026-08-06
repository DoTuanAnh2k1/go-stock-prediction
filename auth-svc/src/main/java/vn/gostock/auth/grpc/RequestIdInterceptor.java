package vn.gostock.auth.grpc;

import io.grpc.ForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import net.devh.boot.grpc.server.interceptor.GrpcGlobalServerInterceptor;
import org.slf4j.MDC;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;

import java.util.UUID;

/**
 * Global gRPC server interceptor that establishes a correlation id (request id) for every RPC.
 *
 * <p>Contract (identical across all services):
 * <ul>
 *   <li>Inbound gRPC metadata key: {@code x-request-id} (lowercase).</li>
 *   <li>Value is a UUID v4 — read from inbound metadata if present, otherwise minted fresh.</li>
 *   <li>Exposed to logback as MDC key {@code requestId} → rendered as {@code request_id=<id>}
 *       via the {@code %X{requestId:-}} pattern token.</li>
 * </ul>
 *
 * <p>Ordering: this interceptor is registered OUTERMOST relative to
 * {@link GrpcLoggingInterceptor}. net.devh sorts global interceptors by Spring order
 * ascending and applies them via {@code ServerInterceptors.interceptForward}, so the
 * lowest order value becomes the outermost interceptor (first to see an inbound call).
 * {@link GrpcLoggingInterceptor} carries no explicit order, so it defaults to
 * {@link Ordered#LOWEST_PRECEDENCE} (innermost). By setting a near-highest precedence here
 * the MDC is already populated by the time the logging interceptor writes its line.
 *
 * <p>grpc-java may invoke listener callbacks on threads drawn from a pool, so the MDC is
 * (re)asserted around every listener callback and removed after the terminal callback
 * ({@code onComplete}/{@code onCancel}) to guarantee it neither goes missing mid-call nor
 * leaks onto a subsequent call reusing the same thread.
 */
@Component
@GrpcGlobalServerInterceptor
@Order(Ordered.HIGHEST_PRECEDENCE + 1)
public class RequestIdInterceptor implements ServerInterceptor {

    /** MDC key consumed by logback's {@code %X{requestId:-}} token. */
    static final String MDC_KEY = "requestId";

    /** Inbound metadata (gRPC header) carrying the correlation id. */
    static final Metadata.Key<String> REQUEST_ID_HEADER =
            Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);

    @Override
    public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
            ServerCall<ReqT, RespT> call,
            Metadata headers,
            ServerCallHandler<ReqT, RespT> next) {

        final String requestId = resolveRequestId(headers);

        // Populate the MDC for the (synchronous, blocking) handler invocation that
        // startCall triggers, so any logging on the calling thread already sees it.
        MDC.put(MDC_KEY, requestId);
        final ServerCall.Listener<ReqT> delegate;
        try {
            delegate = next.startCall(call, headers);
        } finally {
            // Do not leak the MDC onto whatever else runs on this thread after startCall
            // returns; each listener callback re-asserts it below.
            MDC.remove(MDC_KEY);
        }

        return new ForwardingServerCallListener.SimpleForwardingServerCallListener<>(delegate) {
            @Override
            public void onMessage(ReqT message) {
                MDC.put(MDC_KEY, requestId);
                try {
                    super.onMessage(message);
                } finally {
                    MDC.remove(MDC_KEY);
                }
            }

            @Override
            public void onHalfClose() {
                MDC.put(MDC_KEY, requestId);
                try {
                    super.onHalfClose();
                } finally {
                    MDC.remove(MDC_KEY);
                }
            }

            @Override
            public void onReady() {
                MDC.put(MDC_KEY, requestId);
                try {
                    super.onReady();
                } finally {
                    MDC.remove(MDC_KEY);
                }
            }

            @Override
            public void onComplete() {
                MDC.put(MDC_KEY, requestId);
                try {
                    super.onComplete();
                } finally {
                    MDC.remove(MDC_KEY);
                }
            }

            @Override
            public void onCancel() {
                MDC.put(MDC_KEY, requestId);
                try {
                    super.onCancel();
                } finally {
                    MDC.remove(MDC_KEY);
                }
            }
        };
    }

    /** Read {@code x-request-id} if present and non-blank, otherwise mint a UUID v4. */
    private static String resolveRequestId(Metadata headers) {
        String incoming = headers.get(REQUEST_ID_HEADER);
        if (incoming != null && !incoming.isBlank()) {
            return incoming.trim();
        }
        return UUID.randomUUID().toString();
    }
}
