package watcher

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"solana_paywall/backend/database"
	"solana_paywall/backend/enum"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Invoice struct {
	// ... (Invoice struct remains the same) ...
	ID            int64
	WalletAddress string
	Reference     string
	Signature     sql.NullString
	Amount        int64
	Status        string
	CreatedAt     time.Time
	RetryCount    int
	LastRetryAt   sql.NullTime
}

func Start() {
	log.Println("Starting transaction watcher...")
	// The primary watcher logic is now deprecated in favor of synchronous confirmation
	// and the sweeper. This watcher remains only to recover from explicit error states.

	// --- ADD ROUTINE 3: Reconciler Error Invoices ---
	go func() {
		// Run every 2 minutes
		recoverTicker := time.NewTicker(2 * time.Minute)
		defer recoverTicker.Stop()
		for range recoverTicker.C {
			RecoverErrorInvoices()
		}
	}()
}

// // Function to perform cleanup
// func cleanupExpiredInvoices() {
// 	// Logic: Find all 'pending' invoices created over 30 minutes ago -> Change to 'expired'
// 	// Interval '30 minutes' is a safe time for user interaction
// 	result, err := database.DB.Exec(`
//         UPDATE invoice
//         SET status = '` + enum.INVOICE_EXPIRED + `'
//         WHERE status = '` + enum.INVOICE_PENDING + `'
//         AND created_at < NOW() - INTERVAL '30 minutes'
//     `)

// 	if err != nil {
// 		log.Printf("Error cleaning up expired invoices: %v", err)
// 		return
// 	}

// 	rowsAffected, _ := result.RowsAffected()
// 	if rowsAffected > 0 {
// 		log.Printf("Garbage Collector: Cleaned up %d expired invoices.", rowsAffected)
// 	}
// }

// func checkPendingPayments() {
// 	// Bắt đầu 1 transaction để giữ khóa
// 	tx, err := database.DB.Begin()
// 	if err != nil {
// 		log.Printf("Error starting transaction: %v", err)
// 		return
// 	}
// 	// Defer rollback phòng trường hợp panic, nếu commit thành công thì rollback không tác dụng
// 	defer tx.Rollback()

// 	// The interval is in seconds, calculated as 2^retry_count, capped at 8 for interval calculation to prevent very long waits.
// 	rows, err := database.DB.Query(`
// 		SELECT id, wallet_address, reference, signature, amount, status, created_at, retry_count, last_retry_at
// 		FROM invoice
// 		WHERE status = '` + enum.INVOICE_PENDING + `' AND signature IS NOT NULL
// 		AND (
// 			last_retry_at IS NULL
// 			OR
// 			last_retry_at < NOW() - (interval '10 seconds' * power(2, LEAST(retry_count, 8)))
// 		)
// 	`)
// 	if err != nil {
// 		log.Printf("Error querying pending invoices: %v", err)
// 		return
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var invoice Invoice
// 		if err := rows.Scan(&invoice.ID, &invoice.WalletAddress, &invoice.Reference, &invoice.Signature, &invoice.Amount, &invoice.Status, &invoice.CreatedAt, &invoice.RetryCount, &invoice.LastRetryAt); err != nil {
// 			log.Printf("Error scanning invoice row: %v", err)
// 			continue
// 		}

// 		// Process invoices one by one to avoid overwhelming the RPC
// 		err = verifyAndCompleteTransaction(invoice)
// 		if err != nil {
// 			if strings.Contains(err.Error(), "Transaction not found") {
// 				handleTransactionNotFound(invoice, err)
// 			} else {
// 				log.Printf("Error verifying transaction for invoice %d: %v. Marking as failed.", invoice.ID, err)
// 				updateInvoiceStatus(invoice.ID, enum.INVOICE_ERROR)
// 			}
// 		}
// 	}
// 	tx.Commit() // Commit để giải phóng lock
// }

