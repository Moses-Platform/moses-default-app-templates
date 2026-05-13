# Single-stage builder doing npm install + Vite/TS build in ONE RUN, then
# a slim nginx runtime.
#
# Why everything in one RUN:
# Kaniko has a long-standing bug (kaniko#1743, upstream archived UNFIXED
# 2025-06-03) where its tar serialization writes npm-deduped hardlinks
# with an absolute-path target. This bug bites at EVERY tar round-trip:
#   1. registry cache push/pull (addressed by CHAT-1mp04: --cache=false)
#   2. inter-stage tar handoff (addressed by collapsing to one stage)
#   3. per-RUN intermediate snapshot — even within a single stage, with
#      cache disabled, Kaniko tars the layer after each RUN/COPY and
#      re-applies it before the next step, which re-creates the
#      corrupted 0-byte launcher shim under node_modules/.bin/tsc and
#      node_modules/esbuild/bin/esbuild, producing EACCES / ENOEXEC.
#
# Putting `npm install` and the Vite build in ONE RUN means no Kaniko
# snapshot happens between node_modules being created and tsc/vite
# being invoked. The launcher shims stay intact for the duration of
# the build. After the build completes, Kaniko's final snapshot may
# still corrupt node_modules but by then /app/dist is produced and
# the runtime stage only consumes that.
#
# Trade-off: --no-cache friendly (matches CHAT-1mp04 cache=false), but
# any source change re-runs npm install too (no layer-level reuse).
# Acceptable as the steady state until either Kaniko-fork bump or
# BuildKit migration eliminates the underlying bug.

# --- Builder (install + Vite/TS build, all in ONE RUN) ------------------
FROM node:20-alpine AS builder
WORKDIR /app

COPY . .

# Moses BLF-B build args. Declared HERE (not in a prior stage) so they
# don't influence npm install via env-var leakage — the ARG values are
# only resolved when the single RUN below executes them as shell vars.
ARG VITE_MOSES_CHART_ID=""
ARG VITE_MOSES_DEPLOYMENT_ID=""
ARG VITE_MOSES_API_BASE=""

RUN VITE_MOSES_CHART_ID="$VITE_MOSES_CHART_ID" \
    VITE_MOSES_DEPLOYMENT_ID="$VITE_MOSES_DEPLOYMENT_ID" \
    VITE_MOSES_API_BASE="$VITE_MOSES_API_BASE" \
    sh -c 'npm install && npx tsc -p tsconfig.build.json && npx vite build'

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
