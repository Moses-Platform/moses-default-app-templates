# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files (glob makes package-lock.json optional for self-contained builds)
COPY package.json package-lock.json* ./
RUN npm install

# Copy source code
COPY . .

# Build the application
RUN npx tsc -p tsconfig.build.json && npx vite build

# Runtime stage
FROM nginx:alpine

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

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
