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
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type CheckoutRecoveryRow struct {
	ID             int64
	EventID        int64
	WalletAddress  string
	Reference      string
	Signature      string
	Amount         int64
	Status         string
	CreatedAt      time.Time
	MerchantWallet string
}

func Start() {
	log.Println("Starting transaction watcher...")
	// The primary watcher logic is now deprecated in favor of synchronous confirmation.
	// This watcher remains only to recover inconsistent checkout states.
	go func() {
		recoverTicker := time.NewTicker(2 * time.Minute)
		defer recoverTicker.Stop()
		for range recoverTicker.C {
			RecoverErrorInvoices()
		}
	}()
}

func RecoverErrorInvoices() {
	// Recover stale/failed event checkouts with signature by re-verifying on-chain.
	rows, err := database.DB.Query(`
		SELECT ec.id, ec.event_id, COALESCE(ec.wallet_address, ''), ec.reference, COALESCE(ec.signature, ''), ec.amount,
		       ec.status::text, ec.created_at, e.merchant_wallet
		FROM event_checkouts ec
		JOIN events e ON e.id = ec.event_id
		WHERE ec.created_at > NOW() - INTERVAL '24 hours'
		  AND ec.status IN ('pending_payment', 'failed')
		  AND ec.signature IS NOT NULL
		  AND ec.signature <> ''
		ORDER BY ec.created_at DESC
		LIMIT 200
	`)
	if err != nil {
		log.Printf("Reconciler: Error querying recoverable event_checkouts: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var row CheckoutRecoveryRow
		if err := rows.Scan(
			&row.ID,
			&row.EventID,
			&row.WalletAddress,
			&row.Reference,
			&row.Signature,
			&row.Amount,
			&row.Status,
			&row.CreatedAt,
			&row.MerchantWallet,
		); err != nil {
			log.Printf("Reconciler: Failed to scan event_checkouts row: %v", err)
			continue
		}

		log.Printf("Reconciler: Attempting to recover checkout %d (event %d)", row.ID, row.EventID)

		senderWallet, verifyErr := VerifyTransactionForMerchantWithSender(
			row.Reference,
			row.Signature,
			row.Amount,
			row.MerchantWallet,
		)

		if verifyErr == nil {
			if err := markCheckoutPaid(row.ID, row.Signature, senderWallet); err != nil {
				log.Printf("Reconciler: Failed to mark checkout %d paid: %v", row.ID, err)
				continue
			}
			log.Printf("Reconciler: Successfully recovered checkout %d to paid", row.ID)
		} else {
			log.Printf("Reconciler: Checkout %d still invalid. Reason: %v", row.ID, verifyErr)
			if time.Since(row.CreatedAt) > 60*time.Minute {
				if err := markCheckoutFailed(row.ID); err != nil {
					log.Printf("Reconciler: Failed to mark checkout %d failed: %v", row.ID, err)
				}
			}
		}
	}
}

func markCheckoutPaid(checkoutID int64, signature string, senderWallet string) error {
	_, err := database.DB.Exec(`
		UPDATE event_checkouts
		SET status = 'paid',
		    signature = $2,
		    paid_at = COALESCE(paid_at, NOW()),
		    wallet_address = CASE
		      WHEN COALESCE(wallet_address, '') = '' THEN $3
		      ELSE wallet_address
		    END
		WHERE id = $1
	`, checkoutID, signature, senderWallet)
	return err
}

func markCheckoutFailed(checkoutID int64) error {
	_, err := database.DB.Exec(`
		UPDATE event_checkouts
		SET status = 'failed'
		WHERE id = $1
		  AND status <> 'paid'
	`, checkoutID)
	return err
}

// VerifyTransaction remains as backward-compatible signature-based verifier
// and now resolves merchant wallet per checkout reference from event data.
func VerifyTransaction(reference string, signature string, expectedAmount int64) error {
	merchantWallet, err := merchantWalletByReference(reference)
	if err != nil {
		return err
	}
	return VerifyTransactionForMerchant(reference, signature, expectedAmount, merchantWallet)
}

func merchantWalletByReference(reference string) (string, error) {
	var merchantWallet string
	err := database.DB.QueryRow(`
		SELECT e.merchant_wallet
		FROM event_checkouts ec
		JOIN events e ON e.id = ec.event_id
		WHERE ec.reference = $1
		LIMIT 1
	`, reference).Scan(&merchantWallet)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("merchant wallet not found for reference")
		}
		return "", fmt.Errorf("failed to resolve merchant wallet by reference: %w", err)
	}
	if strings.TrimSpace(merchantWallet) == "" {
		return "", errors.New("merchant wallet is empty for reference")
	}
	return strings.TrimSpace(merchantWallet), nil
}

func VerifyTransactionForMerchant(reference string, signature string, expectedAmount int64, merchantWalletStr string) error {
	_, err := VerifyTransactionForMerchantWithSender(reference, signature, expectedAmount, merchantWalletStr)
	return err
}