func RecoverErrorInvoices() {
	// 1. Lấy các invoice bị lỗi trong 24 giờ qua
	rows, err := database.DB.Query(`
        SELECT id, wallet_address, reference, signature, amount, status, created_at, retry_count, last_retry_at
        FROM invoice
        WHERE status = '` + enum.INVOICE_ERROR + `'
        AND created_at > NOW() - INTERVAL '24 hours'
    `)
	if err != nil {
		log.Printf("Reconciler: Error querying error invoices: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var invoice Invoice

		if err := rows.Scan(&invoice.ID, &invoice.WalletAddress, &invoice.Reference, &invoice.Signature, &invoice.Amount, &invoice.Status, &invoice.CreatedAt, &invoice.RetryCount, &invoice.LastRetryAt); err != nil {
			log.Printf("Reconciler: Failed to scan invoice row: %v", err)
			continue
		}

		log.Printf("Reconciler: Attempting to recover invoice %d", invoice.ID)

		// 2. Tái kiểm tra trên Blockchain
		// Lưu ý: Cần đảm bảo hàm verify này không throw panic
		err := verifyAndCompleteTransaction(invoice)

		if err == nil {
			// Case: Đã tìm thấy tiền và update thành PAID thành công bên trong hàm verify
			log.Printf("Reconciler: Successfully recovered invoice %d to PAID", invoice.ID)
		} else {
			// Case: Vẫn lỗi hoặc chưa thấy tiền
			log.Printf("Reconciler: Invoice %d still invalid. Reason: %v", invoice.ID, err)

			// Nếu lỗi là "Transaction not found" và đã quá lâu (ví dụ 60p) -> Có thể set về FAILED
			if time.Since(invoice.CreatedAt) > 60*time.Minute {
				updateInvoiceStatus(invoice.ID, enum.INVOICE_FAILED)
			}
		}
	}
}

// func handleTransactionNotFound(invoice Invoice, err error) {
// 	const maxRetries = 2
// 	if invoice.RetryCount >= maxRetries {
// 		log.Printf("Invoice %d exceeded max retries. Marking as failed. (Hint: Transaction might not have been sent by frontend)", invoice.ID)
// 		updateInvoiceStatus(invoice.ID, enum.INVOICE_FAILED)
// 		return
// 	}

// 	// Increment retry count and update last_retry_at timestamp.
// 	_, dbErr := database.DB.Exec(`
// 		UPDATE invoice
// 		SET retry_count = retry_count + 1, last_retry_at = NOW()
// 		WHERE id = $1
// 	`, invoice.ID)

// 	if dbErr != nil {
// 		log.Printf("Error updating retry count for invoice %d: %v", invoice.ID, dbErr)
// 	} else {
// 		// Log originalErr to see the actual error from JS script
// 		log.Printf("Transaction not found for invoice %d. Retrying later. Retry attempt %d. Details: %v", invoice.ID, invoice.RetryCount+1, err)
// 	}
// }

func updateInvoiceStatus(invoiceID int64, status string, reason ...string) {
	if len(reason) > 0 {
		_, err := database.DB.Exec("UPDATE invoice SET status = $1, err_reason = $2 WHERE id = $3", status, reason[0], invoiceID)
		if err != nil {
			log.Printf("Error updating invoice %d status to %s with reason %s: %v", invoiceID, status, reason[0], err)
		}
	} else {
		_, err := database.DB.Exec("UPDATE invoice SET status = $1 WHERE id = $2", status, invoiceID)
		if err != nil {
			log.Printf("Error updating invoice %d status to %s: %v", invoiceID, status, err)
		}
	}
}

func verifyAndCompleteTransaction(invoice Invoice) error {
	log.Printf("Checking transaction for invoice %d with signature %s", invoice.ID, invoice.Signature.String)

	err := VerifyTransaction(invoice.Reference, invoice.Signature.String, invoice.Amount)
	if err != nil {
		// If verification fails, update status to error and provide a reason
		updateInvoiceStatus(invoice.ID, enum.INVOICE_ERROR, err.Error())
		return err
	}

	// 7. Success
	updateInvoiceStatus(invoice.ID, enum.INVOICE_PAID)
	log.Printf("Payment confirmed for invoice %d", invoice.ID)
	return nil
}

// VerifyTransaction performs the core on-chain verification for a given transaction.
// It can be called from different contexts (API handlers, watchers, etc.).
func VerifyTransaction(reference string, signature string, expectedAmount int64) error {
	merchantWalletStr := os.Getenv("MERCHANT_WALLET")
	usdcMintStr := os.Getenv("USDC_MINT")
	if merchantWalletStr == "" || usdcMintStr == "" {
		return errors.New("MERCHANT_WALLET or USDC_MINT environment variable is not set")
	}
	return VerifyTransactionForMerchant(reference, signature, expectedAmount, merchantWalletStr)
}

func VerifyTransactionForMerchant(reference string, signature string, expectedAmount int64, merchantWalletStr string) error {
	usdcMintStr := os.Getenv("USDC_MINT")
	if merchantWalletStr == "" || usdcMintStr == "" {
		return errors.New("merchant wallet or USDC_MINT is not set")
	}

	// 1. Setup Client
	client := getRpcClientWithFailover()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	txResult, err := client.GetTransaction(
		ctx,
		sig,
		&rpc.GetTransactionOpts{
			Commitment: rpc.CommitmentConfirmed,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	if txResult == nil || txResult.Meta == nil {
		return errors.New("transaction data is empty")
	}

	// Check On-Chain errors
	if txResult.Meta.Err != nil {
		log.Printf("Transaction failed on-chain: %v", txResult.Meta.Err)
		return errors.New("payment verification failed: transaction failed on-chain")
	}

	// 6. Decode Transaction to read content
	tx, err := txResult.Transaction.GetTransaction()
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	// ================= VERIFY LOGIC =================

	merchantPubkey := solana.MustPublicKeyFromBase58(merchantWalletStr)
	usdcMint := solana.MustPublicKeyFromBase58(usdcMintStr)
	memoProgramIDV1 := solana.MustPublicKeyFromBase58("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")

	merchantAta, _, err := solana.FindAssociatedTokenAddress(
		merchantPubkey,
		usdcMint,
	)
	if err != nil {
		return fmt.Errorf("failed to derive merchant ATA: %w", err)
	}

	memoFound := false
	transferFound := false

	for _, instruction := range tx.Message.Instructions {
		if int(instruction.ProgramIDIndex) >= len(tx.Message.AccountKeys) {
			continue
		}
		programID := tx.Message.AccountKeys[instruction.ProgramIDIndex]

		if programID.Equals(solana.MemoProgramID) || programID.Equals(memoProgramIDV1) {
			memoText := string(instruction.Data)
			if strings.Contains(memoText, reference) {
				memoFound = true
			}
		}

		if programID.Equals(solana.TokenProgramID) {
			if len(instruction.Data) == 0 {
				continue
			}

			if instruction.Data[0] == 3 && len(instruction.Data) >= 9 { // Transfer
				if len(instruction.Accounts) < 3 {
					continue
				}
				amount := binary.LittleEndian.Uint64(instruction.Data[1:9])
				destIndex := instruction.Accounts[1]
				ownerIndex := instruction.Accounts[2]
				destKey := tx.Message.AccountKeys[destIndex]
				ownerKey := tx.Message.AccountKeys[ownerIndex]

				if ownerKey.Equals(merchantPubkey) {
					return errors.New("security violation: merchant self-payment")
				}
				if destKey.Equals(merchantAta) && amount == uint64(expectedAmount) {
					transferFound = true
				}
			} else if instruction.Data[0] == 12 && len(instruction.Data) >= 9 { // TransferChecked
				if len(instruction.Accounts) < 4 {
					continue
				}
				amount := binary.LittleEndian.Uint64(instruction.Data[1:9])
				destIndex := instruction.Accounts[2]
				ownerIndex := instruction.Accounts[3]
				destKey := tx.Message.AccountKeys[destIndex]
				ownerKey := tx.Message.AccountKeys[ownerIndex]

				if ownerKey.Equals(merchantPubkey) {
					return errors.New("security violation: merchant self-payment")
				}
				if destKey.Equals(merchantAta) && amount == uint64(expectedAmount) {
					transferFound = true
				}
			}
		}
	}

	if !memoFound {
		return fmt.Errorf("payment verification failed: memo '%s' not found", reference)
	}
	if !transferFound {
		return errors.New("payment verification failed: no matching SPL transfer found")
	}

	return nil
}

// VerifyDirectTransferForMerchant validates a direct transfer without requiring memo/reference.
// Used for manual transfer fallback where users transfer by scanning recipient QR.
func VerifyDirectTransferForMerchant(signature string, expectedAmount int64, merchantWalletStr string, senderWalletStr string) error {
	usdcMintStr := os.Getenv("USDC_MINT")
	if merchantWalletStr == "" || usdcMintStr == "" {
		return errors.New("merchant wallet or USDC_MINT is not set")
	}

	client := getRpcClientWithFailover()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	txResult, err := client.GetTransaction(
		ctx,
		sig,
		&rpc.GetTransactionOpts{
			Commitment: rpc.CommitmentConfirmed,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}
	if txResult == nil || txResult.Meta == nil {
		return errors.New("transaction data is empty")
	}
	if txResult.Meta.Err != nil {
		return errors.New("payment verification failed: transaction failed on-chain")
	}

	tx, err := txResult.Transaction.GetTransaction()
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	senderPubkey := solana.MustPublicKeyFromBase58(senderWalletStr)
	merchantPubkey := solana.MustPublicKeyFromBase58(merchantWalletStr)
	usdcMint := solana.MustPublicKeyFromBase58(usdcMintStr)
	merchantAta, _, err := solana.FindAssociatedTokenAddress(merchantPubkey, usdcMint)
	if err != nil {
		return fmt.Errorf("failed to derive merchant ATA: %w", err)
	}

	transferFound := false
	for _, instruction := range tx.Message.Instructions {
		if int(instruction.ProgramIDIndex) >= len(tx.Message.AccountKeys) {
			continue
		}
		programID := tx.Message.AccountKeys[instruction.ProgramIDIndex]
		if !programID.Equals(solana.TokenProgramID) || len(instruction.Data) == 0 {
			continue
		}

		if instruction.Data[0] == 3 && len(instruction.Data) >= 9 {
			if len(instruction.Accounts) < 3 {
				continue
			}
			// amount := binary.LittleEndian.Uint64(instruction.Data[1:9])
			destKey := tx.Message.AccountKeys[instruction.Accounts[1]]
			ownerKey := tx.Message.AccountKeys[instruction.Accounts[2]]
			if ownerKey.Equals(merchantPubkey) {
				return errors.New("security violation: merchant self-payment")
			}
			if !ownerKey.Equals(senderPubkey) {
				return errors.New("security violation: sender mismatch")
			}
			if destKey.Equals(merchantAta) {
				transferFound = true
			}
		} else if instruction.Data[0] == 12 && len(instruction.Data) >= 9 {
			if len(instruction.Accounts) < 4 {
				continue
			}
			// amount := binary.LittleEndian.Uint64(instruction.Data[1:9])
			destKey := tx.Message.AccountKeys[instruction.Accounts[2]]
			ownerKey := tx.Message.AccountKeys[instruction.Accounts[3]]
			if ownerKey.Equals(merchantPubkey) {
				return errors.New("security violation: merchant self-payment")
			}
			if destKey.Equals(merchantAta) {
				transferFound = true
			}
		}
	}

	if !transferFound {
		return errors.New("payment verification failed: no matching SPL transfer found")
	}
	return nil
}

// ForceRecheckByReference allows API to trigger re-verification of a specific invoice
func ForceRecheckByReference(reference string, walletAddress string) (string, error) {
	// 1. Find invoice by reference in the database
	var invoice Invoice
	err := database.DB.QueryRow(`
		SELECT id, wallet_address, reference, signature, amount, status, created_at, retry_count, last_retry_at
		FROM invoice WHERE reference = $1
	`, reference).Scan(&invoice.ID, &invoice.WalletAddress, &invoice.Reference, &invoice.Signature, &invoice.Amount, &invoice.Status, &invoice.CreatedAt, &invoice.RetryCount, &invoice.LastRetryAt)

	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("error querying database: %w", err)
	}

	// 2. If already paid, report immediately
	if err == nil && invoice.WalletAddress != walletAddress {
		return "not_found", nil
	}
	if err == nil && invoice.Status == enum.INVOICE_PAID {
		return enum.INVOICE_PAID, nil
	}

	// If it's in the DB but not paid (e.g., status is 'error'), try to re-verify
	if err == nil && invoice.Signature.Valid {
		log.Printf("Re-checking existing invoice %d from DB.", invoice.ID)
		err = verifyAndCompleteTransaction(invoice)
		if err == nil {
			return enum.INVOICE_PAID, nil
		}
	}

	// 3. If not in DB, check Redis
	redisKey := fmt.Sprintf("invoice:%s", reference)
	redisStatus, err := database.RDB.Exists(database.Ctx, redisKey).Result()
	if err != nil {
		return "", fmt.Errorf("error checking redis: %w", err)
	}
	if redisStatus > 0 {
		invoiceData, redisErr := database.RDB.HGetAll(database.Ctx, redisKey).Result()
		if redisErr != nil {
			return "", fmt.Errorf("error reading redis invoice data: %w", redisErr)
		}
		if invoiceData["wallet_address"] != walletAddress {
			return "not_found", nil
		}
		// return enum.INVOICE_PENDING, nil // It exists, but hasn't been confirmed by the user yet.
	}

	// 4. If not in DB and not in Redis, it might be an orphaned transaction.
	// Perform a targeted sweep.
	log.Printf("Invoice %s not found in DB or Redis. Performing targeted sweep...", reference)
	recoveredStatus, err := targetedSweep(reference)
	if err != nil {
		return "", err
	}

	return recoveredStatus, nil
}

// targetedSweep looks for a specific reference on the blockchain.
func targetedSweep(reference string) (string, error) {
	merchantWalletStr := os.Getenv("MERCHANT_WALLET")
	if merchantWalletStr == "" {
		return "", errors.New("MERCHANT_WALLET environment variable is not set")
	}
	merchantPubkey := solana.MustPublicKeyFromBase58(merchantWalletStr)

	client := getRpcClientWithFailover()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	limit := 30
	// Get recent transactions and scan for our memo
	signatures, err := client.GetSignaturesForAddressWithOpts(ctx, merchantPubkey, &rpc.GetSignaturesForAddressOpts{
		Limit: &limit, // Check a decent number of recent txs
	})
	if err != nil {
		return "", fmt.Errorf("failed to get signatures for address: %w", err)
	}

	memoProgramIDV1 := solana.MustPublicKeyFromBase58("Memo1UhkJRfHyvLMcVucJwxXeuD728EqVDDwQDxFMNo")

	for _, txSig := range signatures {
		// Fetch the full transaction to reliably get the memo
		version := uint64(0)
		txResult, err := client.GetTransaction(ctx, txSig.Signature, &rpc.GetTransactionOpts{
			MaxSupportedTransactionVersion: &version,
		})
		if err != nil || txResult == nil {
			continue // Skip if we can't get the transaction
		}

		tx, err := txResult.Transaction.GetTransaction()
		if err != nil {
			continue
		}

		// Look for the reference in the memo
		for _, instruction := range tx.Message.Instructions {
			programID := tx.Message.AccountKeys[instruction.ProgramIDIndex]
			if programID.Equals(solana.MemoProgramID) || programID.Equals(memoProgramIDV1) {
				if string(instruction.Data) == reference {
					log.Printf("Targeted Sweep: Found matching memo for %s in signature %s", reference, txSig.Signature)
					// We found it on-chain. The main sweeper will pick it up and process it.
					// We can't process it here because the invoice data from Redis is gone.
					return "confirmed_on_chain", nil
				}
			}
		}
	}

	return "not_found", nil
}

// Tạo danh sách RPC (Nên để trong .env ngăn cách bởi dấu phẩy)
func getRpcClientWithFailover() *rpc.Client {
	primaryURL := os.Getenv("SOLANA_RPC_URL")
	backupURL := "https://api.devnet.solana.com" // Backup public

	// Thử Ping RPC chính
	client := rpc.New(primaryURL)
	_, err := client.GetHealth(context.Background())
	if err == nil {
		return client
	}

	log.Printf("Warning: Primary RPC %s is down/slow. Switching to backup.", primaryURL)

	// Nếu lỗi, trả về backup
	return rpc.New(backupURL)
}
