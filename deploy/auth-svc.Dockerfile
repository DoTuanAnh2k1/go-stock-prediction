# Stage 1: Build
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests -B

# Stage 2: Runtime
FROM eclipse-temurin:21-jre
WORKDIR /app
ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY
# Shared version-stamp dir — 0777 so the named volume initializes world-writable
# (services run under different non-root UIDs; each writes /versions/<svc>.json).
RUN mkdir -p /versions && chmod 0777 /versions
COPY --from=builder /app/target/auth-service-*.jar app.jar
ENTRYPOINT ["java", "-jar", "app.jar"]
