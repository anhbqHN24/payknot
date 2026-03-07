FROM node:20-alpine AS base
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

FROM base AS deps
RUN apk add --no-cache python3 make g++ eudev-dev pkgconf
COPY package*.json ./
RUN npm ci --no-audit --no-fund

FROM base AS builder
COPY --from=deps /app/node_modules ./node_modules
COPY . .

ARG NEXT_PUBLIC_SOLANA_RPC_URL
ARG NEXT_PUBLIC_USDC_MINT
ARG NEXT_PUBLIC_MERCHANT_WALLET
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID
ARG API_PROXY_TARGET

ENV NEXT_PUBLIC_SOLANA_RPC_URL=${NEXT_PUBLIC_SOLANA_RPC_URL}
ENV NEXT_PUBLIC_USDC_MINT=${NEXT_PUBLIC_USDC_MINT}
ENV NEXT_PUBLIC_MERCHANT_WALLET=${NEXT_PUBLIC_MERCHANT_WALLET}
ENV NEXT_PUBLIC_GOOGLE_CLIENT_ID=${NEXT_PUBLIC_GOOGLE_CLIENT_ID}
ENV API_PROXY_TARGET=${API_PROXY_TARGET}

RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup -S nextjs && adduser -S nextjs -G nextjs

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]
