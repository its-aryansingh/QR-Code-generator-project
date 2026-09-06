# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source files
COPY . .

# Build arguments for client-side baked env variables
ARG NEXT_PUBLIC_API_URL=http://localhost:8084/api/v1
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8084
ARG NEXT_PUBLIC_APP_URL=http://localhost:3000
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID=""

ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL \
    NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL \
    NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL \
    NEXT_PUBLIC_GOOGLE_CLIENT_ID=$NEXT_PUBLIC_GOOGLE_CLIENT_ID \
    NEXT_TELEMETRY_DISABLED=1

# Build the application
RUN npm run build

# Production stage
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME="0.0.0.0"

# Create non-root system user
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# Copy package manifest
COPY --from=builder /app/package.json ./package.json

# Copy necessary files from builder with correct ownership
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

# Create wrapper so 'next' or 'next start' command routes directly to standalone server.js
RUN printf '#!/bin/sh\nexec node /app/server.js "$@"\n' > /usr/local/bin/next && \
    chmod +x /usr/local/bin/next

# Entrypoint to resolve monorepo nested server.js if needed
RUN printf '#!/bin/sh\nset -e\nif [ ! -f /app/server.js ]; then\n  FOUND=$(find /app -name server.js -maxdepth 3 | head -n 1)\n  if [ -n "$FOUND" ]; then cp "$FOUND" /app/server.js; fi\nfi\nexec "$@"\n' > /usr/local/bin/entrypoint.sh && \
    chmod +x /usr/local/bin/entrypoint.sh

USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["node", "server.js"]
