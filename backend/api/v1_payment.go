package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"
	"solana_paywall/backend/watcher"
)

type V1CreatePaymentSessionRequest struct {
	EventID         int64                  `json:"eventId"`
	Slug            string                 `json:"slug"`
	PaymentMethod   string                 `json:"paymentMethod"`
	WalletAddress   string                 `json:"walletAddress"`
	ParticipantData map[string]interface{} `json:"participantData"`
}

type V1PaymentSessionResponse struct {
	SessionID    string `json:"sessionId"`
	Reference    string `json:"reference"`
	State        string `json:"state"`
	PaymentMethod string `json:"paymentMethod"`
	AmountAtomic int64  `json:"amountAtomic"`
	Mint         string `json:"mint"`
	ExpiresAt    string `json:"expiresAt"`
}

type V1SubmitSignatureRequest struct {
	Signature string `json:"signature"`
}

func V1CreatePaymentSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := middleware.CurrentUser(r)
	ownerEmail := "public_checkout"
	if ok && strings.TrimSpace(claims.Email) != "" {
		ownerEmail = strings.TrimSpace(claims.Email)
	}

	var req V1CreatePaymentSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	req.WalletAddress = strings.TrimSpace(req.WalletAddress)
	req.Slug = strings.TrimSpace(req.Slug)

	var slug string
	var amount int64
	var merchantWallet string
	var formJSON []byte
	var methodsJSON []byte
	var eventID int64
	var err error
	if req.EventID > 0 {
		err = database.DB.QueryRow(`
			SELECT id, slug, amount_usdc, merchant_wallet, participant_form_schema, payment_methods
			FROM events
			WHERE id = $1 AND status = 'active'
		`, req.EventID).Scan(&eventID, &slug, &amount, &merchantWallet, &formJSON, &methodsJSON)
	} else {
		err = database.DB.QueryRow(`
			SELECT id, slug, amount_usdc, merchant_wallet, participant_form_schema, payment_methods
			FROM events
			WHERE slug = $1 AND status = 'active'
		`, req.Slug).Scan(&eventID, &slug, &amount, &merchantWallet, &formJSON, &methodsJSON)
	}
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	fields := decodeParticipantFields(formJSON)
	if err := validateParticipantData(fields, req.ParticipantData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ensureParticipantEmailAvailable(eventID, req.ParticipantData); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	methods := map[string]bool{"wallet": true, "qr": true}
	_ = json.Unmarshal(methodsJSON, &methods)
	if req.PaymentMethod == "wallet" && !methods["wallet"] {
		http.Error(w, "wallet method disabled", http.StatusBadRequest)
		return
	}
	if req.PaymentMethod == "qr" && !methods["qr"] {
		http.Error(w, "qr method disabled", http.StatusBadRequest)
		return
	}
	if req.PaymentMethod != "wallet" && req.PaymentMethod != "qr" {
		http.Error(w, "invalid payment method", http.StatusBadRequest)
		return
	}
	if req.PaymentMethod == "wallet" && !isValidWalletAddress(req.WalletAddress) {
		http.Error(w, "walletAddress is required for wallet", http.StatusBadRequest)
		return
	}

	sessionID := uuid.NewString()
	reference := uuid.NewString()
	expiresAt := time.Now().Add(20 * time.Minute)
	participantJSON, _ := json.Marshal(req.ParticipantData)

	redisKey := fmt.Sprintf("invoice:%s", reference)
	invoiceData := map[string]interface{}{
		"wallet_address":   req.WalletAddress,
		"amount":           amount,
		"event_id":         eventID,
		"merchant_wallet":  merchantWallet,
		"payment_method":   req.PaymentMethod,
		"participant_data": string(participantJSON),
	}
	if err := database.RDB.HSet(database.Ctx, redisKey, invoiceData).Err(); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	_ = database.RDB.Expire(database.Ctx, redisKey, 20*time.Minute).Err()

	_, err = database.DB.Exec(`
		INSERT INTO payment_sessions (
			id, event_id, owner_email, participant_data, wallet_address, payment_method, state,
			reference, amount_atomic, mint, merchant_wallet, idempotency_key, expires_at
		)
		VALUES ($1::uuid,$2,$3,$4::jsonb,$5,$6,'awaiting_payment',$7::uuid,$8,$9,$10,$11,$12)
	`, sessionID, eventID, ownerEmail, string(participantJSON), req.WalletAddress, req.PaymentMethod,
		reference, amount, usdcMintAddress(), merchantWallet, strings.TrimSpace(r.Header.Get("Idempotency-Key")), expiresAt)
	if err != nil {
		http.Error(w, "Failed to persist session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(V1PaymentSessionResponse{
		SessionID:    sessionID,
		Reference:    reference,
		State:        "awaiting_payment",
		PaymentMethod: req.PaymentMethod,
		AmountAtomic: amount,
		Mint:         usdcMintAddress(),
		ExpiresAt:    expiresAt.UTC().Format(time.RFC3339),
	})
}

