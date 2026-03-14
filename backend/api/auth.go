package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"

	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthUserResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Provider string `json:"provider"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GoogleLoginRequest struct {
	Credential string `json:"credential"`
}

type authResponse struct {
	User AuthUserResponse `json:"user"`
}

type authMessageResponse struct {
	Message string `json:"message"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

func Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if name == "" || email == "" || password == "" {
		http.Error(w, "name, email, password are required", http.StatusBadRequest)
		return
	}
	if len(password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if len(email) > 254 || !strings.Contains(email, "@") {
		http.Error(w, "email is invalid", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	var userID int64
	err = database.DB.QueryRow(`
		INSERT INTO users (name, email, password_hash, auth_provider, email_verified)
		VALUES ($1, $2, $3, 'password', false)
		RETURNING id
	`, name, email, string(hash)).Scan(&userID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	if err := createAndSendVerificationToken(userID, email); err != nil {
		http.Error(w, "Failed to send verification email", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authMessageResponse{
		Message: "Registration successful. Please verify your email before login.",
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password
	if email == "" || password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	var userID int64
	var name, provider string
	var passwordHash sql.NullString
	var emailVerified bool
	err := database.DB.QueryRow(`
		SELECT id, name, COALESCE(password_hash, ''), auth_provider, email_verified
		FROM users
		WHERE email = $1
	`, email).Scan(&userID, &name, &passwordHash, &provider, &emailVerified)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if provider != "password" || !passwordHash.Valid || strings.TrimSpace(passwordHash.String) == "" {
		http.Error(w, "This account uses Google login", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if !emailVerified {
		http.Error(w, "EMAIL_NOT_VERIFIED", http.StatusForbidden)
		return
	}

	if err := issueSessionAndRespond(w, r, userID, name, email, provider); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	credential := strings.TrimSpace(req.Credential)
	if credential == "" {
		http.Error(w, "credential is required", http.StatusBadRequest)
		return
	}

	email, name, err := verifyGoogleIDToken(credential)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var userID int64
	var provider string
	err = database.DB.QueryRow(`
		SELECT id, auth_provider
		FROM users
		WHERE email = $1
	`, email).Scan(&userID, &provider)
	if err != nil {
		if err != sql.ErrNoRows {
			http.Error(w, "Failed to query user", http.StatusInternalServerError)
			return
		}

		err = database.DB.QueryRow(`
			INSERT INTO users (name, email, auth_provider, email_verified)
			VALUES ($1, $2, 'google', true)
			RETURNING id
		`, name, email).Scan(&userID)
		if err != nil {
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
		provider = "google"
	} else {
		_, _ = database.DB.Exec(`UPDATE users SET name = $1, email_verified = true, updated_at = NOW() WHERE id = $2`, name, userID)
	}

	if provider == "password" {
		http.Error(w, "This email is already registered with password login", http.StatusConflict)
		return
	}

	if err := issueSessionAndRespond(w, r, userID, name, email, provider); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := middleware.CurrentUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var user AuthUserResponse
	err := database.DB.QueryRow(`
		SELECT id, name, email, auth_provider
		FROM users
		WHERE id = $1
	`, claims.UserID).Scan(&user.ID, &user.Name, &user.Email, &user.Provider)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authResponse{User: user})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, err := middleware.GetClaimsFromRequest(r)
	if err == nil {
		_ = middleware.RevokeSession(claims.SessionID)
	}
	middleware.ClearSessionCookie(w, r)
	w.WriteHeader(http.StatusOK)
}

func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	hash := hashVerificationToken(token)

	result, err := database.DB.Exec(`
		UPDATE users
		SET email_verified = true,
		    email_verification_token_hash = NULL,
		    email_verification_expires_at = NULL,
		    updated_at = NOW()
		WHERE email_verification_token_hash = $1
		  AND email_verification_expires_at > NOW()
		  AND email_verified = false
	`, hash)
	if err != nil {
		http.Error(w, "verification failed", http.StatusInternalServerError)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authMessageResponse{Message: "Email verified successfully. You can now login."})
}

func ResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(authMessageResponse{Message: "If the email exists, a verification message has been sent."})
		return
	}

	var userID int64
	var verified bool
	var provider string
	err := database.DB.QueryRow(`SELECT id, email_verified, auth_provider FROM users WHERE email = $1`, email).Scan(&userID, &verified, &provider)
	if err != nil || verified || provider != "password" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(authMessageResponse{Message: "If the email exists, a verification message has been sent."})
		return
	}
	if err := createAndSendVerificationToken(userID, email); err != nil {
		http.Error(w, "Failed to send verification email", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authMessageResponse{Message: "If the email exists, a verification message has been sent."})
}

func issueSessionAndRespond(w http.ResponseWriter, r *http.Request, userID int64, name, email, provider string) error {
	sessionID := uuid.NewString()
	expiresAt := middleware.SessionExpiryFromNow()

	ipAddr := clientIP(r)
	userAgent := strings.TrimSpace(r.UserAgent())
	_, err := database.DB.Exec(`
		INSERT INTO user_sessions (id, user_id, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
	`, sessionID, userID, expiresAt, ipAddr, userAgent)
	if err != nil {
		return fmt.Errorf("failed to create session")
	}

	token, err := middleware.SignJWT(middleware.AuthClaims{
		UserID:    userID,
		Email:     email,
		SessionID: sessionID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return err
	}

	middleware.SetSessionCookie(w, r, token, expiresAt)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authResponse{User: AuthUserResponse{
		ID:       userID,
		Name:     name,
		Email:    email,
		Provider: provider,
	}})
	return nil
}

func createAndSendVerificationToken(userID int64, email string) error {
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	hash := hashVerificationToken(token)
	expiresAt := time.Now().Add(30 * time.Minute)

	_, err = database.DB.Exec(`
		UPDATE users
		SET email_verification_token_hash = $1,
		    email_verification_expires_at = $2,
		    email_verification_sent_at = NOW(),
		    updated_at = NOW()
		WHERE id = $3
	`, hash, expiresAt, userID)
	if err != nil {
		return err
	}

	return sendVerificationEmail(email, token)
}

func randomToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sendVerificationEmail(email, token string) error {
	appURL := strings.TrimSpace(os.Getenv("APP_BASE_URL"))
	if appURL == "" {
		appURL = "https://pay.crea8r.xyz"
	}
	verifyLink := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(appURL, "/"), url.QueryEscape(token))

	mode := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_MODE")))
	if mode == "" {
		mode = "log"
	}
	if mode == "log" {
		fmt.Printf("[email-verify] %s -> %s\n", email, verifyLink)
		return nil
	}

	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	smtpPort := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	smtpUser := strings.TrimSpace(os.Getenv("SMTP_USER"))
	smtpPass := strings.TrimSpace(os.Getenv("SMTP_PASS"))
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		from = smtpUser
	}
	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" || from == "" {
		return fmt.Errorf("smtp is not configured")
	}

	subject := "Verify your Payknot account"
	body := fmt.Sprintf("Please verify your email by opening this link:\n\n%s\n\nThis link expires in 30 minutes.", verifyLink)
	msg := []byte("From: " + from + "\r\n" +
		"To: " + email + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
		body + "\r\n")

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{email}, msg)
}

func verifyGoogleIDToken(idToken string) (string, string, error) {
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("google verification unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("invalid google credential")
	}

	var payload struct {
		Aud           string `json:"aud"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		EmailVerified string `json:"email_verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", fmt.Errorf("invalid google response")
	}

	expectedClientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	if expectedClientID != "" && payload.Aud != expectedClientID {
		return "", "", fmt.Errorf("google credential audience mismatch")
	}

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" || payload.EmailVerified != "true" {
		return "", "", fmt.Errorf("google email is not verified")
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = email
	}

	return email, name, nil
}

func clientIP(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		first := strings.TrimSpace(parts[0])
		if first != "" {
			return first
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && ip != "" {
		return ip
	}
	return r.RemoteAddr
}
