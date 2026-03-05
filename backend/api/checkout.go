package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"solana_paywall/backend/database"
	"solana_paywall/backend/enum"
	"solana_paywall/backend/watcher"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CheckoutEventResponse struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	EventImageURL  string `json:"eventImageUrl"`
	EventDate      string `json:"eventDate"`
	Location       string `json:"location"`
	OrganizerName  string `json:"organizerName"`
	MerchantWallet string `json:"merchantWallet"`
	AmountUSDC     string `json:"amountUsdc"`
	AmountRaw      int64  `json:"amountRaw"`
	Network        string `json:"network"`
}

type ValidateInviteRequest struct {
	Slug string `json:"slug"`
	Code string `json:"code"`
}

type InviteStatusResponse struct {
	Valid   bool                    `json:"valid"`
	Reason  string                  `json:"reason"`
	Receipt *CheckoutStatusResponse `json:"receipt,omitempty"`
}

type CreateCheckoutInvoiceRequest struct {
	Slug          string `json:"slug"`
	InviteCode    string `json:"inviteCode"`
	WalletAddress string `json:"walletAddress"`
}

type CreateCheckoutInvoiceResponse struct {
	Reference string `json:"reference"`
	AmountRaw int64  `json:"amountRaw"`
}

type ConfirmCheckoutRequest struct {
	Reference string `json:"reference"`
	Signature string `json:"signature"`
}

type RecheckCheckoutRequest struct {
	Reference string `json:"reference"`
	Signature string `json:"signature"`
}

type CancelCheckoutRequest struct {
	Reference string `json:"reference"`
}

type ManualVerifyRequest struct {
	Slug          string `json:"slug"`
	InviteCode    string `json:"inviteCode"`
	WalletAddress string `json:"walletAddress"`
	Signature     string `json:"signature"`
}

type CheckoutStatusResponse struct {
	Reference  string `json:"reference"`
	Status     string `json:"status"`
	Signature  string `json:"signature"`
	Network    string `json:"network"`
	SolscanURL string `json:"solscanUrl"`
	ApprovedBy string `json:"approvedBy"`
	ApprovedAt string `json:"approvedAt"`
	Reason     string `json:"reason"`
}

// GET /api/checkout/{slug}
func GetCheckoutBySlug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/checkout/")
	slug := strings.Trim(path, "/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	var resp CheckoutEventResponse
	var eventDate *time.Time
	err := database.DB.QueryRow(`
		SELECT slug, title, description, event_image_url, event_date, location, organizer_name, merchant_wallet, amount_usdc, network
		FROM events
		WHERE slug = $1 AND status = 'active'
	`, slug).Scan(&resp.Slug, &resp.Title, &resp.Description, &resp.EventImageURL, &eventDate, &resp.Location, &resp.OrganizerName, &resp.MerchantWallet, &resp.AmountRaw, &resp.Network)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if eventDate != nil {
		resp.EventDate = eventDate.Format(time.RFC3339)
	}
	resp.AmountUSDC = fmt.Sprintf("%.2f", float64(resp.AmountRaw)/1_000_000)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// POST /api/invite/validate
func ValidateInviteCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if req.Slug == "" || req.Code == "" {
		http.Error(w, "slug and code are required", http.StatusBadRequest)
		return
	}

	valid, reason, err := isInviteCodeUsable(req.Slug, req.Code)
	if err != nil {
		http.Error(w, "Failed to validate invite code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":  valid,
		"reason": reason,
	})
}

