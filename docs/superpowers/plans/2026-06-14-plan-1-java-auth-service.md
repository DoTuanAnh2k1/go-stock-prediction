# Java Auth Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây dựng Java microservice xử lý toàn bộ auth/RBAC/market-groups qua gRPC, chia sẻ MySQL với Go API.

**Architecture:** Spring Boot 3 + grpc-spring-boot-starter expose gRPC trên port 8120. Flyway tạo 3 bảng mới (`market_groups`, `market_group_markets`, `user_market_groups`) trong DB `go_stock_prediction`. Java owns toàn bộ logic user/auth; Go API chỉ validate JWT local và proxy HTTP→gRPC.

**Tech Stack:** Java 21, Spring Boot 3.3, Spring Data JPA, net.devh grpc-spring-boot-starter 3.1.0, com.auth0 java-jwt 4.4, Flyway 10, BCrypt, MySQL Connector/J, JUnit 5 + Mockito, Maven.

**Prerequisite:** Docker stack đang chạy với MySQL healthy (`docker compose ps db`).

---

## File Map

### Tạo mới
```
auth-service/
├── pom.xml
├── Dockerfile
├── src/main/
│   ├── proto/
│   │   └── auth.proto                          ← canonical proto (copy sang api/proto/auth/)
│   ├── resources/
│   │   ├── application.yml
│   │   └── db/migration/
│   │       └── V1__create_auth_tables.sql
│   └── java/vn/gostock/auth/
│       ├── AuthServiceApplication.java
│       ├── config/
│       │   └── JwtConfig.java
│       ├── entity/
│       │   ├── User.java
│       │   ├── MarketGroup.java
│       │   └── UserMarketGroup.java
│       ├── repository/
│       │   ├── UserRepository.java
│       │   ├── MarketGroupRepository.java
│       │   └── UserMarketGroupRepository.java
│       ├── service/
│       │   ├── JwtService.java
│       │   ├── UserService.java
│       │   └── MarketGroupService.java
│       ├── grpc/
│       │   └── AuthGrpcServiceImpl.java
│       └── seeder/
│           └── SuperAdminSeeder.java
└── src/test/java/vn/gostock/auth/
    └── grpc/
        └── AuthGrpcServiceImplTest.java
```

### Copy để Go dùng
```
api/proto/auth/auth.proto     ← copy từ auth-service/src/main/proto/auth.proto
```

---

## Task 1: pom.xml + Dockerfile

**Files:**
- Create: `auth-service/pom.xml`
- Create: `auth-service/Dockerfile`

- [ ] **Tạo `auth-service/pom.xml`:**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.3.6</version>
        <relativePath/>
    </parent>

    <groupId>vn.gostock</groupId>
    <artifactId>auth-service</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <properties>
        <java.version>21</java.version>
        <grpc.version>1.63.0</grpc.version>
        <grpc-spring-boot.version>3.1.0.RELEASE</grpc-spring-boot.version>
        <protobuf.version>3.25.3</protobuf.version>
        <java-jwt.version>4.4.0</java-jwt.version>
    </properties>

    <dependencies>
        <!-- Spring Data JPA -->
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-data-jpa</artifactId>
        </dependency>

        <!-- MySQL -->
        <dependency>
            <groupId>com.mysql</groupId>
            <artifactId>mysql-connector-j</artifactId>
            <scope>runtime</scope>
        </dependency>

        <!-- gRPC server -->
        <dependency>
            <groupId>net.devh</groupId>
            <artifactId>grpc-server-spring-boot-starter</artifactId>
            <version>${grpc-spring-boot.version}</version>
        </dependency>

        <!-- Protobuf runtime -->
        <dependency>
            <groupId>com.google.protobuf</groupId>
            <artifactId>protobuf-java</artifactId>
            <version>${protobuf.version}</version>
        </dependency>

        <!-- JWT -->
        <dependency>
            <groupId>com.auth0</groupId>
            <artifactId>java-jwt</artifactId>
            <version>${java-jwt.version}</version>
        </dependency>

        <!-- BCrypt -->
        <dependency>
            <groupId>org.springframework.security</groupId>
            <artifactId>spring-security-crypto</artifactId>
        </dependency>

        <!-- Flyway -->
        <dependency>
            <groupId>org.flywaydb</groupId>
            <artifactId>flyway-core</artifactId>
        </dependency>
        <dependency>
            <groupId>org.flywaydb</groupId>
            <artifactId>flyway-mysql</artifactId>
        </dependency>

        <!-- Lombok -->
        <dependency>
            <groupId>org.projectlombok</groupId>
            <artifactId>lombok</artifactId>
            <optional>true</optional>
        </dependency>

        <!-- Test -->
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <extensions>
            <extension>
                <groupId>kr.motd.maven</groupId>
                <artifactId>os-maven-plugin</artifactId>
                <version>1.7.1</version>
            </extension>
        </extensions>
        <plugins>
            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
                <configuration>
                    <excludes>
                        <exclude>
                            <groupId>org.projectlombok</groupId>
                            <artifactId>lombok</artifactId>
                        </exclude>
                    </excludes>
                </configuration>
            </plugin>
            <plugin>
                <groupId>org.xolstice.maven.plugins</groupId>
                <artifactId>protobuf-maven-plugin</artifactId>
                <version>0.6.1</version>
                <configuration>
                    <protocArtifact>com.google.protobuf:protoc:${protobuf.version}:exe:${os.detected.classifier}</protocArtifact>
                    <pluginId>grpc-java</pluginId>
                    <pluginArtifact>io.grpc:protoc-gen-grpc-java:${grpc.version}:exe:${os.detected.classifier}</pluginArtifact>
                </configuration>
                <executions>
                    <execution>
                        <goals>
                            <goal>compile</goal>
                            <goal>compile-custom</goal>
                        </goals>
                    </execution>
                </executions>
            </plugin>
        </plugins>
    </build>
