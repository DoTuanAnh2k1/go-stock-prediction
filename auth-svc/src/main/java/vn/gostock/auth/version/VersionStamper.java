package vn.gostock.auth.version;

import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;

/**
 * Writes a version stamp file and logs a single startup version line.
 * Reads GIT_SHA / BUILD_TIME / GIT_DIRTY env vars injected at Docker build time.
 */
@Component
@Order(0)
@Slf4j
public class VersionStamper implements ApplicationRunner {

    private static final String SERVICE_NAME = "auth-svc";
    private static final Path VERSIONS_DIR = Paths.get("/versions");
    private static final Path VERSION_FILE = VERSIONS_DIR.resolve("auth-svc.json");

    @Override
    public void run(ApplicationArguments args) {
        String gitSha = envOrUnknown("GIT_SHA");
        String buildTime = envOrUnknown("BUILD_TIME");
        String dirty = envOrUnknown("GIT_DIRTY");
        String startedAt = LocalDateTime.now().format(DateTimeFormatter.ISO_LOCAL_DATE_TIME);

        log.info("version git_sha={} build_time={} dirty={}", gitSha, buildTime, dirty);

        String json = "{"
            + "\"service\":\"" + SERVICE_NAME + "\","
            + "\"git_sha\":\"" + escape(gitSha) + "\","
            + "\"build_time\":\"" + escape(buildTime) + "\","
            + "\"dirty\":\"" + escape(dirty) + "\","
            + "\"started_at\":\"" + escape(startedAt) + "\""
            + "}";

        try {
            Files.createDirectories(VERSIONS_DIR);
            Files.writeString(VERSION_FILE, json);
        } catch (Exception e) {
            log.warn("Could not write version stamp to {}: {}", VERSION_FILE, e.getMessage());
        }
    }

    private static String envOrUnknown(String key) {
        String value = System.getenv(key);
        return (value == null || value.isEmpty()) ? "unknown" : value;
    }

    private static String escape(String s) {
        return s.replace("\\", "\\\\").replace("\"", "\\\"");
    }
}
