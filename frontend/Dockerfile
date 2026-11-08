FROM node:24.21.0-alpine3.24 AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Vite embeds these public settings into the static bundle at build time.
ARG VITE_API_BASE_URL=/api
ARG VITE_SOLANA_RPC_URL=/solana-rpc
ARG VITE_SOLANA_CLUSTER=devnet
ARG VITE_SOLANA_PROGRAM_ID
ARG VITE_SOLANA_USDC_MINT
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL} \
    VITE_SOLANA_RPC_URL=${VITE_SOLANA_RPC_URL} \
    VITE_SOLANA_CLUSTER=${VITE_SOLANA_CLUSTER} \
    VITE_SOLANA_PROGRAM_ID=${VITE_SOLANA_PROGRAM_ID} \
    VITE_SOLANA_USDC_MINT=${VITE_SOLANA_USDC_MINT}

RUN npm run build

FROM caddy:2.11.9-alpine

COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=builder /app/dist /srv

EXPOSE 80 443 443/udp