</project>
```

- [ ] **Tạo `auth-service/Dockerfile`:**

```dockerfile
# Stage 1: Build
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests -B

# Stage 2: Runtime
FROM eclipse-temurin:21-jre-slim
WORKDIR /app
COPY --from=builder /app/target/auth-service-*.jar app.jar
ENTRYPOINT ["java", "-jar", "app.jar"]
```

- [ ] **Commit:**
```bash
git add auth-service/pom.xml auth-service/Dockerfile
git commit -m "feat(auth): add Java auth service Maven project skeleton"
```

---

## Task 2: auth.proto

**Files:**
- Create: `auth-service/src/main/proto/auth.proto`
- Create: `api/proto/auth/auth.proto` (copy — Go stubs generated từ đây ở Plan 2)

- [ ] **Tạo `auth-service/src/main/proto/auth.proto`:**

```protobuf
syntax = "proto3";
package auth;

option java_package = "vn.gostock.auth.proto";
option java_multiple_files = true;
option go_package = "go-stock-prediction/proto/auth";

service AuthService {
  // Auth
  rpc Login(LoginRequest)               returns (LoginResponse);
  rpc GetMe(CallerMeta)                 returns (UserResponse);
  rpc ChangePassword(ChangePassRequest) returns (Empty);

  // User management
  rpc ListUsers(CallerMeta)             returns (ListUsersResponse);
  rpc CreateUser(CreateUserRequest)     returns (UserResponse);
  rpc DeleteUser(DeleteUserRequest)     returns (Empty);
  rpc UpdateUserRole(UpdateRoleRequest) returns (UserResponse);

  // Market groups
  rpc ListMarketGroups(CallerMeta)            returns (ListGroupsResponse);
  rpc CreateMarketGroup(CreateGroupRequest)   returns (MarketGroupResponse);
  rpc UpdateMarketGroup(UpdateGroupRequest)   returns (MarketGroupResponse);
  rpc DeleteMarketGroup(DeleteGroupRequest)   returns (Empty);
  rpc SetGroupMarkets(SetGroupMarketsRequest) returns (Empty);
  rpc ListGroupUsers(GroupRequest)            returns (ListUsersResponse);
  rpc AddUserToGroup(UserGroupRequest)        returns (Empty);
  rpc RemoveUserFromGroup(UserGroupRequest)   returns (Empty);
  rpc GetUserMarketGroups(UserRequest)        returns (ListGroupsResponse);
}

message CallerMeta {
  int64  caller_id   = 1;
  string caller_role = 2;
}

message Empty {}

message LoginRequest {
  string username = 1;
  string password = 2;
}

message LoginResponse {
  string token    = 1;
  string username = 2;
  string role     = 3;
}

message UserResponse {
  int64  id         = 1;
  string username   = 2;
  string role       = 3;
  string created_at = 4;
}

message ListUsersResponse {
  repeated UserResponse users = 1;
}

message CreateUserRequest {
  CallerMeta caller   = 1;
  string     username = 2;
  string     password = 3;
  string     role     = 4;
}

message DeleteUserRequest {
  CallerMeta caller    = 1;
  int64      target_id = 2;
}

message UpdateRoleRequest {
  CallerMeta caller    = 1;
  int64      target_id = 2;
  string     new_role  = 3;
}

message ChangePassRequest {
  CallerMeta caller       = 1;
  string     old_password = 2;
  string     new_password = 3;
}

