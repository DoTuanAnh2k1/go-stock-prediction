package vn.gostock.auth.grpc;

import io.grpc.ForwardingServerCall;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import net.devh.boot.grpc.server.interceptor.GrpcGlobalServerInterceptor;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

/**
 * Global gRPC server interceptor that logs one line per RPC: method, status, duration.
 * Registered globally via {@link GrpcGlobalServerInterceptor}.
 *
 * Example log line: {@code grpc method=AuthService/Login status=OK dur_ms=12}
 */
@Component
@GrpcGlobalServerInterceptor
public class GrpcLoggingInterceptor implements ServerInterceptor {

    private static final Logger log = LoggerFactory.getLogger(GrpcLoggingInterceptor.class);

    @Override
    public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
            ServerCall<ReqT, RespT> call,
            Metadata headers,
            ServerCallHandler<ReqT, RespT> next) {

        // Full method name looks like "vn.gostock.auth.AuthService/Login";
        // strip the package prefix so it reads like "AuthService/Login".
        final String method = shortMethod(call.getMethodDescriptor().getFullMethodName());
        final long startNanos = System.nanoTime();

        ServerCall<ReqT, RespT> wrappedCall =
            new ForwardingServerCall.SimpleForwardingServerCall<>(call) {
                @Override
                public void close(Status status, Metadata trailers) {
                    long durMs = (System.nanoTime() - startNanos) / 1_000_000L;
                    if (status.isOk()) {
                        log.info("grpc method={} status={} dur_ms={}",
                            method, status.getCode(), durMs);
                    } else {
                        log.warn("grpc method={} status={} dur_ms={}",
                            method, status.getCode(), durMs);
                    }
                    super.close(status, trailers);
                }
            };

        return next.startCall(wrappedCall, headers);
    }

    /** Drop the package prefix, keep "Service/Method". */
    private static String shortMethod(String fullMethodName) {
        if (fullMethodName == null) {
            return "unknown";
        }
        int slash = fullMethodName.indexOf('/');
        if (slash < 0) {
            return fullMethodName;
        }
        String service = fullMethodName.substring(0, slash);
        String rpc = fullMethodName.substring(slash + 1);
        int lastDot = service.lastIndexOf('.');
        if (lastDot >= 0) {
            service = service.substring(lastDot + 1);
        }
        return service + "/" + rpc;
    }
}
