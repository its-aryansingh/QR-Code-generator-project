FROM node:22-bookworm-slim AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS builder
WORKDIR /app
COPY pnpm-lock.yaml* package.json pnpm-workspace.yaml turbo.json ./
COPY packages/ packages/
COPY apps/render/ apps/render/

RUN pnpm install --frozen-lockfile=false
RUN pnpm --filter @qrit/qr-render build
RUN pnpm --filter @qrit/render build

FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3001
ENV HOST=0.0.0.0

COPY --from=builder /app /app

EXPOSE 3001
CMD ["node", "apps/render/dist/server.js"]
