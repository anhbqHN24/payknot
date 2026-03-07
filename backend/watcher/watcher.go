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
	// The primary watcher logic is now deprecated in favor of synchronous confirmation.
	// This watcher remains only to recover from explicit error states.

	// --- ADD ROUTINE: Reconciler Error Invoices ---
	go func() {
		// Run every 2 minutes
		recoverTicker := time.NewTicker(2 * time.Minute)
		defer recoverTicker.Stop()
		for range recoverTicker.C {
			RecoverErrorInvoices()
		}
	}()
}
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
