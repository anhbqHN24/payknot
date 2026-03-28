package api

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
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

func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email != "" {
		var userID int64
		var provider string
		err := database.DB.QueryRow(`SELECT id, auth_provider FROM users WHERE email = $1`, email).Scan(&userID, &provider)
		if err == nil && provider == "password" {
			_ = createAndSendPasswordResetToken(userID, email)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authMessageResponse{Message: "If the email exists, a reset link has been sent."})
}

func ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	newPassword := req.NewPassword
	if token == "" || len(newPassword) < 8 {
		http.Error(w, "token and valid newPassword are required", http.StatusBadRequest)
		return
	}
	hash := hashVerificationToken(token)
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	result, err := database.DB.Exec(`
		UPDATE users
		SET password_hash = $1,
		    password_reset_token_hash = NULL,
		    password_reset_expires_at = NULL,
		    updated_at = NOW()
		WHERE password_reset_token_hash = $2
		  AND password_reset_expires_at > NOW()
		  AND auth_provider = 'password'
	`, string(newHash), hash)
	if err != nil {
		http.Error(w, "reset failed", http.StatusInternalServerError)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authMessageResponse{Message: "Password reset successful. You can now login."})
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

func createAndSendPasswordResetToken(userID int64, email string) error {
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	hash := hashVerificationToken(token)
	expiresAt := time.Now().Add(30 * time.Minute)
	_, err = database.DB.Exec(`
		UPDATE users
		SET password_reset_token_hash = $1,
		    password_reset_expires_at = $2,
		    password_reset_sent_at = NOW(),
		    updated_at = NOW()
		WHERE id = $3
	`, hash, expiresAt, userID)
	if err != nil {
		return err
	}
	return sendPasswordResetEmail(email, token)
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
	subject := "Verify your Payknot account"
	textBody := fmt.Sprintf(
		"Verify your Payknot account.\n\nOpen this link to verify your email:\n%s\n\nThis link expires in 30 minutes. If you did not create a Payknot account, you can ignore this email.",
		verifyLink,
	)
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <body style="margin:0;padding:0;background-color:#f3f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#0f172a;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color:#f3f5f7;padding:32px 16px;">
      <tr>
        <td align="center">
          <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="max-width:560px;background-color:#ffffff;border:1px solid #e2e8f0;border-radius:20px;overflow:hidden;">
            <tr>
              <td style="padding:32px 32px 12px 32px;">
                <div style="font-size:12px;letter-spacing:0.08em;text-transform:uppercase;color:#64748b;font-weight:700;">Payknot</div>
                <h1 style="margin:12px 0 0 0;font-size:28px;line-height:1.2;color:#0f172a;">Verify your email</h1>
              </td>
            </tr>
            <tr>
              <td style="padding:8px 32px 0 32px;font-size:15px;line-height:1.7;color:#334155;">
                <p style="margin:0 0 16px 0;">Thanks for creating your Payknot account. Confirm your email address to continue into the app and manage event payment sessions.</p>
                <p style="margin:0 0 24px 0;">This verification link expires in <strong>30 minutes</strong>.</p>
              </td>
            </tr>
            <tr>
              <td style="padding:0 32px 8px 32px;">
                <a href="%s" style="display:inline-block;background-color:#0f172a;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 22px;border-radius:12px;">Verify Email</a>
              </td>
            </tr>
            <tr>
              <td style="padding:16px 32px 0 32px;font-size:13px;line-height:1.7;color:#64748b;">
                <p style="margin:0 0 8px 0;">If the button does not work, copy and paste this link into your browser:</p>
                <p style="margin:0;word-break:break-all;"><a href="%s" style="color:#2563eb;text-decoration:underline;">%s</a></p>
              </td>
            </tr>
            <tr>
              <td style="padding:24px 32px 32px 32px;font-size:12px;line-height:1.7;color:#94a3b8;">
                <p style="margin:0;">If you did not create a Payknot account, you can safely ignore this email.</p>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`, verifyLink, verifyLink, verifyLink)
	return sendEmailWithModeHTML(email, subject, textBody, htmlBody, "email-verify")
}

func sendPasswordResetEmail(email, token string) error {
	appURL := strings.TrimSpace(os.Getenv("APP_BASE_URL"))
	if appURL == "" {
		appURL = "https://pay.crea8r.xyz"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(appURL, "/"), url.QueryEscape(token))
	subject := "Reset your Payknot password"
	body := fmt.Sprintf("You requested a password reset. Open this link to set a new password:\n\n%s\n\nThis link expires in 30 minutes.", resetLink)
	return sendEmailWithMode(email, subject, body, "password-reset")
}

func sendEmailWithMode(email, subject, body, logTag string) error {
	return sendEmailWithModeHTML(email, subject, body, "", logTag)
}

func sendEmailWithModeHTML(email, subject, textBody, htmlBody, logTag string) error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_MODE")))
	if mode == "" {
		if strings.TrimSpace(os.Getenv("RESEND_API_KEY")) != "" {
			mode = "resend"
		} else {
			mode = "log"
		}
	}
	if mode == "log" {
		fmt.Printf("[%s] %s -> %s\n", logTag, email, textBody)
		return nil
	}
	if mode == "resend" {
		return sendViaResend(email, subject, textBody, htmlBody, logTag)
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

	var msg []byte
	if strings.TrimSpace(htmlBody) == "" {
		msg = []byte("From: " + from + "\r\n" +
			"To: " + email + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
			textBody + "\r\n")
	} else {
		boundary := "payknot-alt-" + uuid.NewString()
		msg = []byte("From: " + from + "\r\n" +
			"To: " + email + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
			textBody + "\r\n\r\n" +
			"--" + boundary + "\r\n" +
			"Content-Type: text/html; charset=\"utf-8\"\r\n\r\n" +
			htmlBody + "\r\n\r\n" +
			"--" + boundary + "--\r\n")
	}
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{email}, msg)
}

func sendViaResend(email, subject, textBody, htmlBody, logTag string) error {
	apiKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	if apiKey == "" {
		return fmt.Errorf("resend is not configured")
	}

	fromEmail := strings.TrimSpace(os.Getenv("RESEND_FROM_EMAIL"))
	if fromEmail == "" {
		fromEmail = "payknot@notify.crea8r.xyz"
	}
	fromName := strings.TrimSpace(os.Getenv("RESEND_FROM_NAME"))
	if fromName == "" {
		fromName = "Payknot"
	}
	from := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{email},
		"subject": subject,
		"text":    textBody,
		"headers": map[string]string{
			"X-Payknot-Message-Type": logTag,
		},
	}
	if strings.TrimSpace(htmlBody) != "" {
		payload["html"] = htmlBody
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Payknot/1.0")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("resend send failed: %s", strings.TrimSpace(string(respBody)))
	}
	return nil
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
