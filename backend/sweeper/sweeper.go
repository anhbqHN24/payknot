package sweeper

import (
	"context"
	"fmt"
	"log"
	"os"
	"solana_paywall/backend/database"
	"solana_paywall/backend/enum"
	"solana_paywall/backend/watcher"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func Start() {
	log.Println("Starting merchant wallet sweeper...")
	go func() {
		// Run once on startup
		sweepAndRecover()
		// Then run on a ticker
		ticker := time.NewTicker(1 * time.Minute) // Check every 1 minute
		defer ticker.Stop()

		for range ticker.C {
			sweepAndRecover()
		}
	}()
}

func sweepAndRecover() {
	merchantWalletStr := os.Getenv("MERCHANT_WALLET")
	if merchantWalletStr == "" {
		log.Println("Sweeper: MERCHANT_WALLET environment variable is not set. Aborting sweep.")
		return
	}
	merchantPubkey := solana.MustPublicKeyFromBase58(merchantWalletStr)

	client := rpc.New(os.Getenv("SOLANA_RPC_URL"))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	limit := 50
	// Get the most recent 50 transaction signatures for the merchant's wallet
	signatures, err := client.GetSignaturesForAddressWithOpts(ctx, merchantPubkey, &rpc.GetSignaturesForAddressOpts{
		Limit: &limit,
	})
	if err != nil {
		log.Printf("Sweeper: Failed to get signatures for address: %v", err)
		return
	}

	candidateCount := 0
	recoveredCount := 0
	for _, txSig := range signatures {
		// For each signature, check if it's an orphaned payment
		candidate, recovered := processSignature(client, txSig.Signature)
		if candidate {
			candidateCount++
		}
		if recovered {
			recoveredCount++
		}
	}

	// Keep logs actionable: emit only when there are actual recoverable candidates.
	if candidateCount > 0 {
		log.Printf("Sweeper: Processed %d recoverable candidate(s), recovered %d.", candidateCount, recoveredCount)
	}
}

func processSignature(client *rpc.Client, signature solana.Signature) (bool, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	version := uint64(0)
	txResult, err := client.GetTransaction(ctx, signature, &rpc.GetTransactionOpts{
		MaxSupportedTransactionVersion: &version,
	})
	if err != nil {
		// This can be noisy, so we log it conditionally or with less priority
		// log.Printf("Sweeper: Failed to get transaction details for sig %s: %v", signature, err)
		return false, false
	}

	if txResult == nil || txResult.Transaction == nil {
		return false, false
	}

	tx, err := txResult.Transaction.GetTransaction()
	if err != nil {
		return false, false
	}

	// Find the memo which contains our reference
	reference := ""
	memoProgramIDV1 := solana.MustPublicKeyFromBase58("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")

	for _, instruction := range tx.Message.Instructions {
		programID := tx.Message.AccountKeys[instruction.ProgramIDIndex]
		if programID.Equals(solana.MemoProgramID) || programID.Equals(memoProgramIDV1) {
			// A simple heuristic: if the memo looks like a UUID, it's probably ours.
			memo := string(instruction.Data)
			if len(memo) == 36 && strings.Count(memo, "-") == 4 {
				reference = memo
				break
			}
		}
	}

	if reference == "" {
		// Not a transaction with a valid reference memo
		return false, false
	}

	// Now that we have a reference, check if it's a pending invoice in Redis
	redisKey := fmt.Sprintf("invoice:%s", reference)
	invoiceData, err := database.RDB.HGetAll(database.Ctx, redisKey).Result()
	if err != nil || len(invoiceData) == 0 {
		// If it's not in Redis, it's either already processed or not ours.
		return false, false
	}

	log.Printf("Sweeper: Found potentially orphaned invoice for reference %s. Attempting recovery...", reference)

	// We found an orphaned invoice! Let's recover it.
	walletAddress := invoiceData["wallet_address"]
	amountStr := invoiceData["amount"]
	amount, _ := strconv.ParseInt(amountStr, 10, 64)

	// Use the already-existing VerifyTransaction function.
	// We already have the transaction, so this is slightly inefficient, but reuses code well.
	err = watcher.VerifyTransaction(reference, signature.String(), amount)
	if err != nil {
		log.Printf("Sweeper: Verification failed for recovered invoice %s: %v", reference, err)
		// We could potentially mark this as an error in Redis to avoid re-checking.
		return true, false
	}

	// Verification successful! Save to DB.
	_, err = database.DB.Exec(`
		INSERT INTO invoice (wallet_address, reference, amount, signature, status) 
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (reference) DO NOTHING`, // Prevent race conditions
		walletAddress, reference, amount, signature.String(), enum.INVOICE_PAID,
	)
	if err != nil {
		log.Printf("Sweeper: Failed to save recovered invoice %s to DB: %v", reference, err)
		return true, false
	}

	// Clean up from Redis
	database.RDB.Del(database.Ctx, redisKey)

	log.Printf("Sweeper: Successfully recovered invoice %s!", reference)
	return true, true
}
