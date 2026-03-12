package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"
	"strconv"
	"strings"
	"time"
)

type PaymentMethods struct {
	Wallet bool `json:"wallet"`
	QR     bool `json:"qr"`
}

type CreateEventRequest struct {
	Title                 string             `json:"title"`
	Description           string             `json:"description"`
	EventImageURL         string             `json:"eventImageUrl"`
	EventDate             string             `json:"eventDate"`
	CheckoutExpiresAt     string             `json:"checkoutExpiresAt"`
	Location              string             `json:"location"`
	OrganizerName         string             `json:"organizerName"`
	MerchantWallet        string             `json:"merchantWallet"`
	AmountUSDC            int64              `json:"amountUsdc"`
	EventSource           string             `json:"eventSource"`
	SourceURL             string             `json:"sourceUrl"`
	ParticipantFormSchema []ParticipantField `json:"participantFormSchema"`
	PaymentMethods        PaymentMethods     `json:"paymentMethods"`
}

type UpdateEventRequest = CreateEventRequest

type CreateEventResponse struct {
	EventID               int64              `json:"eventId"`
	Slug                  string             `json:"slug"`
	CheckoutURL           string             `json:"checkoutUrl"`
	EventImageURL         string             `json:"eventImageUrl"`
	CheckoutExpiresAt     string             `json:"checkoutExpiresAt"`
	AmountUSDC            string             `json:"amountUsdc"`
	MerchantWallet        string             `json:"merchantWallet"`
	EventSource           string             `json:"eventSource"`
	SourceURL             string             `json:"sourceUrl"`
	ParticipantFormSchema []ParticipantField `json:"participantFormSchema"`
	PaymentMethods        PaymentMethods     `json:"paymentMethods"`
}

type EventSummary struct {
	EventID               int64              `json:"eventId"`
	Slug                  string             `json:"slug"`
	Title                 string             `json:"title"`
	Description           string             `json:"description"`
	EventImageURL         string             `json:"eventImageUrl"`
	EventDate             string             `json:"eventDate"`
	CheckoutExpiresAt     string             `json:"checkoutExpiresAt"`
	Location              string             `json:"location"`
	OrganizerName         string             `json:"organizerName"`
	MerchantWallet        string             `json:"merchantWallet"`
	AmountUSDC            string             `json:"amountUsdc"`
	EventSource           string             `json:"eventSource"`
	SourceURL             string             `json:"sourceUrl"`
	ParticipantFormSchema []ParticipantField `json:"participantFormSchema"`
	PaymentMethods        PaymentMethods     `json:"paymentMethods"`
	CreatedAt             string             `json:"createdAt"`
}

type EventCheckoutRow struct {
	ID              int64                  `json:"id"`
	WalletAddress   string                 `json:"walletAddress"`
	Reference       string                 `json:"reference"`
	Signature       string                 `json:"signature"`
	Status          string                 `json:"status"`
	PaymentMethod   string                 `json:"paymentMethod"`
	CreatedAt       time.Time              `json:"createdAt"`
	PaidAt          *time.Time             `json:"paidAt"`
	ParticipantData map[string]interface{} `json:"participantData"`
}

func EventsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		CreateEvent(w, r)
	case http.MethodGet:
		ListEvents(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func ListEvents(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, slug, title, description, event_image_url, event_date, checkout_expires_at, location, organizer_name, merchant_wallet, amount_usdc,
		       event_source, source_url, participant_form_schema, payment_methods, created_at
		FROM events
		WHERE owner_email = $1
		ORDER BY created_at DESC
	`, claims.Email)
	if err != nil {
		http.Error(w, "Failed to list events", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]EventSummary, 0)
	for rows.Next() {
		var s EventSummary
		var eventDate *time.Time
		var checkoutExpiresAt time.Time
		var amountRaw int64
		var created time.Time
		var formJSON []byte
		var methodsJSON []byte
		if scanErr := rows.Scan(&s.EventID, &s.Slug, &s.Title, &s.Description, &s.EventImageURL, &eventDate, &checkoutExpiresAt, &s.Location, &s.OrganizerName, &s.MerchantWallet, &amountRaw, &s.EventSource, &s.SourceURL, &formJSON, &methodsJSON, &created); scanErr != nil {
			continue
		}
		if eventDate != nil {
			s.EventDate = eventDate.Format(time.RFC3339)
		}
		s.CheckoutExpiresAt = checkoutExpiresAt.Format(time.RFC3339)
		s.AmountUSDC = fmt.Sprintf("%.2f", float64(amountRaw)/1_000_000)
		s.CreatedAt = created.Format(time.RFC3339)
		s.ParticipantFormSchema = decodeParticipantFields(formJSON)
		s.PaymentMethods = PaymentMethods{Wallet: true, QR: true}
		_ = json.Unmarshal(methodsJSON, &s.PaymentMethods)
		out = append(out, s)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"events": out})
}

func CreateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := normalizeAndValidateEventRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	eventTime := time.Time{}
	if strings.TrimSpace(req.EventDate) != "" {
		parsed, err := time.Parse(time.RFC3339, req.EventDate)
		if err != nil {
			http.Error(w, "eventDate must be RFC3339 format", http.StatusBadRequest)
			return
		}
		eventTime = parsed
	}
	checkoutExpiresAt, err := time.Parse(time.RFC3339, req.CheckoutExpiresAt)
	if err != nil {
		http.Error(w, "checkoutExpiresAt must be RFC3339 format", http.StatusBadRequest)
		return
	}

	slug := buildEventSlug(req.Title)
	if slug == "" {
		http.Error(w, "invalid title", http.StatusBadRequest)
		return
	}
	slug = ensureUniqueEventSlug(slug)

	formJSON, _ := json.Marshal(req.ParticipantFormSchema)
	methodsJSON, _ := json.Marshal(req.PaymentMethods)

	var eventID int64
	err = database.DB.QueryRow(`
		INSERT INTO events (
			slug, title, description, event_image_url, event_date, location, organizer_name, merchant_wallet, amount_usdc, owner_email,
			event_source, source_url, participant_form_schema, payment_methods, checkout_expires_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb,$15)
		RETURNING id
	`, slug, req.Title, req.Description, req.EventImageURL, nullTime(eventTime), req.Location, req.OrganizerName, req.MerchantWallet, req.AmountUSDC*1_000_000, claims.Email, req.EventSource, req.SourceURL, string(formJSON), string(methodsJSON), checkoutExpiresAt).Scan(&eventID)
	if err != nil {
		http.Error(w, "Failed to create event", http.StatusInternalServerError)
		return
	}

	resp := CreateEventResponse{
		EventID:               eventID,
		Slug:                  slug,
		CheckoutURL:           "/checkout/" + slug,
		EventImageURL:         req.EventImageURL,
		CheckoutExpiresAt:     req.CheckoutExpiresAt,
		AmountUSDC:            fmt.Sprintf("%.2f", float64(req.AmountUSDC)),
		MerchantWallet:        req.MerchantWallet,
		EventSource:           req.EventSource,
		SourceURL:             req.SourceURL,
		ParticipantFormSchema: req.ParticipantFormSchema,
		PaymentMethods:        req.PaymentMethods,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func updateEvent(w http.ResponseWriter, r *http.Request, eventID int64, ownerEmail string) {
	var req UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := normalizeAndValidateEventRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	locked, err := hasPaidDeposits(eventID)
	if err != nil {
		http.Error(w, "Failed to validate event state", http.StatusInternalServerError)
		return
	}
	if locked {
		http.Error(w, "event cannot be edited after at least one successful deposit", http.StatusConflict)
		return
	}

	eventTime := time.Time{}
	if strings.TrimSpace(req.EventDate) != "" {
		parsed, err := time.Parse(time.RFC3339, req.EventDate)
		if err != nil {
			http.Error(w, "eventDate must be RFC3339 format", http.StatusBadRequest)
			return
		}
		eventTime = parsed
	}
	checkoutExpiresAt, err := time.Parse(time.RFC3339, req.CheckoutExpiresAt)
	if err != nil {
		http.Error(w, "checkoutExpiresAt must be RFC3339 format", http.StatusBadRequest)
		return
	}

	formJSON, _ := json.Marshal(req.ParticipantFormSchema)
	methodsJSON, _ := json.Marshal(req.PaymentMethods)
	res, err := database.DB.Exec(`
		UPDATE events
		SET title = $1,
			description = $2,
			event_image_url = $3,
			event_date = $4,
			location = $5,
			organizer_name = $6,
			merchant_wallet = $7,
			amount_usdc = $8,
			event_source = $9,
			source_url = $10,
			participant_form_schema = $11::jsonb,
			payment_methods = $12::jsonb,
			checkout_expires_at = $13
		WHERE id = $14 AND owner_email = $15
	`, req.Title, req.Description, req.EventImageURL, nullTime(eventTime), req.Location, req.OrganizerName, req.MerchantWallet, req.AmountUSDC*1_000_000, req.EventSource, req.SourceURL, string(formJSON), string(methodsJSON), checkoutExpiresAt, eventID, ownerEmail)
	if err != nil {
		http.Error(w, "Failed to update event", http.StatusInternalServerError)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		http.Error(w, "Event not found or not owned by user", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteEvent(w http.ResponseWriter, r *http.Request, eventID int64, ownerEmail string) {
	locked, err := hasPaidDeposits(eventID)
	if err != nil {
		http.Error(w, "Failed to validate event state", http.StatusInternalServerError)
		return
	}
	if locked {
		http.Error(w, "event cannot be deleted after at least one successful deposit", http.StatusConflict)
		return
	}
	res, err := database.DB.Exec(`DELETE FROM events WHERE id = $1 AND owner_email = $2`, eventID, ownerEmail)
	if err != nil {
		http.Error(w, "Failed to delete event", http.StatusInternalServerError)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		http.Error(w, "Event not found or not owned by user", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// /api/events/{id} or /api/events/{id}/checkouts
func EventsSubroutes(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/events/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	eventID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPut:
			updateEvent(w, r, eventID, claims.Email)
		case http.MethodDelete:
			deleteEvent(w, r, eventID, claims.Email)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	switch parts[1] {
	case "invite-codes":
		http.Error(w, "Invite codes are removed from this product version", http.StatusGone)
		return
	case "checkouts":
		if r.Method != http.MethodGet {
			http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := database.DB.Query(`
			SELECT ec.id, ec.wallet_address, ec.reference, COALESCE(ec.signature, ''), ec.status, COALESCE(ec.payment_method, 'wallet'), ec.created_at, ec.paid_at, ec.participant_data
			FROM event_checkouts ec
			JOIN events e ON e.id = ec.event_id
			WHERE ec.event_id = $1 AND e.owner_email = $2
			ORDER BY ec.created_at DESC
			LIMIT 500
		`, eventID, claims.Email)
		if err != nil {
			http.Error(w, "Failed to list checkouts", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := make([]EventCheckoutRow, 0)
		for rows.Next() {
			var row EventCheckoutRow
			var participantJSON []byte
			if err := rows.Scan(&row.ID, &row.WalletAddress, &row.Reference, &row.Signature, &row.Status, &row.PaymentMethod, &row.CreatedAt, &row.PaidAt, &participantJSON); err == nil {
				if len(participantJSON) > 0 {
					_ = json.Unmarshal(participantJSON, &row.ParticipantData)
				}
				out = append(out, row)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"checkouts": out})
	default:
		http.NotFound(w, r)
	}
}

// Kept only for backwards compatibility if older frontend still calls this route.
func CheckoutsSubroutes(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Checkout moderation endpoints are removed", http.StatusGone)
}

func normalizeAndValidateEventRequest(req *CreateEventRequest) error {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.EventImageURL = strings.TrimSpace(req.EventImageURL)
	req.Location = strings.TrimSpace(req.Location)
	req.OrganizerName = strings.TrimSpace(req.OrganizerName)
	req.EventSource = strings.ToLower(strings.TrimSpace(req.EventSource))
	req.SourceURL = strings.TrimSpace(req.SourceURL)
	req.CheckoutExpiresAt = strings.TrimSpace(req.CheckoutExpiresAt)

	if req.EventSource == "" {
		req.EventSource = "custom"
	}
	if req.EventSource != "custom" && req.EventSource != "luma" {
		return fmt.Errorf("eventSource must be custom or luma")
	}
	if req.EventSource == "luma" && req.SourceURL == "" {
		return fmt.Errorf("sourceUrl is required when eventSource is luma")
	}
	if req.CheckoutExpiresAt == "" {
		return fmt.Errorf("checkoutExpiresAt is required")
	}
	if _, err := time.Parse(time.RFC3339, req.CheckoutExpiresAt); err != nil {
		return fmt.Errorf("checkoutExpiresAt must be RFC3339 format")
	}
	if req.Title == "" || req.MerchantWallet == "" || req.AmountUSDC <= 0 {
		return fmt.Errorf("title, merchantWallet, amountUsdc are required")
	}
	if !isValidWalletAddress(req.MerchantWallet) {
		return fmt.Errorf("merchantWallet is invalid")
	}

	req.ParticipantFormSchema = normalizeParticipantFields(req.ParticipantFormSchema)
	req.PaymentMethods = normalizePaymentMethods(req.PaymentMethods)
	return nil
}

func normalizePaymentMethods(methods PaymentMethods) PaymentMethods {
	if !methods.Wallet && !methods.QR {
		return PaymentMethods{Wallet: true, QR: true}
	}
	return methods
}

func hasPaidDeposits(eventID int64) (bool, error) {
	var exists bool
	err := database.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM event_checkouts
			WHERE event_id = $1 AND status = 'paid'
		)
	`, eventID).Scan(&exists)
	return exists, err
}

