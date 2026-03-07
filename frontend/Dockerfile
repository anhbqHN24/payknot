FROM node:20-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci

FROM node:20-alpine AS builder
WORKDIR /app
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

COPY --from=builder /app/package*.json ./
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/next.config.js ./next.config.js

EXPOSE 3000
CMD ["npm", "run", "start"]
