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
FROM node:20-alpine AS builder
WORKDIR /app

COPY . .

# Moses BLF-B build args. ARG scope is bounded to this stage.
ARG VITE_MOSES_CHART_ID=""
ARG VITE_MOSES_DEPLOYMENT_ID=""
ARG VITE_MOSES_API_BASE=""

# CHAT-s6qf3: optional Moses in-cluster package cache (Nexus). Empty = upstream default.
ARG NPM_REGISTRY=""

RUN NPM_REGISTRY="$NPM_REGISTRY" \
    VITE_MOSES_CHART_ID="$VITE_MOSES_CHART_ID" \
    VITE_MOSES_DEPLOYMENT_ID="$VITE_MOSES_DEPLOYMENT_ID" \
    VITE_MOSES_API_BASE="$VITE_MOSES_API_BASE" \
    sh -c 'if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi && npm install && npx tsc -p tsconfig.build.json && npx vite build'

# --- Runtime ------------------------------------------------------------
FROM nginx:alpine

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
# Helm chart (see helm/templates/deployment.yaml). entrypoint.sh has shell-default
# fallbacks (`: "${BACKEND_SERVICE_HOST:=...}"`) so standalone `docker run` works.
# We deliberately do NOT set ENV defaults here because they would pollute
# non-K8s environments with Moses-specific DNS names that won't resolve outside
# the cluster — better to fail loudly via the entrypoint default than to point
# at a hostname the runner can't reach.

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