message MarketGroupResponse {
  int64           id          = 1;
  string          name        = 2;
  string          description = 3;
  repeated string market_keys = 4;
  string          created_at  = 5;
  string          updated_at  = 6;
}

message ListGroupsResponse {
  repeated MarketGroupResponse groups = 1;
}

message CreateGroupRequest {
  CallerMeta caller      = 1;
  string     name        = 2;
  string     description = 3;
}

message UpdateGroupRequest {
  CallerMeta caller      = 1;
  int64      group_id    = 2;
  string     name        = 3;
  string     description = 4;
}

message DeleteGroupRequest {
  CallerMeta caller   = 1;
  int64      group_id = 2;
}

message SetGroupMarketsRequest {
  CallerMeta      caller      = 1;
  int64           group_id    = 2;
  repeated string market_keys = 3;
}

message GroupRequest {
  CallerMeta caller   = 1;
  int64      group_id = 2;
}

message UserGroupRequest {
  CallerMeta caller   = 1;
  int64      user_id  = 2;
  int64      group_id = 3;
}

message UserRequest {
  CallerMeta caller  = 1;
  int64      user_id = 2;
}
```

- [ ] **Copy proto sang Go API:**
```bash
mkdir -p api/proto/auth
cp auth-service/src/main/proto/auth.proto api/proto/auth/auth.proto
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/proto/auth.proto api/proto/auth/auth.proto
git commit -m "feat(auth): add auth.proto — defines all Auth gRPC RPCs"
```

---

## Task 3: application.yml + Flyway migration

**Files:**
- Create: `auth-service/src/main/resources/application.yml`
- Create: `auth-service/src/main/resources/db/migration/V1__create_auth_tables.sql`
- Create: `auth-service/src/main/java/vn/gostock/auth/AuthServiceApplication.java`

- [ ] **Tạo `auth-service/src/main/resources/application.yml`:**

```yaml
spring:
  application:
    name: auth-service
  datasource:
    url: jdbc:mysql://${DB_HOST:localhost}:${DB_PORT:3306}/${DB_NAME:go_stock_prediction}?useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Ho_Chi_Minh
    username: ${DB_USER:root}
    password: ${DB_PASSWORD:123}
    driver-class-name: com.mysql.cj.jdbc.Driver
  jpa:
    hibernate:
      ddl-auto: validate        # Flyway owns schema; JPA chỉ validate
    show-sql: false
    properties:
      hibernate:
        dialect: org.hibernate.dialect.MySQLDialect
  flyway:
    enabled: true
    baseline-on-migrate: true
    locations: classpath:db/migration

grpc:
  server:
    port: ${GRPC_PORT:8120}
    security:
      enabled: false            # internal service, no TLS

jwt:
  secret: ${JWT_SECRET:change-me-in-production}
  expiration-ms: 86400000      # 24h

logging:
  level:
    vn.gostock: INFO
    net.devh.boot.grpc: WARN
```

- [ ] **Tạo `auth-service/src/main/resources/db/migration/V1__create_auth_tables.sql`:**

```sql
-- Flyway V1: tạo 3 bảng mới cho market groups
-- Bảng users đã tồn tại (tạo bởi Go GORM), không touch

