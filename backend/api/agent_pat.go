package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"

	"github.com/google/uuid"
)

type AgentPATCreateRequest struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expiresInDays"`
}

type AgentPATRevokeRequest struct {
	TokenID string `json:"tokenId"`
}

type AgentPATView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TokenHint  string `json:"tokenHint"`
	Scope      string `json:"scope"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	RevokedAt  string `json:"revokedAt,omitempty"`
}

func AgentPATsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ListAgentPATs(w, r)
	case http.MethodPost:
		CreateAgentPAT(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func ListAgentPATs(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT id::text, token_name, token_prefix, scope, created_at, last_used_at, expires_at, revoked_at
		FROM agent_personal_access_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, claims.UserID)
	if err != nil {
		http.Error(w, "Failed to list agent PATs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]AgentPATView, 0)
	for rows.Next() {
		var item AgentPATView
		var createdAt time.Time
		var lastUsedAt, expiresAt, revokedAt *time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.TokenHint, &item.Scope, &createdAt, &lastUsedAt, &expiresAt, &revokedAt); err != nil {
			continue
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		if lastUsedAt != nil {
			item.LastUsedAt = lastUsedAt.Format(time.RFC3339)
		}
		if expiresAt != nil {
			item.ExpiresAt = expiresAt.Format(time.RFC3339)
		}
		if revokedAt != nil {
			item.RevokedAt = revokedAt.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"tokens": items})
}

func CreateAgentPAT(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AgentPATCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > 365 {
		http.Error(w, "expiresInDays must be between 0 and 365", http.StatusBadRequest)
		return
	}

	secret, err := generatePATSecret()
	if err != nil {
		http.Error(w, "Failed to generate PAT", http.StatusInternalServerError)
		return
	}
	token := "pkt_pat_" + secret
	tokenPrefix := token
	if len(tokenPrefix) > 18 {
		tokenPrefix = tokenPrefix[:18] + "..."
	}
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	tokenID := uuid.NewString()

	var expiresAt interface{}
	var expiresAtValue string
	if req.ExpiresInDays > 0 {
		ts := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = ts
		expiresAtValue = ts.Format(time.RFC3339)
	}

	_, err = database.DB.Exec(`
		INSERT INTO agent_personal_access_tokens(id, user_id, token_name, token_prefix, token_hash, scope, expires_at)
		VALUES ($1::uuid, $2, $3, $4, $5, 'agent:runtime', $6)
	`, tokenID, claims.UserID, req.Name, tokenPrefix, tokenHash, expiresAt)
	if err != nil {
		http.Error(w, "Failed to create PAT", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":         true,
		"tokenId":    tokenID,
		"name":       req.Name,
		"token":      token,
		"scope":      "agent:runtime",
		"expiresAt":  expiresAtValue,
		"createdAt":  time.Now().UTC().Format(time.RFC3339),
		"tokenHint":  tokenPrefix,
		"ownerEmail": claims.Email,
	})
}

func RevokeAgentPAT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AgentPATRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.TokenID = strings.TrimSpace(req.TokenID)
	if req.TokenID == "" {
		http.Error(w, "tokenId is required", http.StatusBadRequest)
		return
	}

	_, err := database.DB.Exec(`
		UPDATE agent_personal_access_tokens
		SET revoked_at = NOW()
		WHERE id = $1::uuid
		  AND user_id = $2
		  AND revoked_at IS NULL
	`, req.TokenID, claims.UserID)
	if err != nil {
		http.Error(w, "Failed to revoke PAT", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "tokenId": req.TokenID})
}

func generatePATSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