func V1GetPaymentSessionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/status")
	id = strings.TrimSuffix(id, "/")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var resp V1PaymentSessionResponse
	var expiresAt time.Time
	err := database.DB.QueryRow(`
		SELECT id::text, reference::text, state, payment_method, amount_atomic, mint, expires_at
		FROM payment_sessions WHERE id = $1::uuid
	`, id).Scan(&resp.SessionID, &resp.Reference, &resp.State, &resp.PaymentMethod, &resp.AmountAtomic, &resp.Mint, &expiresAt)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	resp.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func V1GetWalletInstructions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/wallet-instructions")
	id = strings.TrimSuffix(id, "/")

	var reference, merchantWallet, state string
	var amount int64
	err := database.DB.QueryRow(`
		SELECT reference::text, merchant_wallet, amount_atomic, state
		FROM payment_sessions
		WHERE id = $1::uuid
	`, id).Scan(&reference, &merchantWallet, &amount, &state)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if state != "awaiting_payment" {
		http.Error(w, "session is not payable", http.StatusConflict)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"reference":      reference,
		"merchantWallet": merchantWallet,
		"amountAtomic":   amount,
		"mint":           usdcMintAddress(),
		"network":        networkFromMint(),
	})
}

func V1SubmitSignature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/submit-signature")
	id = strings.TrimSuffix(id, "/")
	var req V1SubmitSignatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Signature = strings.TrimSpace(req.Signature)
	if req.Signature == "" {
		http.Error(w, "signature is required", http.StatusBadRequest)
		return
	}

	var reference, merchantWallet, state string
	var amount int64
	err := database.DB.QueryRow(`
		SELECT reference::text, merchant_wallet, amount_atomic, state
		FROM payment_sessions
		WHERE id = $1::uuid
	`, id).Scan(&reference, &merchantWallet, &amount, &state)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if state != "awaiting_payment" && state != "verifying" {
		http.Error(w, "session cannot accept signature", http.StatusConflict)
		return
	}
	_, _ = database.DB.Exec(`UPDATE payment_sessions SET state='verifying', updated_at=NOW() WHERE id=$1::uuid`, id)

	redisKey := fmt.Sprintf("invoice:%s", reference)
	invoiceData, _ := database.RDB.HGetAll(database.Ctx, redisKey).Result()
	if len(invoiceData) == 0 {
		http.Error(w, "session expired", http.StatusNotFound)
		return
	}
	if err := watcher.VerifyTransactionForMerchant(reference, req.Signature, amount, merchantWallet); err != nil {
		_, _ = database.DB.Exec(`INSERT INTO payment_attempts(session_id, signature, status, error_reason) VALUES ($1::uuid,$2,'failed',$3)`, id, req.Signature, truncateErrorMessage(err.Error(), 500))
		_, _ = database.DB.Exec(`UPDATE payment_sessions SET state='failed', updated_at=NOW() WHERE id=$1::uuid`, id)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := finalizePaidCheckout(reference, req.Signature, invoiceData, amount); err != nil {
		_, _ = database.DB.Exec(`INSERT INTO payment_attempts(session_id, signature, status, error_reason) VALUES ($1::uuid,$2,'failed',$3)`, id, req.Signature, truncateErrorMessage(err.Error(), 500))
		_, _ = database.DB.Exec(`UPDATE payment_sessions SET state='failed', updated_at=NOW() WHERE id=$1::uuid`, id)
		http.Error(w, "failed to finalize payment", http.StatusInternalServerError)
		return
	}
	_, _ = database.DB.Exec(`INSERT INTO payment_attempts(session_id, signature, status) VALUES ($1::uuid,$2,'paid')`, id, req.Signature)
	_, _ = database.DB.Exec(`UPDATE payment_sessions SET state='paid', signature=$2, updated_at=NOW() WHERE id=$1::uuid`, id, req.Signature)
	_ = database.RDB.Del(database.Ctx, redisKey).Err()

	status, _ := getCheckoutStatusByReference(reference)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func V1CancelPaymentSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/cancel")
	id = strings.TrimSuffix(id, "/")
	var reference string
	var state string
	err := database.DB.QueryRow(`SELECT reference::text, state FROM payment_sessions WHERE id=$1::uuid`, id).Scan(&reference, &state)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if state == "paid" {
		http.Error(w, "paid session cannot be cancelled", http.StatusConflict)
		return
	}
	_, _ = database.DB.Exec(`UPDATE payment_sessions SET state='cancelled', updated_at=NOW() WHERE id=$1::uuid`, id)
	_, _ = database.DB.Exec(`DELETE FROM event_checkouts WHERE reference = $1 AND signature IS NULL AND status = 'pending_payment'`, reference)
	_ = database.RDB.Del(database.Ctx, fmt.Sprintf("invoice:%s", reference)).Err()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func V1VerifyPaymentSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/verify")
	id = strings.TrimSuffix(id, "/")
	var reference, signature string
	err := database.DB.QueryRow(`SELECT reference::text, COALESCE(signature,'') FROM payment_sessions WHERE id=$1::uuid`, id).Scan(&reference, &signature)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if signature == "" {
		http.Error(w, "signature is missing; submit-signature first", http.StatusBadRequest)
		return
	}
	resp, err := getCheckoutStatusByReference(reference)
	if err != nil {
		http.Error(w, "status unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func V1GetQrSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/qr")
	id = strings.TrimSuffix(id, "/")

	var reference, merchantWallet, state string
	var amount int64
	err := database.DB.QueryRow(`
		SELECT reference::text, merchant_wallet, amount_atomic, state
		FROM payment_sessions
		WHERE id = $1::uuid
	`, id).Scan(&reference, &merchantWallet, &amount, &state)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if state != "awaiting_payment" {
		http.Error(w, "session is not payable", http.StatusConflict)
		return
	}
	amountUsdc := float64(amount) / 1_000_000
	query := url.Values{}
	query.Set("amount", fmt.Sprintf("%.6f", amountUsdc))
	query.Set("spl-token", usdcMintAddress())
	query.Set("memo", reference)
	query.Set("label", "Payknot Event Deposit")
	query.Set("message", "Pay event deposit")
	solanaPayURL := fmt.Sprintf("solana:%s?%s", merchantWallet, query.Encode())
	qrURL := "https://api.qrserver.com/v1/create-qr-code/?size=320x320&data=" + url.QueryEscape(solanaPayURL)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"reference":    reference,
		"solanaPayUrl": solanaPayURL,
		"qrImageUrl":   qrURL,
		"network":      networkFromMint(),
	})
}

