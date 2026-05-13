# Multi-stage build with Moses BLF-B late-stage ARG pattern (CHAT-kdep).
#
# The `deps` stage installs npm packages and the `builder` stage runs the
# actual Vite/TS build with per-deploy ARGs. Declaring per-deploy ARGs only
# in the `builder` stage scopes their visibility and keeps Dockerfile intent
# explicit.
#
# Note (CHAT-1mp04): the platform runs Kaniko with --cache=false, so the
# `deps` stage layer is NOT actually cached across builds. `npm install`
# re-runs on every Moses deploy. The multi-stage split remains useful for
# organizational clarity and ARG scope, but is no longer a performance
# optimisation. The intermediate-layer cache was disabled to fix an
# upstream Kaniko bug with npm hardlink corruption (kaniko#1743).

# --- Dependencies -------------------------------------------------------
FROM node:20-alpine AS deps
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

# --- Build --------------------------------------------------------------
FROM deps AS builder
WORKDIR /app

COPY . .

# Moses BLF-B build args. Declared ONLY in this stage so they don't leak
# into the deps stage's ARG scope.
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
