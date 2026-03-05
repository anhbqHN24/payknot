# Product Spec

## Product
Event Deposit Checkout Builder on Solana USDC.

## Problem
Event hosts need a simple way to collect participant deposits with invite-code control, receipt verification, and approval/rejection workflow.

## Target Users
- Event hosts (create/manage event checkout sessions)
- Participants (pay deposit with invite code)

## Core Value
- Host can create an event checkout link quickly.
- Participant can validate invite code and pay USDC.
- Host can approve/reject participant deposits with a clear ledger.

## Primary Flows
1. Host auth
- Register/login (password or Google)
- Session is backend-authenticated via HttpOnly cookie

2. Host event management
- Create event metadata
- Generate/manage invite codes
- Share checkout URL
- Review participant deposits
- Approve/reject with reason

3. Participant checkout
- Open event checkout page
- Validate invite code
- Pay deposit (wallet flow or manual transfer verification)
- Receive transaction receipt + status

## MVP Scope
- Solana USDC deposit flow
- Invite code gate
- Host dashboard CRUD for events
- Participant deposit review/approval/rejection
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
- Host can reliably decide approve/reject status
