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

type CreateEventRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	EventImageURL  string `json:"eventImageUrl"`
	EventDate      string `json:"eventDate"`
	Location       string `json:"location"`
	OrganizerName  string `json:"organizerName"`
	MerchantWallet string `json:"merchantWallet"`
	AmountUSDC     int64  `json:"amountUsdc"`
	Network        string `json:"network"`
}

type UpdateEventRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	EventImageURL  string `json:"eventImageUrl"`
	EventDate      string `json:"eventDate"`
	Location       string `json:"location"`
	OrganizerName  string `json:"organizerName"`
	MerchantWallet string `json:"merchantWallet"`
	AmountUSDC     int64  `json:"amountUsdc"`
	Network        string `json:"network"`
}

type CreateEventResponse struct {
	EventID        int64    `json:"eventId"`
	Slug           string   `json:"slug"`
	CheckoutURL    string   `json:"checkoutUrl"`
	InviteCodes    []string `json:"inviteCodes"`
	EventImageURL  string   `json:"eventImageUrl"`
	AmountUSDC     string   `json:"amountUsdc"`
	Network        string   `json:"network"`
	MerchantWallet string   `json:"merchantWallet"`
}

type EventSummary struct {
	EventID        int64  `json:"eventId"`
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	EventImageURL  string `json:"eventImageUrl"`
	EventDate      string `json:"eventDate"`
	Location       string `json:"location"`
	OrganizerName  string `json:"organizerName"`
	MerchantWallet string `json:"merchantWallet"`
	AmountUSDC     string `json:"amountUsdc"`
	Network        string `json:"network"`
	CreatedAt      string `json:"createdAt"`
}

type GenerateCodesRequest struct {
	Count  int `json:"count"`
	Length int `json:"length"`
}

type ApproveCheckoutRequest struct {
	ApprovedBy string `json:"approvedBy"`
	Notes      string `json:"notes"`
}

type RejectCheckoutRequest struct {
	RejectedBy string `json:"rejectedBy"`
	Reason     string `json:"reason"`
}

type EventCheckoutRow struct {
	ID            int64      `json:"id"`
	WalletAddress string     `json:"walletAddress"`
	Reference     string     `json:"reference"`
	Signature     string     `json:"signature"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	PaidAt        *time.Time `json:"paidAt"`
	ApprovedAt    *time.Time `json:"approvedAt"`
	ApprovedBy    string     `json:"approvedBy"`
	InviteCode    string     `json:"inviteCode"`
	Notes         string     `json:"notes"`
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
		SELECT id, slug, title, description, event_image_url, event_date, location, organizer_name, merchant_wallet, amount_usdc, network, created_at
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
		var amountRaw int64
		var created time.Time
		if scanErr := rows.Scan(&s.EventID, &s.Slug, &s.Title, &s.Description, &s.EventImageURL, &eventDate, &s.Location, &s.OrganizerName, &s.MerchantWallet, &amountRaw, &s.Network, &created); scanErr == nil {
			if eventDate != nil {
				s.EventDate = eventDate.Format(time.RFC3339)
			}
			s.AmountUSDC = fmt.Sprintf("%.2f", float64(amountRaw)/1_000_000)
			s.CreatedAt = created.Format(time.RFC3339)
			out = append(out, s)
		}
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

	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.EventImageURL = strings.TrimSpace(req.EventImageURL)
	req.Location = strings.TrimSpace(req.Location)
	req.OrganizerName = strings.TrimSpace(req.OrganizerName)
	req.Network = strings.TrimSpace(req.Network)
	if req.Network == "" {
		req.Network = "devnet"
	}

	if req.Title == "" || req.MerchantWallet == "" || req.AmountUSDC <= 0 {
		http.Error(w, "title, merchantWallet, amountUsdc are required", http.StatusBadRequest)
		return
	}
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !isValidWalletAddress(req.MerchantWallet) {
		http.Error(w, "merchantWallet is invalid", http.StatusBadRequest)
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

	slug := buildEventSlug(req.Title)
	if slug == "" {
		http.Error(w, "invalid title", http.StatusBadRequest)
		return
	}
	slug = ensureUniqueEventSlug(slug)

	var eventID int64
	err := database.DB.QueryRow(`
		INSERT INTO events (slug, title, description, event_image_url, event_date, location, organizer_name, merchant_wallet, amount_usdc, network, owner_email)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id
	`, slug, req.Title, req.Description, req.EventImageURL, nullTime(eventTime), req.Location, req.OrganizerName, req.MerchantWallet, req.AmountUSDC*1_000_000, req.Network, claims.Email).Scan(&eventID)
	if err != nil {
		http.Error(w, "Failed to create event", http.StatusInternalServerError)
		return
	}

	seedCodes, err := insertInviteCodes(eventID, 10, 1, 8)
	if err != nil {
		http.Error(w, "Failed to seed invite codes", http.StatusInternalServerError)
		return
	}

	resp := CreateEventResponse{
		EventID:        eventID,
		Slug:           slug,
		CheckoutURL:    "/checkout/" + slug,
		InviteCodes:    seedCodes,
		EventImageURL:  req.EventImageURL,
		AmountUSDC:     fmt.Sprintf("%.2f", float64(req.AmountUSDC)),
		Network:        req.Network,
		MerchantWallet: req.MerchantWallet,
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
	if !isValidWalletAddress(strings.TrimSpace(req.MerchantWallet)) {
		http.Error(w, "merchantWallet is invalid", http.StatusBadRequest)
		return
	}
	if req.AmountUSDC <= 0 {
		http.Error(w, "amountUsdc must be > 0", http.StatusBadRequest)
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
			network = $9
		WHERE id = $10 AND owner_email = $11
	`, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), strings.TrimSpace(req.EventImageURL), nullTime(eventTime), strings.TrimSpace(req.Location), strings.TrimSpace(req.OrganizerName), strings.TrimSpace(req.MerchantWallet), req.AmountUSDC*1_000_000, strings.TrimSpace(req.Network), eventID, ownerEmail)
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