func buildEventSlug(title string) string {
	base := strings.ToLower(strings.TrimSpace(title))
	base = strings.ReplaceAll(base, "'", "")
	base = strings.ReplaceAll(base, "\"", "")
	base = strings.ReplaceAll(base, "&", " and ")
	base = strings.ReplaceAll(base, "_", "-")
	base = strings.ReplaceAll(base, " ", "-")

	filtered := make([]rune, 0, len(base))
	lastDash := false
	for _, ch := range base {
		ok := (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
		if ok {
			filtered = append(filtered, ch)
			lastDash = false
			continue
		}
		if ch == '-' && !lastDash {
			filtered = append(filtered, '-')
			lastDash = true
		}
	}
	slug := strings.Trim(string(filtered), "-")
	if len(slug) > 64 {
		slug = slug[:64]
		slug = strings.Trim(slug, "-")
	}
	return slug
}

func ensureUniqueEventSlug(base string) string {
	slug := base
	for i := 0; i < 50; i++ {
		var exists bool
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM events WHERE slug = $1)", slug).Scan(&exists)
		if err == nil && !exists {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, (time.Now().Unix()%100000)+int64(i))
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano()%1000000)
}

func insertInviteCodes(eventID int64, count int, maxUses int, length int) ([]string, error) {
	// Deprecated: kept so older code paths continue compiling without behavior changes.
	if maxUses <= 0 {
		maxUses = 1
	}
	if length < 6 {
		length = 8
	}
	codes := make([]string, 0, count)
	for len(codes) < count {
		code := randomInviteCode(length)
		_, err := database.DB.Exec(`
			INSERT INTO invite_codes (event_id, code, max_uses)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO NOTHING
		`, eventID, code, maxUses)
		if err != nil {
			return nil, err
		}
		var exists bool
		if err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM invite_codes WHERE event_id = $1 AND code = $2)", eventID, code).Scan(&exists); err == nil && exists {
			codes = append(codes, code)
		}
	}
	return codes, nil
}

func randomInviteCode(length int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	out := make([]byte, length)
	for i := range out {
		out[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(out)
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
