package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"solana_paywall/backend/database"
	"solana_paywall/backend/enum"
	"solana_paywall/backend/watcher"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CheckoutEventResponse struct {
	Slug                string             `json:"slug"`
	Title               string             `json:"title"`
	Description         string             `json:"description"`
	EventImageURL       string             `json:"eventImageUrl"`
	EventDate           string             `json:"eventDate"`
	Location            string             `json:"location"`
	OrganizerName       string             `json:"organizerName"`
	MerchantWallet      string             `json:"merchantWallet"`
	AmountUSDC          string             `json:"amountUsdc"`
	AmountRaw           int64              `json:"amountRaw"`
	Network             string             `json:"network"`
	ParticipantForm     []ParticipantField `json:"participantForm"`
	PaymentMethodWallet bool               `json:"paymentMethodWallet"`
	PaymentMethodQR     bool               `json:"paymentMethodQr"`
}

type CreateCheckoutInvoiceRequest struct {
	Slug            string                 `json:"slug"`
	WalletAddress   string                 `json:"walletAddress"`
	ParticipantData map[string]interface{} `json:"participantData"`
	PaymentMethod   string                 `json:"paymentMethod"`
}

type CreateCheckoutInvoiceResponse struct {
	Reference string `json:"reference"`
	AmountRaw int64  `json:"amountRaw"`
	Network   string `json:"network"`
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
	Slug          string                 `json:"slug"`
	WalletAddress string                 `json:"walletAddress"`
	Signature     string                 `json:"signature"`
	Participant   map[string]interface{} `json:"participantData"`
}

type ParticipantStatusRequest struct {
	Slug            string                 `json:"slug"`
	ParticipantData map[string]interface{} `json:"participantData"`
}

type CheckoutStatusResponse struct {
	Reference       string                 `json:"reference"`
	Status          string                 `json:"status"`
	Signature       string                 `json:"signature"`
	Network         string                 `json:"network"`
	SolscanURL      string                 `json:"solscanUrl"`
	PaymentMethod   string                 `json:"paymentMethod"`
	ParticipantData map[string]interface{} `json:"participantData,omitempty"`
}

