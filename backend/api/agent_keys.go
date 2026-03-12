package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"
)

type AgentKeyUpsertRequest struct {
	AgentID         string `json:"agentId"`
	PublicKeyBase64 string `json:"publicKeyBase64"`
}

type AgentKeyRevokeRequest struct {
	AgentID string `json:"agentId"`
}

type AgentKeyView struct {
	AgentID       string `json:"agentId"`
	PublicKeyBase string `json:"publicKeyBase64"`
	Active        bool   `json:"active"`
	CreatedBy     string `json:"createdBy"`
	CreatedAt     string `json:"createdAt"`
	RevokedAt     string `json:"revokedAt,omitempty"`
}

func AgentKeysRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ListAgentKeys(w, r)
	case http.MethodPost:
		UpsertAgentKey(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func ListAgentKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
		SELECT agent_id, public_key_base64, active, created_by, created_at, revoked_at
		FROM agent_api_keys
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, "Failed to list agent keys", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]AgentKeyView, 0)
	for rows.Next() {
		var item AgentKeyView
		var createdAt time.Time
		var revokedAt *time.Time
		if scanErr := rows.Scan(&item.AgentID, &item.PublicKeyBase, &item.Active, &item.CreatedBy, &createdAt, &revokedAt); scanErr != nil {
			continue
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		if revokedAt != nil {
			item.RevokedAt = revokedAt.Format(time.RFC3339)
		}
		out = append(out, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"agentKeys": out})
}

func UpsertAgentKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AgentKeyUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.PublicKeyBase64 = strings.TrimSpace(req.PublicKeyBase64)
	if req.AgentID == "" || req.PublicKeyBase64 == "" {
		http.Error(w, "agentId and publicKeyBase64 are required", http.StatusBadRequest)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(req.PublicKeyBase64)
	if err != nil || len(decoded) != 32 {
		http.Error(w, "publicKeyBase64 must be valid base64 ed25519 public key", http.StatusBadRequest)
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO agent_api_keys (agent_id, public_key_base64, active, created_by)
		VALUES ($1, $2, TRUE, $3)
		ON CONFLICT (agent_id)
		DO UPDATE SET
			public_key_base64 = EXCLUDED.public_key_base64,
			active = TRUE,
			revoked_at = NULL,
			created_by = EXCLUDED.created_by
	`, req.AgentID, req.PublicKeyBase64, claims.Email)
	if err != nil {
		http.Error(w, "Failed to upsert agent key", http.StatusInternalServerError)
		return
	}

	middleware.InvalidateAgentKeyCache()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "agentId": req.AgentID})
}

func RevokeAgentKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentKeyRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.AgentID = strings.TrimSpace(req.AgentID)
	if req.AgentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	_, err := database.DB.Exec(`
		UPDATE agent_api_keys
		SET active = FALSE, revoked_at = NOW()
		WHERE agent_id = $1
	`, req.AgentID)
	if err != nil {
		http.Error(w, "Failed to revoke agent key", http.StatusInternalServerError)
		return
	}

	middleware.InvalidateAgentKeyCache()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "agentId": req.AgentID})
}
