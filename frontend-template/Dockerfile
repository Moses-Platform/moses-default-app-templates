# Build stage
FROM node:20-alpine AS build

WORKDIR /app

# Copy package files (glob makes package-lock.json optional for self-contained builds)
COPY package.json package-lock.json* ./

# Install dependencies (npm install works with or without lock file)
RUN npm install

# Copy source files
COPY . .

# Build application
RUN npx tsc -p tsconfig.build.json && npx vite build

# Runtime stage
FROM nginx:alpine

# Copy built assets
COPY --from=build /app/dist /usr/share/nginx/html/

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
