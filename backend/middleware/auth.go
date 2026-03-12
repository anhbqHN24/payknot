package middleware

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"solana_paywall/backend/database"
)

type contextKey string

const authClaimsKey contextKey = "auth_claims"

type AuthClaims struct {
	UserID    int64  `json:"sub"`
	Email     string `json:"email"`
	SessionID string `json:"sid"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func SessionCookieName() string {
	if v := strings.TrimSpace(os.Getenv("AUTH_COOKIE_NAME")); v != "" {
		return v
	}
	return "spw_session"
}

func sessionTTL() time.Duration {
	return 30 * 24 * time.Hour
}

func jwtSecret() string {
	return strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET"))
}

func secureCookieEnabled(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_COOKIE_SECURE")), "true") {
		return true
	}
	return r.TLS != nil
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieEnabled(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieEnabled(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func SignJWT(claims AuthClaims) (string, error) {
	secret := jwtSecret()
	if secret == "" {
		return "", errors.New("AUTH_JWT_SECRET is not set")
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

func ParseAndVerifyJWT(token string) (AuthClaims, error) {
	var claims AuthClaims
	secret := jwtSecret()
	if secret == "" {
		return claims, errors.New("AUTH_JWT_SECRET is not set")
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
	if err != nil {
		return claims, errors.New("invalid token signature encoding")
	}
	if !hmac.Equal(sig, expected) {
		return claims, errors.New("invalid token signature")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, errors.New("invalid token header")
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return claims, errors.New("invalid token header json")
	}
	if header["alg"] != "HS256" {
		return claims, errors.New("unsupported jwt alg")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errors.New("invalid token payload")
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return claims, errors.New("invalid token payload json")
	}

	now := time.Now().Unix()
	if claims.ExpiresAt <= now {
		return claims, errors.New("token expired")
	}
	if claims.UserID <= 0 || strings.TrimSpace(claims.Email) == "" || strings.TrimSpace(claims.SessionID) == "" {
		return claims, errors.New("invalid token claims")
	}

	return claims, nil
}

func GetClaimsFromRequest(r *http.Request) (AuthClaims, error) {
	var claims AuthClaims
	cookie, err := r.Cookie(SessionCookieName())
	if err != nil {
		return claims, err
	}
	return ParseAndVerifyJWT(strings.TrimSpace(cookie.Value))
}

func SessionExpiryFromNow() time.Time {
	return time.Now().Add(sessionTTL())
}

func agentPublicKeys() map[string]ed25519.PublicKey {
	raw := strings.TrimSpace(os.Getenv("AGENT_PUBLIC_KEYS_JSON"))
	if raw == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	out := make(map[string]ed25519.PublicKey, len(m))
	for agentID, v := range m {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
		if err != nil || len(keyBytes) != ed25519.PublicKeySize {
			continue
		}
		out[agentID] = ed25519.PublicKey(keyBytes)
	}
	return out
}

func verifyAgentSignature(r *http.Request) (AuthClaims, bool) {
	agentID := strings.TrimSpace(r.Header.Get("X-Agent-Id"))
	tsRaw := strings.TrimSpace(r.Header.Get("X-Agent-Timestamp"))
	sigRaw := strings.TrimSpace(r.Header.Get("X-Agent-Signature"))
	if agentID == "" || tsRaw == "" || sigRaw == "" {
		return AuthClaims{}, false
	}

	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return AuthClaims{}, false
	}
	now := time.Now().Unix()
	if ts < now-300 || ts > now+60 {
		return AuthClaims{}, false
	}

	keys := agentPublicKeys()
	pub, ok := keys[agentID]
	if !ok {
		return AuthClaims{}, false
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return AuthClaims{}, false
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	bodyHash := sha256.Sum256(bodyBytes)

	canonical := strings.Join([]string{
		r.Method,
		r.URL.Path,
		tsRaw,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")

	sig, err := base64.StdEncoding.DecodeString(sigRaw)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return AuthClaims{}, false
	}
	if !ed25519.Verify(pub, []byte(canonical), sig) {
		return AuthClaims{}, false
	}

	return AuthClaims{
		UserID:    -1,
		Email:     "agent:" + agentID,
		SessionID: "agent:" + agentID,
		IssuedAt:  now,
		ExpiresAt: now + 3600,
	}, true
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := GetClaimsFromRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var exists bool
		err = database.DB.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM user_sessions
				WHERE id = $1
				  AND user_id = $2
				  AND revoked_at IS NULL
				  AND expires_at > NOW()
			)
		`, claims.SessionID, claims.UserID).Scan(&exists)
		if err != nil || !exists {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), authClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func RequireAuthOrAgentKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := verifyAgentSignature(r); ok {
			ctx := context.WithValue(r.Context(), authClaimsKey, claims)
			next(w, r.WithContext(ctx))
			return
		}
		RequireAuth(next)(w, r)
	}
}

func CurrentUser(r *http.Request) (AuthClaims, bool) {
	claims, ok := r.Context().Value(authClaimsKey).(AuthClaims)
	return claims, ok
}

func RevokeSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	_, err := database.DB.Exec(`UPDATE user_sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}
