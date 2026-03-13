package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"solana_paywall/backend/database"
)

type agentContextKey string

const agentClaimsKey agentContextKey = "agent_claims"

type AgentClaims struct {
	AgentPubkey string `json:"sub"`
	JTI         string `json:"jti"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
	Scope       string `json:"scope"`
}

func agentJWTSecret() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_JWT_SECRET")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET"))
}

func SignAgentJWT(claims AgentClaims) (string, error) {
	secret := agentJWTSecret()
	if secret == "" {
		return "", errors.New("AGENT_JWT_SECRET is not set")
	}
	headerBytes, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature, nil
}

func ParseAndVerifyAgentJWT(token string) (AgentClaims, error) {
	var claims AgentClaims
	secret := agentJWTSecret()
	if secret == "" {
		return claims, errors.New("AGENT_JWT_SECRET is not set")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, errors.New("invalid token format")
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, expected) {
		return claims, errors.New("invalid token signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errors.New("invalid payload")
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return claims, errors.New("invalid claims")
	}
	if claims.AgentPubkey == "" || claims.JTI == "" || claims.ExpiresAt <= time.Now().Unix() {
		return claims, errors.New("expired or invalid claims")
	}
	return claims, nil
}

func RequireAgentJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimSpace(authz[7:])
		claims, err := ParseAndVerifyAgentJWT(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var ok bool
		err = database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM agent_sessions
				WHERE jwt_jti = $1
				  AND agent_pubkey = $2
				  AND revoked_at IS NULL
				  AND expires_at > NOW()
			)
		`, claims.JTI, claims.AgentPubkey).Scan(&ok)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = database.DB.Exec(`UPDATE agent_sessions SET last_seen_at = NOW() WHERE jwt_jti = $1`, claims.JTI)
		ctx := context.WithValue(r.Context(), agentClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func CurrentAgent(r *http.Request) (AgentClaims, bool) {
	claims, ok := r.Context().Value(agentClaimsKey).(AgentClaims)
	return claims, ok
}
