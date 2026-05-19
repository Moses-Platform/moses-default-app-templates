# Single-stage builder doing npm install + Vite/TS build in ONE RUN,
# then a slim nginx runtime.
#
# CHAT-1mp04: Kaniko has a long-standing tar-serialization bug
# (kaniko#1743, upstream archived UNFIXED) where npm-deduped hardlinks
# are written with absolute-path targets and corrupt to 0-byte files
# on tar restore. This bites at every Kaniko tar round-trip: registry
# cache push/pull (mitigated by --cache=false on the platform), inter-
# stage handoff (mitigated by single stage), AND per-RUN intra-stage
# snapshot (mitigated by combining npm install + tsc + vite into one
# RUN, so no Kaniko snapshot happens between node_modules creation
# and use).
#
# Trade-off: no layer-level reuse of npm install — any source change
# triggers a full reinstall. Matches the --cache=false steady state.

# --- Builder (install + Vite/TS build, all in ONE RUN) ------------------
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

RUN NPM_REGISTRY="$NPM_REGISTRY" \
    VITE_MOSES_CHART_ID="$VITE_MOSES_CHART_ID" \
    VITE_MOSES_DEPLOYMENT_ID="$VITE_MOSES_DEPLOYMENT_ID" \
    VITE_MOSES_API_BASE="$VITE_MOSES_API_BASE" \
    sh -c 'if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi && npm install && npx tsc -p tsconfig.build.json && npx vite build'

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