CREATE TABLE IF NOT EXISTS market_groups (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    created_at  DATETIME     NOT NULL,
    updated_at  DATETIME     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_market_groups_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS market_group_markets (
    group_id    BIGINT      NOT NULL,
    market_key  VARCHAR(20) NOT NULL,
    PRIMARY KEY (group_id, market_key),
    CONSTRAINT fk_mgm_group FOREIGN KEY (group_id)
        REFERENCES market_groups(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_market_groups (
    user_id   BIGINT NOT NULL,
    group_id  BIGINT NOT NULL,
    PRIMARY KEY (user_id, group_id),
    CONSTRAINT fk_umg_group FOREIGN KEY (group_id)
        REFERENCES market_groups(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Tạo `auth-service/src/main/java/vn/gostock/auth/AuthServiceApplication.java`:**

```java
package vn.gostock.auth;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class AuthServiceApplication {
    public static void main(String[] args) {
        SpringApplication.run(AuthServiceApplication.class, args);
    }
}
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/resources/ auth-service/src/main/java/vn/gostock/auth/AuthServiceApplication.java
git commit -m "feat(auth): add application.yml, Flyway V1 migration, main entry point"
```

---

## Task 4: JPA Entities

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/entity/User.java`
- Create: `auth-service/src/main/java/vn/gostock/auth/entity/MarketGroup.java`
- Create: `auth-service/src/main/java/vn/gostock/auth/entity/UserMarketGroup.java`
- Create: `auth-service/src/main/java/vn/gostock/auth/entity/UserMarketGroupId.java`

- [ ] **Tạo `User.java`** (tương thích với GORM soft-delete — filter `deleted_at IS NULL`):

```java
package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;
import java.time.LocalDateTime;

@Entity
@Table(name = "users")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class User {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, unique = true, length = 50)
    private String username;

    @Column(name = "password_hash", nullable = false)
    private String passwordHash;

    // valid: "super_admin", "admin", "user"
    @Column(nullable = false)
    private String role;

    @Column(name = "created_at")
    private LocalDateTime createdAt;

    @Column(name = "updated_at")
    private LocalDateTime updatedAt;

    // GORM soft-delete compatibility — always filter WHERE deleted_at IS NULL
    @Column(name = "deleted_at")
    private LocalDateTime deletedAt;
}
```

- [ ] **Tạo `MarketGroup.java`:**

```java
package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "market_groups")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class MarketGroup {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, unique = true, length = 100)
    private String name;

    @Column(columnDefinition = "TEXT")
    private String description;

    @Column(name = "created_at")
    private LocalDateTime createdAt;

    @Column(name = "updated_at")
    private LocalDateTime updatedAt;

    @ElementCollection(fetch = FetchType.EAGER)
    @CollectionTable(
        name = "market_group_markets",
        joinColumns = @JoinColumn(name = "group_id")
    )
    @Column(name = "market_key")
    @Builder.Default
    private List<String> marketKeys = new ArrayList<>();
}
```

- [ ] **Tạo `UserMarketGroupId.java`:**

```java
package vn.gostock.auth.entity;

import lombok.*;
import java.io.Serializable;

@Data
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode
public class UserMarketGroupId implements Serializable {
    private Long userId;
    private Long groupId;
}
```

- [ ] **Tạo `UserMarketGroup.java`:**

```java
package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;

@Entity
@Table(name = "user_market_groups")
@Data
@NoArgsConstructor
@AllArgsConstructor
@IdClass(UserMarketGroupId.class)
public class UserMarketGroup {
    @Id
    @Column(name = "user_id")
    private Long userId;

    @Id
    @Column(name = "group_id")
    private Long groupId;
}
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/entity/
git commit -m "feat(auth): add JPA entities — User, MarketGroup, UserMarketGroup"
```

---

## Task 5: Spring Data Repositories

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/repository/UserRepository.java`
- Create: `auth-service/src/main/java/vn/gostock/auth/repository/MarketGroupRepository.java`
- Create: `auth-service/src/main/java/vn/gostock/auth/repository/UserMarketGroupRepository.java`

- [ ] **Tạo `UserRepository.java`:**

```java
package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.User;
import java.util.List;
import java.util.Optional;

public interface UserRepository extends JpaRepository<User, Long> {
    Optional<User> findByUsernameAndDeletedAtIsNull(String username);
    Optional<User> findByIdAndDeletedAtIsNull(Long id);
    List<User> findAllByDeletedAtIsNull();
    boolean existsByUsernameAndDeletedAtIsNull(String username);
    boolean existsByUsernameAndRoleAndDeletedAtIsNull(String username, String role);
}
```

- [ ] **Tạo `MarketGroupRepository.java`:**

```java
package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.MarketGroup;
import java.util.Optional;

public interface MarketGroupRepository extends JpaRepository<MarketGroup, Long> {
    boolean existsByName(String name);
    Optional<MarketGroup> findByName(String name);
}
```

- [ ] **Tạo `UserMarketGroupRepository.java`:**

```java
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
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/repository/
git commit -m "feat(auth): add Spring Data repositories"
```

---

## Task 6: JwtConfig + JwtService

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/config/JwtConfig.java`
- Create: `auth-service/src/main/java/vn/gostock/auth/service/JwtService.java`

- [ ] **Tạo `JwtConfig.java`:**

```java
package vn.gostock.auth.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;

@Configuration
public class JwtConfig {
    @Bean
    public PasswordEncoder passwordEncoder() {
        return new BCryptPasswordEncoder();
    }
}
```

- [ ] **Tạo `JwtService.java`:**

```java
package vn.gostock.auth.service;

import com.auth0.jwt.JWT;
import com.auth0.jwt.algorithms.Algorithm;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import vn.gostock.auth.entity.User;
import java.util.Date;
import java.util.List;

@Service
public class JwtService {

    @Value("${jwt.secret}")
    private String secret;

    @Value("${jwt.expiration-ms:86400000}")
    private long expirationMs;

    public String generateToken(User user, List<String> accessibleMarkets) {
        Algorithm algorithm = Algorithm.HMAC256(secret);
        return JWT.create()
            .withSubject(String.valueOf(user.getId()))
            .withClaim("username", user.getUsername())
            .withClaim("role", user.getRole())
            .withClaim("user_id", user.getId())
            .withClaim("accessible_markets", accessibleMarkets)
            .withIssuedAt(new Date())
            .withExpiresAt(new Date(System.currentTimeMillis() + expirationMs))
            .sign(algorithm);
    }
}
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/config/ auth-service/src/main/java/vn/gostock/auth/service/JwtService.java
git commit -m "feat(auth): add JwtConfig (PasswordEncoder bean) and JwtService"
```

---

## Task 7: UserService

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/service/UserService.java`

- [ ] **Tạo `UserService.java`:**

```java
package vn.gostock.auth.service;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import lombok.RequiredArgsConstructor;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.repository.UserMarketGroupRepository;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;
import java.util.List;

@Service
@RequiredArgsConstructor
public class UserService {

    private static final List<String> ALL_MARKETS = List.of("GOLD", "NASDAQ", "CRYPTO", "SP500");
    private static final List<String> VALID_ROLES  = List.of("super_admin", "admin", "user");

    private final UserRepository userRepository;
    private final UserMarketGroupRepository userMarketGroupRepository;
    private final PasswordEncoder passwordEncoder;

    public User authenticate(String username, String password) {
        User user = userRepository.findByUsernameAndDeletedAtIsNull(username)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.UNAUTHENTICATED.withDescription("invalid credentials")));
        if (!passwordEncoder.matches(password, user.getPasswordHash())) {
            throw new StatusRuntimeException(
                Status.UNAUTHENTICATED.withDescription("invalid credentials"));
        }
        return user;
    }

    public List<String> getAccessibleMarkets(User user) {
        if ("super_admin".equals(user.getRole())) {
            return ALL_MARKETS;
        }
        List<String> markets = userMarketGroupRepository.findMarketKeysByUserId(user.getId());
        return markets.isEmpty() ? List.of() : markets;
    }

    public List<User> listUsers() {
        return userRepository.findAllByDeletedAtIsNull();
    }

    @Transactional
    public User createUser(String callerRole, String username, String password, String role) {
        requireAdminOrSuperAdmin(callerRole);
        String effectiveRole = VALID_ROLES.contains(role) ? role : "user";
        if ("super_admin".equals(effectiveRole) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("only super_admin can create super_admin"));
        }
        if (userRepository.existsByUsernameAndDeletedAtIsNull(username)) {
            throw new StatusRuntimeException(
                Status.ALREADY_EXISTS.withDescription("username already taken"));
        }
        User user = User.builder()
            .username(username)
            .passwordHash(passwordEncoder.encode(password))
            .role(effectiveRole)
            .createdAt(LocalDateTime.now())
            .updatedAt(LocalDateTime.now())
            .build();
        return userRepository.save(user);
    }

    @Transactional
    public void deleteUser(String callerRole, long targetId) {
        requireAdminOrSuperAdmin(callerRole);
        User target = userRepository.findByIdAndDeletedAtIsNull(targetId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(target.getRole()) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot delete super_admin"));
        }
        target.setDeletedAt(LocalDateTime.now());
        userRepository.save(target);
    }

    @Transactional
    public User updateRole(String callerRole, long targetId, String newRole) {
        requireAdminOrSuperAdmin(callerRole);
        if (!VALID_ROLES.contains(newRole)) {
            throw new StatusRuntimeException(
                Status.INVALID_ARGUMENT.withDescription("invalid role: " + newRole));
        }
        User target = userRepository.findByIdAndDeletedAtIsNull(targetId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if ("super_admin".equals(target.getRole()) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("cannot modify super_admin"));
        }
        if ("super_admin".equals(newRole) && !"super_admin".equals(callerRole)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("only super_admin can assign super_admin role"));
        }
        target.setRole(newRole);
        target.setUpdatedAt(LocalDateTime.now());
        return userRepository.save(target);
    }

    @Transactional
    public void changePassword(long callerId, String oldPassword, String newPassword) {
        User user = userRepository.findByIdAndDeletedAtIsNull(callerId)
            .orElseThrow(() -> new StatusRuntimeException(
                Status.NOT_FOUND.withDescription("user not found")));
        if (!passwordEncoder.matches(oldPassword, user.getPasswordHash())) {
            throw new StatusRuntimeException(
                Status.UNAUTHENTICATED.withDescription("current password is incorrect"));
        }
        if (newPassword.length() < 12) {
            throw new StatusRuntimeException(
                Status.INVALID_ARGUMENT.withDescription("new password must be at least 12 characters"));
        }
        user.setPasswordHash(passwordEncoder.encode(newPassword));
        user.setUpdatedAt(LocalDateTime.now());
        userRepository.save(user);
    }

    private void requireAdminOrSuperAdmin(String role) {
        if (!"admin".equals(role) && !"super_admin".equals(role)) {
            throw new StatusRuntimeException(
                Status.PERMISSION_DENIED.withDescription("admin access required"));
        }
    }
}
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/service/UserService.java
git commit -m "feat(auth): add UserService with auth, CRUD, password change logic"
```

---

## Task 8: MarketGroupService

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/service/MarketGroupService.java`

- [ ] **Tạo `MarketGroupService.java`:**

```java
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
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/service/MarketGroupService.java
git commit -m "feat(auth): add MarketGroupService — CRUD groups, assign markets and users"
```

---

## Task 9: SuperAdminSeeder

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/seeder/SuperAdminSeeder.java`

- [ ] **Tạo `SuperAdminSeeder.java`:**

```java
package vn.gostock.auth.seeder;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Component;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.repository.UserRepository;
import java.time.LocalDateTime;

@Component
@RequiredArgsConstructor
@Slf4j
public class SuperAdminSeeder implements ApplicationRunner {

    private static final String SUPER_ADMIN_USERNAME = "chon";
    private static final String SUPER_ADMIN_PASSWORD = "Ch1nch2n@";

    private final UserRepository userRepository;
    private final PasswordEncoder passwordEncoder;

    @Override
    public void run(ApplicationArguments args) {
        boolean exists = userRepository.existsByUsernameAndRoleAndDeletedAtIsNull(
            SUPER_ADMIN_USERNAME, "super_admin");
        if (!exists) {
            User superAdmin = User.builder()
                .username(SUPER_ADMIN_USERNAME)
                .passwordHash(passwordEncoder.encode(SUPER_ADMIN_PASSWORD))
                .role("super_admin")
                .createdAt(LocalDateTime.now())
                .updatedAt(LocalDateTime.now())
                .build();
            userRepository.save(superAdmin);
            log.info("Super admin '{}' seeded successfully", SUPER_ADMIN_USERNAME);
        }
    }
}
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/seeder/SuperAdminSeeder.java
git commit -m "feat(auth): add SuperAdminSeeder — ensures chon/super_admin exists on startup"
```

---

## Task 10: AuthGrpcServiceImpl

**Files:**
- Create: `auth-service/src/main/java/vn/gostock/auth/grpc/AuthGrpcServiceImpl.java`

- [ ] **Tạo `AuthGrpcServiceImpl.java`:**

```java
package vn.gostock.auth.grpc;

import io.grpc.stub.StreamObserver;
import lombok.RequiredArgsConstructor;
import net.devh.boot.grpc.server.service.GrpcService;
import vn.gostock.auth.entity.MarketGroup;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.proto.*;
import vn.gostock.auth.service.JwtService;
import vn.gostock.auth.service.MarketGroupService;
import vn.gostock.auth.service.UserService;
import java.util.List;

@GrpcService
@RequiredArgsConstructor
public class AuthGrpcServiceImpl extends AuthServiceGrpc.AuthServiceImplBase {

    private final UserService userService;
    private final MarketGroupService marketGroupService;
    private final JwtService jwtService;

    // ── Auth ──────────────────────────────────────────────────────────────

    @Override
    public void login(LoginRequest req, StreamObserver<LoginResponse> obs) {
        try {
            User user = userService.authenticate(req.getUsername(), req.getPassword());
            List<String> markets = userService.getAccessibleMarkets(user);
            String token = jwtService.generateToken(user, markets);
            obs.onNext(LoginResponse.newBuilder()
                .setToken(token).setUsername(user.getUsername()).setRole(user.getRole())
                .build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void getMe(CallerMeta req, StreamObserver<UserResponse> obs) {
        try {
            // caller_id comes from Go's JWT claims — return that user's info
            User user = userService.listUsers().stream()
                .filter(u -> u.getId().equals(req.getCallerId()))
                .findFirst()
                .orElseThrow();
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void changePassword(ChangePassRequest req, StreamObserver<Empty> obs) {
        try {
            userService.changePassword(req.getCaller().getCallerId(),
                req.getOldPassword(), req.getNewPassword());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── User management ───────────────────────────────────────────────────

    @Override
    public void listUsers(CallerMeta req, StreamObserver<ListUsersResponse> obs) {
        try {
            List<UserResponse> users = userService.listUsers().stream()
                .map(this::toUserResponse).toList();
            obs.onNext(ListUsersResponse.newBuilder().addAllUsers(users).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void createUser(CreateUserRequest req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.createUser(req.getCaller().getCallerRole(),
                req.getUsername(), req.getPassword(), req.getRole());
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void deleteUser(DeleteUserRequest req, StreamObserver<Empty> obs) {
        try {
            userService.deleteUser(req.getCaller().getCallerRole(), req.getTargetId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateUserRole(UpdateRoleRequest req, StreamObserver<UserResponse> obs) {
        try {
            User user = userService.updateRole(req.getCaller().getCallerRole(),
                req.getTargetId(), req.getNewRole());
            obs.onNext(toUserResponse(user));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Market groups ─────────────────────────────────────────────────────

    @Override
    public void listMarketGroups(CallerMeta req, StreamObserver<ListGroupsResponse> obs) {
        try {
            List<MarketGroupResponse> groups = marketGroupService.listGroups().stream()
                .map(this::toGroupResponse).toList();
            obs.onNext(ListGroupsResponse.newBuilder().addAllGroups(groups).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void createMarketGroup(CreateGroupRequest req, StreamObserver<MarketGroupResponse> obs) {
        try {
            MarketGroup g = marketGroupService.createGroup(
                req.getCaller().getCallerRole(), req.getName(), req.getDescription());
            obs.onNext(toGroupResponse(g));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void updateMarketGroup(UpdateGroupRequest req, StreamObserver<MarketGroupResponse> obs) {
        try {
            MarketGroup g = marketGroupService.updateGroup(
                req.getCaller().getCallerRole(), req.getGroupId(),
                req.getName(), req.getDescription());
            obs.onNext(toGroupResponse(g));
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void deleteMarketGroup(DeleteGroupRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.deleteGroup(req.getCaller().getCallerRole(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void setGroupMarkets(SetGroupMarketsRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.setGroupMarkets(req.getCaller().getCallerRole(),
                req.getGroupId(), req.getMarketKeysList());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void listGroupUsers(GroupRequest req, StreamObserver<ListUsersResponse> obs) {
        try {
            List<UserResponse> users = marketGroupService
                .listGroupUsers(req.getCaller().getCallerRole(), req.getGroupId())
                .stream().map(this::toUserResponse).toList();
            obs.onNext(ListUsersResponse.newBuilder().addAllUsers(users).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void addUserToGroup(UserGroupRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.addUserToGroup(req.getCaller().getCallerRole(),
                req.getUserId(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void removeUserFromGroup(UserGroupRequest req, StreamObserver<Empty> obs) {
        try {
            marketGroupService.removeUserFromGroup(req.getCaller().getCallerRole(),
                req.getUserId(), req.getGroupId());
            obs.onNext(Empty.getDefaultInstance());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    @Override
    public void getUserMarketGroups(UserRequest req, StreamObserver<ListGroupsResponse> obs) {
        try {
            List<MarketGroupResponse> groups = marketGroupService
                .getUserMarketGroups(req.getUserId())
                .stream().map(this::toGroupResponse).toList();
            obs.onNext(ListGroupsResponse.newBuilder().addAllGroups(groups).build());
            obs.onCompleted();
        } catch (Exception e) { obs.onError(e); }
    }

    // ── Converters ────────────────────────────────────────────────────────

    private UserResponse toUserResponse(User u) {
        return UserResponse.newBuilder()
            .setId(u.getId())
            .setUsername(u.getUsername())
            .setRole(u.getRole())
            .setCreatedAt(u.getCreatedAt() != null ? u.getCreatedAt().toString() : "")
            .build();
    }

    private MarketGroupResponse toGroupResponse(MarketGroup g) {
        return MarketGroupResponse.newBuilder()
            .setId(g.getId())
            .setName(g.getName())
            .setDescription(g.getDescription() != null ? g.getDescription() : "")
            .addAllMarketKeys(g.getMarketKeys())
            .setCreatedAt(g.getCreatedAt() != null ? g.getCreatedAt().toString() : "")
            .setUpdatedAt(g.getUpdatedAt() != null ? g.getUpdatedAt().toString() : "")
            .build();
    }
}
```

- [ ] **Commit:**
```bash
git add auth-service/src/main/java/vn/gostock/auth/grpc/AuthGrpcServiceImpl.java
git commit -m "feat(auth): implement AuthGrpcServiceImpl — all 15 RPCs"
```

---

## Task 11: Unit Tests

**Files:**
- Create: `auth-service/src/test/java/vn/gostock/auth/grpc/AuthGrpcServiceImplTest.java`

- [ ] **Viết failing tests trước:**

```java
package vn.gostock.auth.grpc;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.*;
import org.mockito.junit.jupiter.MockitoExtension;
import vn.gostock.auth.entity.User;
import vn.gostock.auth.proto.*;
import vn.gostock.auth.service.JwtService;
import vn.gostock.auth.service.MarketGroupService;
import vn.gostock.auth.service.UserService;
import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class AuthGrpcServiceImplTest {

    @Mock UserService userService;
    @Mock MarketGroupService marketGroupService;
    @Mock JwtService jwtService;

    @InjectMocks AuthGrpcServiceImpl grpcService;

    private User mockUser;

    @BeforeEach
    void setUp() {
        mockUser = User.builder()
            .id(1L).username("testuser").role("user")
            .build();
    }

    @Test
    void login_validCredentials_returnsToken() {
        when(userService.authenticate("testuser", "pass123")).thenReturn(mockUser);
        when(userService.getAccessibleMarkets(mockUser)).thenReturn(List.of("GOLD"));
        when(jwtService.generateToken(mockUser, List.of("GOLD"))).thenReturn("jwt-token");

        StreamObserver<LoginResponse> obs = mock(StreamObserver.class);
        grpcService.login(LoginRequest.newBuilder()
            .setUsername("testuser").setPassword("pass123").build(), obs);

        ArgumentCaptor<LoginResponse> captor = ArgumentCaptor.forClass(LoginResponse.class);
        verify(obs).onNext(captor.capture());
        verify(obs).onCompleted();
        assert captor.getValue().getToken().equals("jwt-token");
    }

    @Test
    void login_invalidCredentials_propagatesUnauthenticated() {
        when(userService.authenticate(anyString(), anyString()))
            .thenThrow(new StatusRuntimeException(Status.UNAUTHENTICATED));

        StreamObserver<LoginResponse> obs = mock(StreamObserver.class);
        grpcService.login(LoginRequest.newBuilder()
            .setUsername("bad").setPassword("bad").build(), obs);

        verify(obs).onError(any(StatusRuntimeException.class));
        verify(obs, never()).onCompleted();
    }

    @Test
    void createUser_adminCaller_createsUser() {
        when(userService.createUser("admin", "newuser", "pass", "user")).thenReturn(
            User.builder().id(2L).username("newuser").role("user").build());

        StreamObserver<UserResponse> obs = mock(StreamObserver.class);
        grpcService.createUser(CreateUserRequest.newBuilder()
            .setCaller(CallerMeta.newBuilder().setCallerId(1L).setCallerRole("admin").build())
            .setUsername("newuser").setPassword("pass").setRole("user").build(), obs);

        ArgumentCaptor<UserResponse> captor = ArgumentCaptor.forClass(UserResponse.class);
        verify(obs).onNext(captor.capture());
        assert captor.getValue().getUsername().equals("newuser");
    }

    @Test
    void deleteUser_adminCannotDeleteSuperAdmin_propagatesPermissionDenied() {
        doThrow(new StatusRuntimeException(Status.PERMISSION_DENIED))
            .when(userService).deleteUser("admin", 99L);

        StreamObserver<Empty> obs = mock(StreamObserver.class);
        grpcService.deleteUser(DeleteUserRequest.newBuilder()
            .setCaller(CallerMeta.newBuilder().setCallerRole("admin").build())
            .setTargetId(99L).build(), obs);

        verify(obs).onError(any(StatusRuntimeException.class));
    }
}
```

- [ ] **Chạy tests (expect fail vì chưa compile proto):**
```bash
cd auth-service && mvn test 2>&1 | tail -20
```

- [ ] **Build full (generate proto stubs + compile + test):**
```bash
cd auth-service && mvn package -B 2>&1 | tail -30
```
Expected: `BUILD SUCCESS`, tests pass.

- [ ] **Commit:**
```bash
git add auth-service/src/test/
git commit -m "test(auth): add unit tests for AuthGrpcServiceImpl"
```

---

## Task 12: Build Docker image + smoke test

- [ ] **Build Docker image:**
```bash
cd /home/chronical/Projects/private/go-stock-prediction
docker build -t go-stock-prediction-auth:latest auth-service/
```
Expected: `Successfully tagged go-stock-prediction-auth:latest`

- [ ] **Smoke test: chạy container local với MySQL đang chạy:**
```bash
docker run --rm --network go-stock-prediction_default \
  -e DB_HOST=db -e DB_PORT=3306 \
  -e DB_NAME=go_stock_prediction \
  -e DB_USER=root -e DB_PASSWORD=123 \
  -e JWT_SECRET=test-secret \
  -e GRPC_PORT=8120 \
  --name auth-smoke \
  -d go-stock-prediction-auth:latest

sleep 5
docker logs auth-smoke 2>&1 | grep -E "(Started|ERROR|Super admin)"
```
Expected log: `Started AuthServiceApplication` và `Super admin 'chon' seeded successfully` (hoặc "already exists").

- [ ] **Dừng container smoke test:**
```bash
docker stop auth-smoke
```

- [ ] **Commit final:**
```bash
git add auth-service/
git commit -m "feat(auth): Java Auth Service complete — gRPC server, entities, Flyway, seeder"
```
