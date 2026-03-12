# Product Spec

## Product
Event Deposit Checkout Builder on Solana USDC.

## Problem
Event hosts need a simple way to collect participant deposits with customizable participant forms, receipt verification, and transaction tracking.

## Target Users
- Event hosts (create/manage event checkout sessions)
- Participants (submit required info and pay deposit)

## Core Value
- Host can create/import event data quickly and publish checkout links.
- Participant can complete required form fields and pay USDC.
- Host can track transaction receipts with clear on-chain references.

## Primary Flows
1. Host auth
- Register/login (password or Google)
- Session is backend-authenticated via HttpOnly cookie

2. Host event management
- Create event metadata
- Choose event source (custom or Luma import)
- Define checkout participant fields (default name/email + custom fields)
- Choose payment methods (wallet / QR)
- Share checkout URL
- Review participant deposits

3. Participant checkout
- Open event checkout page
- Fill required participant fields
- Pay deposit (wallet flow or QR/manual transfer verification)
- Receive transaction receipt + status

## MVP Scope
- Solana USDC deposit flow
- Host dashboard CRUD for events
- Dynamic participant form schema per event
- QR + wallet payment options
- Participant deposit transaction view
- Manual signature verification and recheck
- Mobile + desktop responsive management UI

## Non-Goals (Current)
- Multi-org/team roles
- Advanced analytics/reporting
- Refund automation
- Fiat payment rails

## Success Criteria
- Host can run end-to-end event deposit flow without manual database work
- Participants can submit verifiable on-chain payment proof
- Host can reliably view transaction states and references
