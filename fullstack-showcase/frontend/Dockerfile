# Multi-stage build with Moses BLF-B late-stage ARG pattern (CHAT-kdep).
#
# The `deps` stage installs npm packages — it must be insensitive to
# per-build values like chart_id / deployment_id, otherwise every Moses
# deploy would re-download the entire dependency graph. Moving the ARG
# declarations into the `build` stage scopes their cache-busting effect to
# the final Vite invocation only.

# --- Dependencies (cacheable across builds) -----------------------------
FROM node:20-alpine AS deps
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

# --- Build (re-runs per build, but `deps` layer stays cached) ------------
FROM deps AS builder
WORKDIR /app

COPY . .

# Moses BLF-B build args. Declare ONLY in this stage to preserve the
# `deps` layer cache.
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

# Set default environment variables
ENV BACKEND_SERVICE_HOST=agent-deployed-app-backend
ENV BACKEND_SERVICE_PORT=8080

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