// POST /api/invite/status
// Returns usability + existing receipt (if any) for this invite code.
func GetInviteStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if req.Slug == "" || req.Code == "" {
		http.Error(w, "slug and code are required", http.StatusBadRequest)
		return
	}

	valid, reason, err := isInviteCodeUsable(req.Slug, req.Code)
	if err != nil {
		http.Error(w, "Failed to get invite status", http.StatusInternalServerError)
		return
	}

	receipt, _ := latestReceiptByInviteCode(req.Slug, req.Code)
	resp := InviteStatusResponse{
		Valid:   valid,
		Reason:  reason,
		Receipt: receipt,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// POST /api/checkout/invoice
func CreateCheckoutInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CreateCheckoutInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.InviteCode = strings.ToUpper(strings.TrimSpace(req.InviteCode))
	if req.Slug == "" || req.InviteCode == "" || !isValidWalletAddress(req.WalletAddress) {
		http.Error(w, "slug, inviteCode and walletAddress are required", http.StatusBadRequest)
		return
	}

	valid, reason, err := isInviteCodeUsable(req.Slug, req.InviteCode)
	if err != nil {
		http.Error(w, "Failed to validate invite code", http.StatusInternalServerError)
		return
	}
	if !valid {
		http.Error(w, reason, http.StatusBadRequest)
		return
	}

	var eventID, inviteCodeID int64
	var amount int64
	err = database.DB.QueryRow(`
		SELECT e.id, ic.id, e.amount_usdc
		FROM events e
		JOIN invite_codes ic ON ic.event_id = e.id
		WHERE e.slug = $1 AND ic.code = $2
	`, req.Slug, req.InviteCode).Scan(&eventID, &inviteCodeID, &amount)
	if err != nil {
		http.Error(w, "Event or invite code not found", http.StatusNotFound)
		return
	}

	reference := uuid.NewString()
	redisKey := fmt.Sprintf("invoice:%s", reference)
	invoiceData := map[string]interface{}{
		"wallet_address": req.WalletAddress,
		"amount":         amount,
		"event_id":       eventID,
		"invite_code_id": inviteCodeID,
		"invite_code":    req.InviteCode,
	}
	if err := database.RDB.HSet(database.Ctx, redisKey, invoiceData).Err(); err != nil {
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}
	if err := database.RDB.Expire(database.Ctx, redisKey, 20*time.Minute).Err(); err != nil {
		http.Error(w, "Failed to set invoice expiration", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO invoice (wallet_address, reference, amount, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (reference) DO NOTHING
	`, req.WalletAddress, reference, amount, enum.INVOICE_PENDING)
	if err != nil {
		http.Error(w, "Failed to persist invoice", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO event_checkouts (event_id, invite_code_id, wallet_address, reference, amount, status)
		VALUES ($1,$2,$3,$4,$5,'pending_payment')
	`, eventID, inviteCodeID, req.WalletAddress, reference, amount)
	if err != nil {
		http.Error(w, "Failed to create event checkout", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateCheckoutInvoiceResponse{
		Reference: reference,
		AmountRaw: amount,
	})
}

// POST /api/checkout/cancel
func CancelCheckoutInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CancelCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidReference(strings.TrimSpace(req.Reference)) {
		http.Error(w, "invalid reference", http.StatusBadRequest)
		return
	}

	_, _ = database.DB.Exec(`DELETE FROM event_checkouts WHERE reference = $1 AND signature IS NULL AND status = 'pending_payment'`, req.Reference)
	_, _ = database.DB.Exec(`DELETE FROM invoice WHERE reference = $1 AND signature IS NULL AND status = $2`, req.Reference, enum.INVOICE_PENDING)
	_ = database.RDB.Del(database.Ctx, fmt.Sprintf("invoice:%s", req.Reference)).Err()

	w.WriteHeader(http.StatusOK)
}

// POST /api/checkout/confirm
func ConfirmCheckoutPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConfirmCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidReference(req.Reference) || strings.TrimSpace(req.Signature) == "" {
		http.Error(w, "reference and signature are required", http.StatusBadRequest)
		return
	}

	redisKey := fmt.Sprintf("invoice:%s", req.Reference)
	invoiceData, err := database.RDB.HGetAll(database.Ctx, redisKey).Result()
	if err != nil || len(invoiceData) == 0 {
		http.Error(w, "Invoice not found or expired", http.StatusNotFound)
		return
	}

	amount, err := strconv.ParseInt(invoiceData["amount"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid invoice amount", http.StatusInternalServerError)
		return
	}

	var merchantWallet string
	err = database.DB.QueryRow(`
		SELECT e.merchant_wallet
		FROM event_checkouts ec
		JOIN events e ON e.id = ec.event_id
		WHERE ec.reference = $1
	`, req.Reference).Scan(&merchantWallet)
	if err != nil {
		http.Error(w, "Failed to resolve merchant wallet for checkout", http.StatusInternalServerError)
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE invoice SET signature = $1, status = $2
		WHERE reference = $3
	`, req.Signature, enum.INVOICE_PENDING, req.Reference)
	_, _ = database.DB.Exec(`
		UPDATE event_checkouts SET signature = $1
		WHERE reference = $2
	`, req.Signature, req.Reference)

	if err := watcher.VerifyTransactionForMerchant(req.Reference, req.Signature, amount, merchantWallet); err != nil {
		_, _ = database.DB.Exec(`UPDATE invoice SET status = $1, err_reason = $2 WHERE reference = $3`, enum.INVOICE_ERROR, truncateErrorMessage(err.Error(), 500), req.Reference)
		_, _ = database.DB.Exec(`UPDATE event_checkouts SET status = 'failed' WHERE reference = $1`, req.Reference)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE invoice
		SET status = $1, signature = $2, err_reason = NULL
		WHERE reference = $3
	`, enum.INVOICE_PAID, req.Signature, req.Reference)

	_, err = database.DB.Exec(`
		UPDATE event_checkouts
		SET status = 'paid', paid_at = NOW(), signature = $1
		WHERE reference = $2
	`, req.Signature, req.Reference)
	if err != nil {
		http.Error(w, "Failed to update checkout", http.StatusInternalServerError)
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE invite_codes ic
		SET used_count = used_count + 1,
			status = CASE WHEN used_count + 1 >= max_uses THEN 'used' ELSE status END
		WHERE ic.id = (
			SELECT invite_code_id FROM event_checkouts WHERE reference = $1
		)
	`, req.Reference)

	_ = database.RDB.Del(database.Ctx, redisKey).Err()

	status, _ := getCheckoutStatusByReference(req.Reference)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// POST /api/checkout/recheck
func RecheckCheckoutPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecheckCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidReference(strings.TrimSpace(req.Reference)) {
		http.Error(w, "invalid reference", http.StatusBadRequest)
		return
	}

	var status string
	var amount int64
	var signature string
	var merchantWallet string
	var inviteCodeID int64
	err := database.DB.QueryRow(`
		SELECT ec.status, ec.amount, COALESCE(ec.signature, ''), e.merchant_wallet, ec.invite_code_id
		FROM event_checkouts ec
		JOIN events e ON e.id = ec.event_id
		WHERE ec.reference = $1
	`, req.Reference).Scan(&status, &amount, &signature, &merchantWallet, &inviteCodeID)
	if err != nil {
		http.Error(w, "Checkout not found", http.StatusNotFound)
		return
	}

	if strings.TrimSpace(req.Signature) != "" {
		signature = strings.TrimSpace(req.Signature)
		_, _ = database.DB.Exec(`UPDATE event_checkouts SET signature = $1 WHERE reference = $2`, signature, req.Reference)
		_, _ = database.DB.Exec(`UPDATE invoice SET signature = $1 WHERE reference = $2`, signature, req.Reference)
	}

	if status == "approved" || status == "paid" {
		resp, _ := getCheckoutStatusByReference(req.Reference)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if signature == "" {
		http.Error(w, "Missing signature. Submit payment signature to recheck.", http.StatusBadRequest)
		return
	}

	if err := watcher.VerifyTransactionForMerchant(req.Reference, signature, amount, merchantWallet); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, _ = database.DB.Exec(`UPDATE invoice SET status = $1, signature = $2, err_reason = NULL WHERE reference = $3`, enum.INVOICE_PAID, signature, req.Reference)
	_, _ = database.DB.Exec(`UPDATE event_checkouts SET status = 'paid', paid_at = NOW(), signature = $1 WHERE reference = $2`, signature, req.Reference)
	_, _ = database.DB.Exec(`
		UPDATE invite_codes
		SET used_count = used_count + 1,
			status = CASE WHEN used_count + 1 >= max_uses THEN 'used' ELSE status END
		WHERE id = $1
	`, inviteCodeID)

	resp, _ := getCheckoutStatusByReference(req.Reference)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// POST /api/checkout/manual-verify
func ManualVerifyCheckoutPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ManualVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.InviteCode = strings.ToUpper(strings.TrimSpace(req.InviteCode))
	req.WalletAddress = strings.TrimSpace(req.WalletAddress)
	req.Signature = strings.TrimSpace(req.Signature)
	var errMessage = ""
	if req.InviteCode == "" {
		errMessage += "Invite Code, "
	}
	if req.Signature == "" {
		errMessage += "Signature, "
	}
	if !isValidWalletAddress(req.WalletAddress) {
		errMessage += "Wallet Address "
	}
	if errMessage != "" {
		errMessage += "required"
		http.Error(w, errMessage, http.StatusBadRequest)
		return
	}

	valid, reason, err := isInviteCodeUsable(req.Slug, req.InviteCode)
	if err != nil {
		http.Error(w, "Failed to validate invite code", http.StatusInternalServerError)
		return
	}
	if !valid {
		http.Error(w, reason, http.StatusBadRequest)
		return
	}

	var eventID, inviteCodeID int64
	var amount int64
	var merchantWallet, network string
	err = database.DB.QueryRow(`
		SELECT e.id, ic.id, e.amount_usdc, e.merchant_wallet, e.network
		FROM events e
		JOIN invite_codes ic ON ic.event_id = e.id
		WHERE e.slug = $1 AND ic.code = $2
	`, req.Slug, req.InviteCode).Scan(&eventID, &inviteCodeID, &amount, &merchantWallet, &network)
	if err != nil {
		http.Error(w, "Event or invite code not found", http.StatusNotFound)
		return
	}

	if err := watcher.VerifyDirectTransferForMerchant(req.Signature, amount, merchantWallet, req.WalletAddress); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reference := uuid.NewString()
	_, _ = database.DB.Exec(`
		INSERT INTO invoice (wallet_address, reference, amount, signature, status)
		VALUES ($1, $2, $3, $4, $5)
	`, req.WalletAddress, reference, amount, req.Signature, enum.INVOICE_PAID)
	_, _ = database.DB.Exec(`
		INSERT INTO event_checkouts (event_id, invite_code_id, wallet_address, reference, signature, amount, status, paid_at)
		VALUES ($1,$2,$3,$4,$5,$6,'paid',NOW())
	`, eventID, inviteCodeID, req.WalletAddress, reference, req.Signature, amount)
	_, _ = database.DB.Exec(`
		UPDATE invite_codes
		SET used_count = used_count + 1,
			status = CASE WHEN used_count + 1 >= max_uses THEN 'used' ELSE status END
		WHERE id = $1
	`, inviteCodeID)

	resp := CheckoutStatusResponse{
		Reference:  reference,
		Status:     "paid",
		Signature:  req.Signature,
		Network:    network,
		SolscanURL: solscanURLForNetwork(req.Signature, network),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /api/checkout/status?reference=...
func GetCheckoutStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}
	reference := strings.TrimSpace(r.URL.Query().Get("reference"))
	if !isValidReference(reference) {
		http.Error(w, "invalid reference", http.StatusBadRequest)
		return
	}

	resp, err := getCheckoutStatusByReference(reference)
	if err != nil {
		http.Error(w, "Checkout not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func getCheckoutStatusByReference(reference string) (CheckoutStatusResponse, error) {
	var resp CheckoutStatusResponse
	var approvedAt *time.Time
	err := database.DB.QueryRow(`
		SELECT ec.reference, ec.status, COALESCE(ec.signature, ''), COALESCE(ec.approved_by, ''), ec.approved_at, e.network, COALESCE(ec.notes, '')
		FROM event_checkouts ec
		JOIN events e ON e.id = ec.event_id
		WHERE ec.reference = $1
	`, reference).Scan(&resp.Reference, &resp.Status, &resp.Signature, &resp.ApprovedBy, &approvedAt, &resp.Network, &resp.Reason)
	if err != nil {
		return resp, err
	}
	if approvedAt != nil {
		resp.ApprovedAt = approvedAt.Format(time.RFC3339)
	}
	if resp.Signature != "" {
		resp.SolscanURL = solscanURLForNetwork(resp.Signature, resp.Network)
	}
	return resp, nil
}

func latestReceiptByInviteCode(slug string, code string) (*CheckoutStatusResponse, error) {
	var reference, status, signature, approvedBy, network, reason string
	var approvedAt *time.Time
	err := database.DB.QueryRow(`
		SELECT ec.reference, ec.status, COALESCE(ec.signature, ''), COALESCE(ec.approved_by, ''), ec.approved_at, e.network, COALESCE(ec.notes, '')
		FROM event_checkouts ec
		JOIN invite_codes ic ON ic.id = ec.invite_code_id
		JOIN events e ON e.id = ec.event_id
		WHERE e.slug = $1 AND ic.code = $2
		ORDER BY ec.created_at DESC
		LIMIT 1
	`, slug, code).Scan(&reference, &status, &signature, &approvedBy, &approvedAt, &network, &reason)
	if err != nil {
		return nil, err
	}

	if status != "paid" && status != "approved" && status != "rejected" {
		return nil, nil
	}

	resp := &CheckoutStatusResponse{
		Reference:  reference,
		Status:     status,
		Signature:  signature,
		Network:    network,
		ApprovedBy: approvedBy,
		Reason:     reason,
	}
	if approvedAt != nil {
		resp.ApprovedAt = approvedAt.Format(time.RFC3339)
	}
	if signature != "" {
		resp.SolscanURL = solscanURLForNetwork(signature, network)
	}
	return resp, nil
}

func isInviteCodeUsable(slug string, code string) (bool, string, error) {
	var usedCount, maxUses int
	var status string
	var expiresAt *time.Time
	err := database.DB.QueryRow(`
		SELECT ic.used_count, ic.max_uses, ic.status, ic.expires_at
		FROM invite_codes ic
		JOIN events e ON e.id = ic.event_id
		WHERE e.slug = $1 AND ic.code = $2 AND e.status = 'active'
	`, slug, code).Scan(&usedCount, &maxUses, &status, &expiresAt)
	if err != nil {
		return false, "Invite code not found", nil
	}
	if status != "active" && status != "used" {
		return false, "Invite code is inactive", nil
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return false, "Invite code expired", nil
	}
	if usedCount >= maxUses {
		return false, "Invite code already used", nil
	}
	return true, "ok", nil
}

func truncateErrorMessage(msg string, max int) string {
	if len(msg) <= max {
		return msg
	}
	return msg[:max]
}

func solscanURLForNetwork(signature string, network string) string {
	if network == "mainnet" || network == "mainnet-beta" {
		return fmt.Sprintf("https://solscan.io/tx/%s", signature)
	}
	return fmt.Sprintf("https://solscan.io/tx/%s?cluster=devnet", signature)
}