type DetectCheckoutRequest struct {
	Reference string `json:"reference"`
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
	var formJSON []byte
	var methodsJSON []byte
	err := database.DB.QueryRow(`
		SELECT slug, title, description, event_image_url, event_date, location, organizer_name, merchant_wallet, amount_usdc,
		       participant_form_schema, payment_methods
		FROM events
		WHERE slug = $1 AND status = 'active' AND checkout_expires_at > NOW()
	`, slug).Scan(&resp.Slug, &resp.Title, &resp.Description, &resp.EventImageURL, &eventDate, &resp.Location, &resp.OrganizerName, &resp.MerchantWallet, &resp.AmountRaw, &formJSON, &methodsJSON)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if eventDate != nil {
		resp.EventDate = eventDate.Format(time.RFC3339)
	}
	resp.AmountUSDC = fmt.Sprintf("%.2f", float64(resp.AmountRaw)/1_000_000)
	resp.Network = networkFromMint()
	resp.ParticipantForm = decodeParticipantFields(formJSON)

	methods := map[string]bool{"wallet": true, "qr": true}
	_ = json.Unmarshal(methodsJSON, &methods)
	resp.PaymentMethodWallet = methods["wallet"]
	resp.PaymentMethodQR = methods["qr"]

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
	req.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	req.WalletAddress = strings.TrimSpace(req.WalletAddress)
	if req.Slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}

	var eventID int64
	var amount int64
	var merchantWallet string
	var formJSON []byte
	var methodsJSON []byte
	err := database.DB.QueryRow(`
		SELECT id, amount_usdc, merchant_wallet, participant_form_schema, payment_methods
		FROM events
		WHERE slug = $1 AND status = 'active'
	`, req.Slug).Scan(&eventID, &amount, &merchantWallet, &formJSON, &methodsJSON)
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
	switch req.PaymentMethod {
	case "wallet":
		if !methods["wallet"] {
			http.Error(w, "wallet method disabled", http.StatusBadRequest)
			return
		}
		if !isValidWalletAddress(req.WalletAddress) {
			http.Error(w, "walletAddress is required for wallet method", http.StatusBadRequest)
			return
		}
	case "qr":
		if !methods["qr"] {
			http.Error(w, "qr method disabled", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "invalid payment method", http.StatusBadRequest)
		return
	}

	reference := uuid.NewString()
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
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}
	if err := database.RDB.Expire(database.Ctx, redisKey, 20*time.Minute).Err(); err != nil {
		http.Error(w, "Failed to set invoice expiration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateCheckoutInvoiceResponse{
		Reference: reference,
		AmountRaw: amount,
		Network:   networkFromMint(),
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
	merchantWallet := strings.TrimSpace(invoiceData["merchant_wallet"])
	if merchantWallet == "" {
		http.Error(w, "merchant wallet not found", http.StatusInternalServerError)
		return
	}

	if err := watcher.VerifyTransactionForMerchant(req.Reference, req.Signature, amount, merchantWallet); err != nil {
		_, _ = database.DB.Exec(`
			INSERT INTO invoice (wallet_address, reference, amount, signature, status, err_reason)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (reference) DO UPDATE
			SET status = EXCLUDED.status,
			    signature = EXCLUDED.signature,
			    err_reason = EXCLUDED.err_reason
		`, strings.TrimSpace(invoiceData["wallet_address"]), req.Reference, amount, req.Signature, enum.INVOICE_ERROR, truncateErrorMessage(err.Error(), 500))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := finalizePaidCheckout(req.Reference, req.Signature, invoiceData, amount); err != nil {
		http.Error(w, "Failed to update checkout", http.StatusInternalServerError)
		return
	}

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
	err := database.DB.QueryRow(`
		SELECT ec.status
		FROM event_checkouts ec
		WHERE ec.reference = $1
	`, req.Reference).Scan(&status)
	if err == nil && status == "paid" {
		resp, _ := getCheckoutStatusByReference(req.Reference)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	redisKey := fmt.Sprintf("invoice:%s", req.Reference)
	invoiceData, _ := database.RDB.HGetAll(database.Ctx, redisKey).Result()
	if len(invoiceData) == 0 {
		http.Error(w, "Checkout session expired. Please start a new payment session.", http.StatusNotFound)
		return
	}

	amount, parseErr := strconv.ParseInt(invoiceData["amount"], 10, 64)
	if parseErr != nil {
		http.Error(w, "Invalid session amount", http.StatusBadRequest)
		return
	}
	merchantWallet := strings.TrimSpace(invoiceData["merchant_wallet"])
	if merchantWallet == "" {
		http.Error(w, "merchant wallet not found", http.StatusBadRequest)
		return
	}

	signature := strings.TrimSpace(req.Signature)
	if strings.TrimSpace(req.Signature) != "" {
		signature = strings.TrimSpace(req.Signature)
	}

	if signature == "" {
		http.Error(w, "Missing signature. Submit payment signature to recheck.", http.StatusBadRequest)
		return
	}
	if err := watcher.VerifyTransactionForMerchant(req.Reference, signature, amount, merchantWallet); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := finalizePaidCheckout(req.Reference, signature, invoiceData, amount); err != nil {
		http.Error(w, "Failed to finalize checkout", http.StatusInternalServerError)
		return
	}
	_ = database.RDB.Del(database.Ctx, redisKey).Err()

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
	req.WalletAddress = strings.TrimSpace(req.WalletAddress)
	req.Signature = strings.TrimSpace(req.Signature)
	if req.Slug == "" || req.Signature == "" || !isValidWalletAddress(req.WalletAddress) {
		http.Error(w, "slug, walletAddress and signature are required", http.StatusBadRequest)
		return
	}

	var eventID int64
	var amount int64
	var merchantWallet string
	err := database.DB.QueryRow(`
		SELECT id, amount_usdc, merchant_wallet
		FROM events
		WHERE slug = $1 AND status = 'active'
	`, req.Slug).Scan(&eventID, &amount, &merchantWallet)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	if err := watcher.VerifyDirectTransferForMerchant(req.Signature, amount, merchantWallet, req.WalletAddress); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reference := uuid.NewString()
	participantJSON, _ := json.Marshal(req.Participant)
	_, _ = database.DB.Exec(`
		INSERT INTO invoice (wallet_address, reference, amount, signature, status)
		VALUES ($1, $2, $3, $4, $5)
	`, req.WalletAddress, reference, amount, req.Signature, enum.INVOICE_PAID)
	_, _ = database.DB.Exec(`
		INSERT INTO event_checkouts (event_id, wallet_address, reference, signature, amount, status, paid_at, participant_data)
		VALUES ($1,$2,$3,$4,$5,'paid',NOW(),$6::jsonb)
	`, eventID, req.WalletAddress, reference, req.Signature, amount, string(participantJSON))

	resp := CheckoutStatusResponse{
		Reference:       reference,
		Status:          "paid",
		Signature:       req.Signature,
		Network:         networkFromMint(),
		SolscanURL:      solscanURLForNetwork(req.Signature, networkFromMint()),
		ParticipantData: req.Participant,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// POST /api/checkout/participant-status
func GetParticipantStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ParticipantStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if len(req.ParticipantData) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	payload, _ := json.Marshal(req.ParticipantData)
	var reference string
	err := database.DB.QueryRow(`
		SELECT ec.reference
		FROM event_checkouts ec
		JOIN events e ON e.id = ec.event_id
		WHERE e.slug = $1
		  AND ec.status = 'paid'
		  AND ec.participant_data @> $2::jsonb
		ORDER BY ec.paid_at DESC NULLS LAST, ec.created_at DESC
		LIMIT 1
	`, req.Slug, string(payload)).Scan(&reference)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	resp, err := getCheckoutStatusByReference(reference)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// POST /api/checkout/detect
func DetectCheckoutPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DetectCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Reference = strings.TrimSpace(req.Reference)
	if !isValidReference(req.Reference) {
		http.Error(w, "invalid reference", http.StatusBadRequest)
		return
	}

	existing, err := getCheckoutStatusByReference(req.Reference)
	if err == nil && existing.Status == "paid" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	redisKey := fmt.Sprintf("invoice:%s", req.Reference)
	invoiceData, _ := database.RDB.HGetAll(database.Ctx, redisKey).Result()
	if len(invoiceData) == 0 {
		http.Error(w, "Checkout session expired. Please start a new payment session.", http.StatusNotFound)
		return
	}

	amount, parseErr := strconv.ParseInt(invoiceData["amount"], 10, 64)
	if parseErr != nil {
		http.Error(w, "Invalid session amount", http.StatusBadRequest)
		return
	}
	merchantWallet := strings.TrimSpace(invoiceData["merchant_wallet"])
	if merchantWallet == "" {
		http.Error(w, "merchant wallet not found", http.StatusBadRequest)
		return
	}

	signature, senderWallet, detectErr := watcher.DetectSignatureByReferenceForMerchant(req.Reference, amount, merchantWallet)
	if detectErr != nil {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "pending",
			"message": detectErr.Error(),
		})
		return
	}
	if strings.TrimSpace(invoiceData["wallet_address"]) == "" && senderWallet != "" {
		invoiceData["wallet_address"] = senderWallet
	}
	if err := finalizePaidCheckout(req.Reference, signature, invoiceData, amount); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = database.RDB.Del(database.Ctx, redisKey).Err()
	resp, _ := getCheckoutStatusByReference(req.Reference)
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
	var participantDataJSON []byte
	err := database.DB.QueryRow(`
		SELECT ec.reference, ec.status, COALESCE(ec.signature, ''), ec.participant_data, COALESCE(ec.payment_method, 'wallet')
		FROM event_checkouts ec
		WHERE ec.reference = $1
	`, reference).Scan(&resp.Reference, &resp.Status, &resp.Signature, &participantDataJSON, &resp.PaymentMethod)
	if err != nil {
		return resp, err
	}
	resp.Network = networkFromMint()
	if resp.Signature != "" {
		resp.SolscanURL = solscanURLForNetwork(resp.Signature, resp.Network)
	}
	if len(participantDataJSON) > 0 {
		_ = json.Unmarshal(participantDataJSON, &resp.ParticipantData)
	}
	return resp, nil
}

func finalizePaidCheckout(reference string, signature string, invoiceData map[string]string, amount int64) error {
	walletAddress := strings.TrimSpace(invoiceData["wallet_address"])
	if walletAddress == "" {
		walletAddress = "unknown_wallet"
	}
	eventID, err := strconv.ParseInt(invoiceData["event_id"], 10, 64)
	if err != nil {
		return err
	}
	participantData := strings.TrimSpace(invoiceData["participant_data"])
	if participantData == "" {
		participantData = "{}"
	}
	paymentMethod := strings.TrimSpace(invoiceData["payment_method"])
	if paymentMethod == "" {
		paymentMethod = "wallet"
	}

	// Bound rule: 1 email = 1 paid transaction per event.
	var participantMap map[string]interface{}
	_ = json.Unmarshal([]byte(participantData), &participantMap)
	email := strings.ToLower(strings.TrimSpace(fmt.Sprint(participantMap["email"])))
	if email != "" && email != "<nil>" {
		var exists bool
		err := database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1
				FROM event_checkouts
				WHERE event_id = $1
				  AND status = 'paid'
				  AND reference <> $2
				  AND LOWER(COALESCE(participant_data->>'email', '')) = $3
			)
		`, eventID, reference, email).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("this email already completed a transaction for this event")
		}
	}

	_, err = database.DB.Exec(`
		INSERT INTO invoice (wallet_address, reference, amount, signature, status, err_reason)
		VALUES ($1, $2, $3, $4, $5, NULL)
		ON CONFLICT (reference) DO UPDATE
		SET status = EXCLUDED.status,
		    signature = EXCLUDED.signature,
		    err_reason = NULL
	`, walletAddress, reference, amount, signature, enum.INVOICE_PAID)
	if err != nil {
		return err
	}

	_, err = database.DB.Exec(`
		INSERT INTO event_checkouts (event_id, wallet_address, reference, signature, amount, status, paid_at, participant_data, payment_method)
		VALUES ($1, $2, $3, $4, $5, 'paid', NOW(), $6::jsonb, $7)
		ON CONFLICT (reference) DO UPDATE
		SET status = 'paid',
		    signature = EXCLUDED.signature,
		    paid_at = NOW(),
		    participant_data = EXCLUDED.participant_data,
		    payment_method = EXCLUDED.payment_method
	`, eventID, walletAddress, reference, signature, amount, participantData, paymentMethod)
	return err
}

func ensureParticipantEmailAvailable(eventID int64, participantData map[string]interface{}) error {
	email := strings.ToLower(strings.TrimSpace(fmt.Sprint(participantData["email"])))
	if email == "" || email == "<nil>" {
		return nil
	}
	var exists bool
	err := database.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM event_checkouts
			WHERE event_id = $1
			  AND status = 'paid'
			  AND LOWER(COALESCE(participant_data->>'email', '')) = $2
		)
	`, eventID, email).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("this email already completed a transaction for this event")
	}
	return nil
}

func networkFromMint() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SOLANA_CLUSTER")), "mainnet") {
		return "mainnet"
	}
	mint := strings.TrimSpace(os.Getenv("USDC_MINT"))
	switch mint {
	case "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU":
		return "devnet"
	default:
		return "mainnet"
	}
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