func V1DetectPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/payment-sessions/"))
	id = strings.TrimSuffix(id, "/detect")
	id = strings.TrimSuffix(id, "/")

	var reference, merchantWallet, state string
	var amount int64
	err := database.DB.QueryRow(`
		SELECT reference::text, merchant_wallet, amount_atomic, state
		FROM payment_sessions
		WHERE id = $1::uuid
	`, id).Scan(&reference, &merchantWallet, &amount, &state)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if state == "paid" {
		resp, _ := getCheckoutStatusByReference(reference)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	redisKey := fmt.Sprintf("invoice:%s", reference)
	invoiceData, _ := database.RDB.HGetAll(database.Ctx, redisKey).Result()
	if len(invoiceData) == 0 {
		http.Error(w, "session expired", http.StatusNotFound)
		return
	}

	signature, senderWallet, detectErr := watcher.DetectSignatureByReferenceForMerchant(reference, amount, merchantWallet)
	if detectErr != nil {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending", "message": detectErr.Error()})
		return
	}
	if strings.TrimSpace(invoiceData["wallet_address"]) == "" && senderWallet != "" {
		invoiceData["wallet_address"] = senderWallet
	}
	if err := finalizePaidCheckout(reference, signature, invoiceData, amount); err != nil {
		http.Error(w, "failed to finalize payment", http.StatusBadRequest)
		return
	}
	_, _ = database.DB.Exec(`INSERT INTO payment_attempts(session_id, signature, status) VALUES ($1::uuid,$2,'paid')`, id, signature)
	_, _ = database.DB.Exec(`UPDATE payment_sessions SET state='paid', signature=$2, updated_at=NOW() WHERE id=$1::uuid`, id, signature)
	_ = database.RDB.Del(database.Ctx, redisKey).Err()

	resp, _ := getCheckoutStatusByReference(reference)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func V1PaymentSessionsSubroutes(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if strings.HasSuffix(p, "/status") {
		V1GetPaymentSessionStatus(w, r)
		return
	}
	if strings.HasSuffix(p, "/wallet-instructions") {
		V1GetWalletInstructions(w, r)
		return
	}
	if strings.HasSuffix(p, "/submit-signature") {
		V1SubmitSignature(w, r)
		return
	}
	if strings.HasSuffix(p, "/cancel") {
		V1CancelPaymentSession(w, r)
		return
	}
	if strings.HasSuffix(p, "/verify") {
		V1VerifyPaymentSession(w, r)
		return
	}
	if strings.HasSuffix(p, "/qr") {
		V1GetQrSession(w, r)
		return
	}
	if strings.HasSuffix(p, "/detect") {
		V1DetectPayment(w, r)
		return
	}
	http.Error(w, "Not found", http.StatusNotFound)
}

func usdcMintAddress() string {
	if v := strings.TrimSpace(os.Getenv("USDC_MINT")); v != "" {
		return v
	}
	return "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"
}
