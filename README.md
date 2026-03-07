# Event Deposit Checkout Platform (Solana USDC)

A full-stack event deposit management product on Solana.

## What It Does
- Hosts create event checkout sessions.
- Hosts generate invite codes and share checkout links.
- Participants validate invite codes, pay USDC, and submit tx proof.
- Hosts approve or reject deposits with reason.

## Current Architecture
- Frontend: Next.js 14 + Tailwind + Wallet Adapter + Antd upload/crop
- Backend: Go (net/http) + PostgreSQL + Redis
- Blockchain: Solana RPC verification (Go SDK)
- Auth: Backend session auth (JWT in HttpOnly cookie + DB-backed session rows)

## Core Routes
- Host dashboard: `/`
- Participant checkout: `/checkout/[slug]`

## Quick Start
Backend:
```bash
cd backend
go run main.go
```

Frontend:
```bash
cd frontend
npm run dev
```

## Docker Compose (single env source)
Use one env file for all services:
```bash
cp .env.compose.example .env.compose
# edit values in .env.compose

docker compose --env-file .env.compose up -d --build
```

Services:
- Frontend: http://localhost:3000
- Backend: http://localhost:8080
- Postgres: localhost:5432
- Redis: localhost:6379

## Required Backend Env
- `DATABASE_URL`
- `REDIS_URL`
- `SOLANA_RPC_URL`
- `USDC_MINT`
- `AUTH_JWT_SECRET`
- optional: `GOOGLE_CLIENT_ID`, `AUTH_COOKIE_NAME`, `AUTH_COOKIE_SECURE`

## Required Frontend Env
- `NEXT_PUBLIC_SOLANA_RPC_URL`
- `NEXT_PUBLIC_USDC_MINT`
- `NEXT_PUBLIC_GOOGLE_CLIENT_ID` (for Google login button)

## Required Migrations
Apply all migrations in order, including:
- `000005_event_checkout_platform`
- `000006_add_event_image`
- `000007_add_event_owner_fields`
- `000008_add_rejected_checkout_status`
- `000009_add_auth_tables`

## Project Docs (Must-Have)
- [Product Spec](./PRODUCT_SPEC.md)
- [API Contract](./API_CONTRACT.md)
- [Database Schema & Migrations](./DATABASE_SCHEMA.md)
- [Auth & Security](./AUTH_SECURITY.md)
- [Payment Integrity](./PAYMENT_INTEGRITY.md)
- [Operations Runbook](./OPERATIONS_RUNBOOK.md)
- [QA / UAT Test Plan](./QA_UAT_TEST_PLAN.md)
- [Privacy Policy (Draft)](./PRIVACY_POLICY.md)
- [Terms of Service (Draft)](./TERMS_OF_SERVICE.md)
- [Changelog](./CHANGELOG.md)
