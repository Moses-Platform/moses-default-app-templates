# Single-stage builder running npm ci then the Vite/TS build,
# then a slim nginx runtime.
#
# Standard two-step build (Kaniko hardlink workaround removed
# post-CHAT-h0v8t.13). The build engine is now Buildah, which does not
# share Kaniko's tar-hardlink corruption bug, so `npm ci` and the
# build can live in separate RUN layers as you'd expect — restoring
# layer-level caching of dependencies across source-only changes.

# --- Builder ------------------------------------------------------------
FROM node:26.13.1-alpine3.24 AS build
WORKDIR /app

# CHAT-s6qf3: optional Moses in-cluster package cache (Nexus). When the
# platform injects NPM_REGISTRY=http://moses-package-cache:8081/repository/npm-proxy/
# we point npm at it for the install; otherwise we use the upstream default.
ARG NPM_REGISTRY=""

# Copy the dependency manifests FIRST so the npm ci layer is cached
# across source-only changes (the layer only busts when package*.json or
# NPM_REGISTRY change).
COPY package*.json ./

# Install deps (registry-aware) in its own layer.
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi \
    && npm ci

# Now copy the source.
COPY . .

# Moses browser-logger build arg (optional; empty = same origin). ARG scope is bounded to this stage; declared after
# the install layer so changing them never busts the dependency cache.
ARG VITE_MOSES_API_BASE=""

# Build with the browser-logger arg in scope.
RUN VITE_MOSES_API_BASE="$VITE_MOSES_API_BASE" \
    sh -c 'npx tsc -p tsconfig.build.json && npx vite build'

# --- Runtime ------------------------------------------------------------
FROM nginx:1.30.7-alpine3.24

# CHAT-pbup: nginx config is rendered at container start by entrypoint.sh
# from MOSES_EMBEDDING_FRAMING + MOSES_EMBEDDING_ALLOWED_ANCESTORS so a
# single image works at multiple BASE_PATHs without rebuild. envsubst
# ships in nginx:alpine via gettext.
RUN apk add --no-cache gettext

# Copy built assets
COPY --from=build /app/dist /usr/share/nginx/html/

# Copy nginx config TEMPLATE — entrypoint.sh runs envsubst over it.
COPY nginx.conf /etc/nginx/nginx.conf.template

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh \
    && test "$(id -u nginx)" = 101 && test "$(id -g nginx)" = 101

USER 101:101

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Start nginx via entrypoint (renders nginx.conf + index.html, then exec nginx)
ENTRYPOINT ["/entrypoint.sh"]