// /api/events/{id} or /api/events/{id}/invite-codes or /api/events/{id}/checkouts
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
		if r.Method == http.MethodGet {
			rows, err := database.DB.Query(`
				SELECT ic.code
				FROM invite_codes ic
				JOIN events e ON e.id = ic.event_id
				WHERE ic.event_id = $1 AND e.owner_email = $2
				ORDER BY ic.created_at ASC, ic.id ASC
			`, eventID, claims.Email)
			if err != nil {
				http.Error(w, "Failed to list invite codes", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			codes := make([]string, 0)
			for rows.Next() {
				var code string
				if scanErr := rows.Scan(&code); scanErr == nil {
					codes = append(codes, code)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"codes": codes})
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Only POST or GET method is allowed", http.StatusMethodNotAllowed)
			return
		}
		var req GenerateCodesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Count <= 0 || req.Count > 500 {
			req.Count = 20
		}
		if req.Length < 6 || req.Length > 16 {
			req.Length = 8
		}
		var ownerOk bool
		if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM events WHERE id = $1 AND owner_email = $2)`, eventID, claims.Email).Scan(&ownerOk); err != nil || !ownerOk {
			http.Error(w, "Event not found or not owned by user", http.StatusNotFound)
			return
		}

		codes, err := insertInviteCodes(eventID, req.Count, 1, req.Length)
		if err != nil {
			http.Error(w, "Failed to generate invite codes", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"codes": codes})
	case "checkouts":
		if r.Method != http.MethodGet {
			http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := database.DB.Query(`
			SELECT ec.id, ec.wallet_address, ec.reference, COALESCE(ec.signature, ''), ec.status, ec.created_at, ec.paid_at, ec.approved_at, COALESCE(ec.approved_by, ''), ic.code, COALESCE(ec.notes, '')
			FROM event_checkouts ec
			JOIN invite_codes ic ON ic.id = ec.invite_code_id
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
			if err := rows.Scan(&row.ID, &row.WalletAddress, &row.Reference, &row.Signature, &row.Status, &row.CreatedAt, &row.PaidAt, &row.ApprovedAt, &row.ApprovedBy, &row.InviteCode, &row.Notes); err == nil {
				out = append(out, row)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"checkouts": out})
	default:
		http.NotFound(w, r)
	}
}

// /api/checkouts/{id}/approve or /api/checkouts/{id}/reject
func CheckoutsSubroutes(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/checkouts/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	checkoutID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid checkout id", http.StatusBadRequest)
		return
	}

	action := parts[1]
	switch action {
	case "approve":
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ApproveCheckoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.ApprovedBy) == "" {
			req.ApprovedBy = "organizer"
		}

		res, err := database.DB.Exec(`
			UPDATE event_checkouts ec
			SET status = 'approved', approved_at = NOW(), approved_by = $1, notes = $2
			FROM events e
			WHERE ec.id = $3 AND ec.event_id = e.id AND e.owner_email = $4 AND ec.status IN ('paid', 'approved')
		`, req.ApprovedBy, req.Notes, checkoutID, claims.Email)
		if err != nil {
			http.Error(w, "Failed to approve checkout", http.StatusInternalServerError)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			http.Error(w, "Checkout not in approvable status", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	case "reject":
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}
		var req RejectCheckoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			http.Error(w, "reason is required", http.StatusBadRequest)
			return
		}
		actor := strings.TrimSpace(req.RejectedBy)
		if actor == "" {
			actor = "organizer"
		}

		res, err := database.DB.Exec(`
			UPDATE event_checkouts ec
			SET status = 'rejected', approved_at = NOW(), approved_by = $1, notes = $2
			FROM events e
			WHERE ec.id = $3 AND ec.event_id = e.id AND e.owner_email = $4 AND ec.status IN ('paid', 'approved')
		`, actor, reason, checkoutID, claims.Email)
		if err != nil {
			http.Error(w, "Failed to reject checkout", http.StatusInternalServerError)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			http.Error(w, "Checkout not in rejectable status", http.StatusBadRequest)
			return
		}

		_, _ = database.DB.Exec(`
			UPDATE invite_codes
			SET used_count = GREATEST(used_count - 1, 0), status = 'active'
			WHERE id = (SELECT invite_code_id FROM event_checkouts WHERE id = $1)
		`, checkoutID)

		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
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
