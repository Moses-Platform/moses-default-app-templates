# Single-stage builder running npm install then the Vite/TS build,
# then a slim nginx runtime.
#
# Standard two-step build (Kaniko hardlink workaround removed
# post-CHAT-h0v8t.13). The build engine is now Buildah, which does not
# share Kaniko's tar-hardlink corruption bug, so `npm install` and the
# build can live in separate RUN layers as you'd expect — restoring
# layer-level caching of dependencies across source-only changes.

# --- Builder ------------------------------------------------------------
FROM node:24-alpine AS build
WORKDIR /app

COPY . .

# Moses BLF-B build args. ARG scope is bounded to this stage.
ARG VITE_MOSES_CHART_ID=""
ARG VITE_MOSES_DEPLOYMENT_ID=""
ARG VITE_MOSES_API_BASE=""

# CHAT-s6qf3: optional Moses in-cluster package cache (Nexus). When the
# platform injects NPM_REGISTRY=http://moses-package-cache:8081/repository/npm-proxy/
# we point npm at it for the install; otherwise we use the upstream default.
ARG NPM_REGISTRY=""

# Install deps (registry-aware) in its own layer.
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi \
    && npm install

# Build with BLF-B vars in scope.
RUN VITE_MOSES_CHART_ID="$VITE_MOSES_CHART_ID" \
    VITE_MOSES_DEPLOYMENT_ID="$VITE_MOSES_DEPLOYMENT_ID" \
    VITE_MOSES_API_BASE="$VITE_MOSES_API_BASE" \
    sh -c 'npx tsc -p tsconfig.build.json && npx vite build'

# --- Runtime ------------------------------------------------------------
FROM nginx:1.28-alpine

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
RUN chmod +x /entrypoint.sh

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Start nginx via entrypoint (renders nginx.conf + index.html, then exec nginx)
ENTRYPOINT ["/entrypoint.sh"]
