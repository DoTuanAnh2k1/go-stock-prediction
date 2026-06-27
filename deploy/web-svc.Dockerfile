# Stage 1: build React app
FROM node:22-alpine AS builder
WORKDIR /app

# Build-time version stamps — passed via --build-arg; default to "unknown"
ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown

# Expose to Vite so import.meta.env.VITE_* picks them up at bundle time
ENV VITE_GIT_SHA=$GIT_SHA
ENV VITE_BUILD_TIME=$BUILD_TIME
ENV VITE_GIT_DIRTY=$GIT_DIRTY

COPY package*.json ./
RUN npm install
COPY . .
# Echo the version args so a changed SHA/BUILD_TIME busts the npm build cache —
# an ENV-only change doesn't invalidate a RUN, which would bake a stale footer.
RUN echo "version $GIT_SHA $BUILD_TIME $GIT_DIRTY" && npm run build

# Write static /version.json into the dist root for nginx to serve directly
RUN printf '{"service":"web-svc","git_sha":"%s","build_time":"%s","dirty":"%s"}\n' \
    "$GIT_SHA" "$BUILD_TIME" "$GIT_DIRTY" > /app/dist/version.json

# Stage 2: serve static files only (gateway handles TLS + API routing)
FROM nginx:alpine

# Carry the same stamps into the runtime image so docker-entrypoint.d scripts
# can write /versions/web-svc.json on each container start
ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown
# ENV set after a RUN that consumes the args (below) so a changed SHA/BUILD_TIME
# busts the ENV cache; the docker-entrypoint.d script reads it at runtime.

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Startup script — writes /versions/web-svc.json before nginx starts
COPY docker-entrypoint.d/40-write-version.sh /docker-entrypoint.d/40-write-version.sh
RUN chmod +x /docker-entrypoint.d/40-write-version.sh
# Shared version-stamp dir 0777 (volume inits world-writable for non-root UIDs).
# The printf consumes the version args so a changed SHA/BUILD_TIME busts the ENV below.
RUN mkdir -p /versions && chmod 0777 /versions && \
    printf 'git_sha=%s build_time=%s dirty=%s\n' "$GIT_SHA" "$BUILD_TIME" "$GIT_DIRTY" > /etc/image-version
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY

EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
