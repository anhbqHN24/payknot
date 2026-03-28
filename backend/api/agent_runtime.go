package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"github.com/mr-tron/base58"
	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"
)

type AgentTokenRequest struct {
	AgentPubkey string `json:"agent_pubkey"`
	Nonce       string `json:"nonce"`
	Signature   string `json:"signature"`
}

type AgentCheckoutCreateRequest struct {
	EventID    int64   `json:"event_id"`
	Recipient  string  `json:"recipient"`
	AmountUSDC float64 `json:"amount_usdc"`
	Memo       string  `json:"memo"`
}

type AgentPATTokenRequest struct {
	Token         string `json:"token"`
	SessionPubkey string `json:"session_pubkey"`
	Label         string `json:"label"`
}

func AgentAuthNonce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pub := strings.TrimSpace(r.URL.Query().Get("agent_pubkey"))
	if pub == "" {
		http.Error(w, "agent_pubkey is required", http.StatusBadRequest)
		return
	}
	if _, err := base58.Decode(pub); err != nil {
		http.Error(w, "invalid agent_pubkey", http.StatusBadRequest)
		return
	}
	nonce, err := randomNonce(32)
	if err != nil {
		http.Error(w, "failed to generate nonce", http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(5 * time.Minute)
	_, err = database.DB.Exec(`
		INSERT INTO agent_nonces(agent_pubkey, nonce, expires_at)
		VALUES ($1,$2,$3)
	`, pub, nonce, expires)
	if err != nil {
		http.Error(w, "failed to persist nonce", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"nonce":      nonce,
		"expires_at": expires.UTC().Format(time.RFC3339),
	})
}

func AgentAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AgentTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.AgentPubkey = strings.TrimSpace(req.AgentPubkey)
	req.Nonce = strings.TrimSpace(req.Nonce)
	req.Signature = strings.TrimSpace(req.Signature)
	if req.AgentPubkey == "" || req.Nonce == "" || req.Signature == "" {
		http.Error(w, "agent_pubkey, nonce, signature are required", http.StatusBadRequest)
		return
	}

	var expiresAt time.Time
	var usedAt *time.Time
	err := database.DB.QueryRow(`
		SELECT expires_at, used_at
		FROM agent_nonces
		WHERE agent_pubkey=$1 AND nonce=$2
		ORDER BY created_at DESC LIMIT 1
	`, req.AgentPubkey, req.Nonce).Scan(&expiresAt, &usedAt)
	if err != nil {
		http.Error(w, "invalid nonce", http.StatusUnauthorized)
		return
	}
	if usedAt != nil || expiresAt.Before(time.Now()) {
		http.Error(w, "nonce expired or already used", http.StatusUnauthorized)
		return
	}

	pubBytes, err := base58.Decode(req.AgentPubkey)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		http.Error(w, "invalid agent_pubkey", http.StatusUnauthorized)
		return
	}
	sigBytes, err := base58.Decode(req.Signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), []byte(req.Nonce), sigBytes) {
		http.Error(w, "signature verification failed", http.StatusUnauthorized)
		return
	}

	_, _ = database.DB.Exec(`UPDATE agent_nonces SET used_at = NOW() WHERE agent_pubkey=$1 AND nonce=$2`, req.AgentPubkey, req.Nonce)

	writeAgentAuthResponse(w, req.AgentPubkey, "agent:settlement")
}

func AgentAuthPAT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentPATTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.SessionPubkey = strings.TrimSpace(req.SessionPubkey)
	req.Label = strings.TrimSpace(req.Label)

	pat, ok := middleware.PersonalAccessTokenAuthFromToken(req.Token)
	if !ok {
		http.Error(w, "invalid PAT", http.StatusUnauthorized)
		return
	}

	if req.SessionPubkey != "" {
		pubBytes, err := base58.Decode(req.SessionPubkey)
		if err != nil || len(pubBytes) != ed25519.PublicKeySize {
			http.Error(w, "invalid session_pubkey", http.StatusBadRequest)
			return
		}
		writeAgentAuthResponse(w, req.SessionPubkey, "agent:settlement")
		return
	}

	writeAgentAuthResponse(w, "pat:"+pat.TokenID, "agent:runtime")
}

func AgentAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	agent, ok := middleware.CurrentAgent(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":    agent.AgentPubkey,
		"scope":       agent.Scope,
		"issued_at":   time.Unix(agent.IssuedAt, 0).UTC().Format(time.RFC3339),
		"expires_at":  time.Unix(agent.ExpiresAt, 0).UTC().Format(time.RFC3339),
		"auth_method": authMethodForAgent(agent.AgentPubkey),
	})
}

func AgentCheckoutCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	agent, ok := middleware.CurrentAgent(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req AgentCheckoutCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Recipient = strings.TrimSpace(req.Recipient)
	req.Memo = strings.TrimSpace(req.Memo)
	if req.AmountUSDC <= 0 || req.AmountUSDC > 500 {
		http.Error(w, "policy rejected: amount_usdc must be > 0 and <= 500", http.StatusForbidden)
		return
	}
	if req.Memo == "" {
		http.Error(w, "policy rejected: memo is required", http.StatusForbidden)
		return
	}
	if _, err := solana.PublicKeyFromBase58(req.Recipient); err != nil {
		http.Error(w, "policy rejected: recipient must be valid Solana address", http.StatusForbidden)
		return
	}

	var hourlyLimit int
	err := database.DB.QueryRow(`
		SELECT COALESCE(hourly_tx_limit, 10)
		FROM automation_policies
		WHERE active = TRUE
		ORDER BY id ASC LIMIT 1
	`).Scan(&hourlyLimit)
	if err != nil || hourlyLimit <= 0 {
		hourlyLimit = 10
	}
	var hourCount int
	_ = database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM automation_intents
		WHERE agent_pubkey = $1
		  AND created_at > NOW() - INTERVAL '1 hour'
		  AND status IN ('signed','broadcasted','confirmed')
	`, agent.AgentPubkey).Scan(&hourCount)
	if hourCount >= hourlyLimit {
		http.Error(w, "policy rejected: hourly transaction limit exceeded", http.StatusForbidden)
		return
	}

	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem != "" {
		var txSig string
		err := database.DB.QueryRow(`
			SELECT COALESCE(tx_signature,'')
			FROM automation_intents
			WHERE agent_pubkey = $1 AND idempotency_key = $2
			LIMIT 1
		`, agent.AgentPubkey, idem).Scan(&txSig)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"tx_signature": txSig,
				"explorer_url": explorerFor(txSig),
			})
			return
		}
	}

	intentID := uuid.NewString()
	_, _ = database.DB.Exec(`
		INSERT INTO automation_intents(id, agent_pubkey, event_id, recipient_wallet, amount_usdc, memo, status, idempotency_key)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,'queued',$7)
	`, intentID, agent.AgentPubkey, req.EventID, req.Recipient, req.AmountUSDC, req.Memo, nullIfEmpty(idem))

	mockEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("SETTLEMENT_MOCK_SUCCESS")), "true") || strings.TrimSpace(os.Getenv("SETTLEMENT_MOCK_SUCCESS")) == ""
	if mockEnabled {
		mockSig := "mock_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		_, _ = database.DB.Exec(`
			UPDATE automation_intents
			SET status='confirmed', tx_signature=$2, updated_at=NOW()
			WHERE id=$1::uuid
		`, intentID, mockSig)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"tx_signature": mockSig,
			"explorer_url": explorerFor(mockSig),
		})
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE automation_intents
		SET status='failed', policy_reason='settlement signer is not configured', updated_at=NOW()
		WHERE id=$1::uuid
	`, intentID)
	http.Error(w, "settlement signer is not configured", http.StatusInternalServerError)
}

func randomNonce(n int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		x, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[i] = chars[x.Int64()]
	}
	return string(b), nil
}

func nullIfEmpty(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}

func explorerFor(sig string) string {
	if strings.TrimSpace(sig) == "" {
		return ""
	}
	return fmt.Sprintf("https://explorer.solana.com/tx/%s", sig)
}

func writeAgentAuthResponse(w http.ResponseWriter, agentID, scope string) {
	jti := uuid.NewString()
	sessionExp := time.Now().Add(24 * time.Hour)
	claims := middleware.AgentClaims{
		AgentPubkey: agentID,
		JTI:         jti,
		IssuedAt:    time.Now().Unix(),
		ExpiresAt:   sessionExp.Unix(),
		Scope:       scope,
	}
	token, err := middleware.SignAgentJWT(claims)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	_, err = database.DB.Exec(`
		INSERT INTO agent_sessions(id, agent_pubkey, jwt_jti, expires_at)
		VALUES ($1::uuid,$2,$3,$4)
	`, uuid.NewString(), agentID, jti, sessionExp)
	if err != nil {
		http.Error(w, "failed to persist session", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":             token,
		"token_type":               "Bearer",
		"expires_in":               86400,
		"agent_id":                 agentID,
		"scope":                    scope,
		"requires_signed_requests": strings.HasPrefix(scope, "agent:settlement"),
	})
}

func authMethodForAgent(agentID string) string {
	if strings.HasPrefix(agentID, "pat:") {
		return "pat"
	}
	if _, err := base58.Decode(agentID); err == nil {
		return "ed25519_session"
	}
	return "unknown"
}
