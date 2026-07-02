# Single-stage builder running npm install then the Vite/TS build,
# then a slim nginx runtime.
#
# Standard two-step build (Kaniko hardlink workaround removed
# post-CHAT-h0v8t.13). The build engine is now Buildah, which handles
# npm-deduped hardlinks correctly, so install and build can be in
# separate RUN layers for proper layer caching on source-only changes.

# --- Builder ------------------------------------------------------------
FROM node:24-alpine AS builder
WORKDIR /app

# Moses BLF-B build args. ARG scope is bounded to this stage.
ARG VITE_MOSES_CHART_ID=""
ARG VITE_MOSES_DEPLOYMENT_ID=""
ARG VITE_MOSES_API_BASE=""

# CHAT-s6qf3: optional Moses in-cluster package cache (Nexus). Empty = upstream default.
ARG NPM_REGISTRY=""

# Manifests first, then install: the deps layer is only invalidated by
# package*.json changes, so source-only edits reuse the cached install.
COPY package*.json ./
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi \
    && npm install

COPY . .

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
# ships in nginx:alpine via gettext — pin the package explicitly so an
# upstream image change cannot silently break entrypoint.sh.
RUN apk add --no-cache gettext

# Copy nginx config template
COPY nginx.conf /etc/nginx/nginx.conf.template

# Copy built application
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# BACKEND_SERVICE_HOST / BACKEND_SERVICE_PORT are injected per-deploy by the
# Helm chart. entrypoint.sh has shell-default fallbacks so standalone runs work.
# We deliberately do NOT set ENV defaults here because they would pollute
# non-K8s environments with Moses-specific DNS names that don't resolve outside
# the cluster.

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