func VerifyTransactionForMerchantWithSender(reference string, signature string, expectedAmount int64, merchantWalletStr string) (string, error) {
	usdcMintStr := os.Getenv("USDC_MINT")
	if merchantWalletStr == "" || usdcMintStr == "" {
		return "", errors.New("merchant wallet or USDC_MINT is not set")
	}

	// 1. Setup Client
	client := getRpcClientWithFailover()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return "", fmt.Errorf("invalid signature format: %w", err)
	}

	maxTxVersion := uint64(0)
	txResult, err := client.GetTransaction(
		ctx,
		sig,
		&rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &maxTxVersion,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction: %w", err)
	}

	if txResult == nil || txResult.Meta == nil {
		return "", errors.New("transaction data is empty")
	}

	// Check On-Chain errors
	if txResult.Meta.Err != nil {
		log.Printf("Transaction failed on-chain: %v", txResult.Meta.Err)
		return "", errors.New("payment verification failed: transaction failed on-chain")
	}

	// 6. Decode Transaction to read content
	tx, err := txResult.Transaction.GetTransaction()
	if err != nil {
		return "", fmt.Errorf("failed to decode transaction: %w", err)
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
		return "", fmt.Errorf("failed to derive merchant ATA: %w", err)
	}

	memoFound := false
	transferFound := false
	senderWallet := ""

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
					return "", errors.New("security violation: merchant self-payment")
				}
				if destKey.Equals(merchantAta) && amount == uint64(expectedAmount) {
					transferFound = true
					senderWallet = ownerKey.String()
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
					return "", errors.New("security violation: merchant self-payment")
				}
				if destKey.Equals(merchantAta) && amount == uint64(expectedAmount) {
					transferFound = true
					senderWallet = ownerKey.String()
				}
			}
		}
	}

	if !memoFound {
		return "", fmt.Errorf("payment verification failed: memo '%s' not found", reference)
	}
	if !transferFound {
		return "", errors.New("payment verification failed: no matching SPL transfer found")
	}
	if senderWallet == "" {
		// Fallback: infer sender from transaction signers when token instruction authority
		// is not directly available in parsed transfer accounts.
		requiredSigners := int(tx.Message.Header.NumRequiredSignatures)
		if requiredSigners > len(tx.Message.AccountKeys) {
			requiredSigners = len(tx.Message.AccountKeys)
		}
		for i := 0; i < requiredSigners; i++ {
			candidate := tx.Message.AccountKeys[i]
			if !candidate.Equals(merchantPubkey) {
				senderWallet = candidate.String()
				break
			}
		}
	}

	return senderWallet, nil
}

func DetectSignatureByReferenceForMerchant(reference string, expectedAmount int64, merchantWalletStr string) (string, string, error) {
	usdcMintStr := os.Getenv("USDC_MINT")
	if merchantWalletStr == "" || usdcMintStr == "" {
		return "", "", errors.New("merchant wallet or USDC_MINT is not set")
	}

	merchantPubkey := solana.MustPublicKeyFromBase58(merchantWalletStr)
	usdcMint := solana.MustPublicKeyFromBase58(usdcMintStr)
	merchantAta, _, err := solana.FindAssociatedTokenAddress(merchantPubkey, usdcMint)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive merchant ATA: %w", err)
	}

	client := getRpcClientWithFailover()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	limit := 30
	sigs, err := client.GetSignaturesForAddressWithOpts(ctx, merchantAta, &rpc.GetSignaturesForAddressOpts{
		Limit:      &limit,
		Commitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return "", "", err
	}

	for _, txSig := range sigs {
		if txSig == nil || txSig.Memo == nil {
			continue
		}
		if !strings.Contains(*txSig.Memo, reference) {
			continue
		}
		signature := txSig.Signature.String()
		sender, verifyErr := VerifyTransactionForMerchantWithSender(reference, signature, expectedAmount, merchantWalletStr)
		if verifyErr == nil {
			return signature, sender, nil
		}
	}

	return "", "", errors.New("transaction not detected yet")
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

	maxTxVersion := uint64(0)
	txResult, err := client.GetTransaction(
		ctx,
		sig,
		&rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &maxTxVersion,
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
			amount := binary.LittleEndian.Uint64(instruction.Data[1:9])
			destKey := tx.Message.AccountKeys[instruction.Accounts[1]]
			ownerKey := tx.Message.AccountKeys[instruction.Accounts[2]]
			if ownerKey.Equals(merchantPubkey) {
				return errors.New("security violation: merchant self-payment")
			}
			if !ownerKey.Equals(senderPubkey) {
				return errors.New("security violation: sender mismatch")
			}
			if destKey.Equals(merchantAta) && amount == uint64(expectedAmount) {
				transferFound = true
			}
		} else if instruction.Data[0] == 12 && len(instruction.Data) >= 9 {
			if len(instruction.Accounts) < 4 {
				continue
			}
			amount := binary.LittleEndian.Uint64(instruction.Data[1:9])
			destKey := tx.Message.AccountKeys[instruction.Accounts[2]]
			ownerKey := tx.Message.AccountKeys[instruction.Accounts[3]]
			if ownerKey.Equals(merchantPubkey) {
				return errors.New("security violation: merchant self-payment")
			}
			if !ownerKey.Equals(senderPubkey) {
				return errors.New("security violation: sender mismatch")
			}
			if destKey.Equals(merchantAta) && amount == uint64(expectedAmount) {
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
	backupURL := "https://api.mainnet-beta.solana.com" // Backup public

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
