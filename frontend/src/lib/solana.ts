import {
  Connection,
  PublicKey,
  Transaction,
  TransactionInstruction,
} from "@solana/web3.js";
import {
  getAssociatedTokenAddress,
  getAssociatedTokenAddressSync,
  getAccount,
  createTransferInstruction,
  createAssociatedTokenAccountIdempotentInstruction,
} from "@solana/spl-token";

// ----------------------------------------------------------------------
// NOTE: Ensure this is the correct USDC Mint address.
// ----------------------------------------------------------------------
export const USDC_MINT_ADDRESS = new PublicKey(
  process.env.NEXT_PUBLIC_USDC_MINT ||
    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
);
/** Returns true if the string is a valid Solana base58 public key. */
export function isValidSolanaAddress(value: string): boolean {
  if (!value || value.length < 32 || value.length > 44) return false;
  try {
    new PublicKey(value);
    return true;
  } catch {
    return false;
  }
}

/**
 * Fetches the USDC balance for a given wallet address.
 *
 * @param connection - The Solana Connection object.
 * @param publicKey - The public key of the wallet.
 * @returns The USDC balance, or null if the account doesn't exist.
 */
export async function getUsdcBalance(
  connection: Connection,
  publicKey: PublicKey,
): Promise<number | null> {
  try {
    const ata = await getAssociatedTokenAddress(USDC_MINT_ADDRESS, publicKey);
    const account = await getAccount(connection, ata);
    return Number(account.amount) / 1_000_000;
  } catch (error) {
    console.warn("Could not fetch USDC balance (likely no account):", error);
    return 0;
  }
}

/**
 * Creates a USDC transfer transaction.
 *
 * @param connection
 * @param publicKey
 * @param recipient
 * @param amount
 * @returns
 */
export async function createUsdcTransfer(
  _connection: Connection,
  publicKey: PublicKey,
  recipient: PublicKey,
  amount: number,
  memo: string,
): Promise<Transaction> {
  const senderAta = getAssociatedTokenAddressSync(USDC_MINT_ADDRESS, publicKey);
  const recipientAta = getAssociatedTokenAddressSync(
    USDC_MINT_ADDRESS,
    recipient,
  );

  // Atomic ATA readiness: no preflight ATA check, always include idempotent create.
  // If ATA exists, instruction is a no-op; if missing, sender funds rent in same tx.
  const transaction = new Transaction();
  transaction.add(
    createAssociatedTokenAccountIdempotentInstruction(
      publicKey,
      recipientAta,
      recipient,
      USDC_MINT_ADDRESS,
    ),
    createTransferInstruction(
      senderAta,
      recipientAta,
      publicKey,
      amount * 1_000_000, // Amount in smallest unit (6 decimals for USDC)
    ),
    new TransactionInstruction({
      keys: [{ pubkey: publicKey, isSigner: true, isWritable: true }],
      data: Buffer.from(memo, "utf-8"),
      programId: new PublicKey("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo"),
    }),
  );
  return transaction;
}
