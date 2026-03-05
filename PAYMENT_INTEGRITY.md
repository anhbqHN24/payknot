# Payment Integrity

## Verification Principles
- Verify on-chain transaction against:
  - recipient merchant wallet ATA
  - expected token mint (USDC)
  - expected amount
  - reference memo when applicable
- Reject merchant self-payment patterns.

## Confirmation Paths
1. Wallet flow
- create invoice reference
- submit signature
- backend verifies tx and updates DB

2. Manual transfer flow
- participant submits tx signature + wallet
- backend verifies direct transfer and records paid receipt

3. Recheck/recovery
- `/checkout/recheck` for explicit reconciliation
- watcher/sweeper recover error/pending inconsistencies

## Failure Handling
- If tx confirmed on-chain but backend write fails:
  - recheck/manual-verify paths provide eventual recovery
- Pending abandoned records can be cancelled via `/checkout/cancel`.

## Operational Guardrails
- Use reliable RPC endpoints with failover
- Persist verification failures with reason
- Monitor mismatch/retry rates
