# Single-stage builder (npm install + Vite/TS build) + slim nginx runtime.
#
# Why ONE builder stage (not deps + builder split):
# Kaniko has a long-standing bug (kaniko#1743, upstream archived UNFIXED
# 2025-06-03) where its tar serialization writes npm-deduped hardlinks
# with an absolute-path target. Both the inter-stage tar handoff (saving
# stage output to /kaniko/stages/<n> and restoring it as the base for the
# next stage) AND the registry cache push/pull paths apply this broken
# serialization. CHAT-1mp04 disabled the registry cache, but the
# inter-stage handoff path bites the same way: a deps stage followed by
# `FROM deps AS builder` re-extracts node_modules from a tar, and the
# launcher shim at node_modules/esbuild/bin/esbuild lands as a 0-byte
# file, producing EACCES / ENOEXEC at `npx tsc` / `npx vite`.
#
# Collapsing deps + build into one stage avoids the node_modules tar
# round-trip entirely. The runtime stage's COPY --from=builder only
# pulls /app/dist (built static assets — no node_modules, no hardlinks),
# which is safe.
#
# Trade-off (small): `npm install` and the Vite build live in one layer,
# so an isolated `RUN` change can't reuse a frozen npm-install layer.
# With --cache=false this is already the steady state — no functional
# change.

# --- Builder (install + Vite/TS build) ----------------------------------
FROM node:20-alpine AS builder
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

COPY . .

# Moses BLF-B build args. ARG scope is bounded to this stage by virtue
# of being the only stage in the chain that consumes user code.
ARG VITE_MOSES_CHART_ID=""
ARG VITE_MOSES_DEPLOYMENT_ID=""
ARG VITE_MOSES_API_BASE=""

RUN VITE_MOSES_CHART_ID="$VITE_MOSES_CHART_ID" \
    VITE_MOSES_DEPLOYMENT_ID="$VITE_MOSES_DEPLOYMENT_ID" \
    VITE_MOSES_API_BASE="$VITE_MOSES_API_BASE" \
    npx tsc -p tsconfig.build.json && npx vite build

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
# Helm chart. entrypoint.sh has shell-default fallbacks so standalone runs work.
# We deliberately do NOT set ENV defaults here because they would pollute
# non-K8s environments with Moses-specific DNS names that don't resolve outside
# the cluster.

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
