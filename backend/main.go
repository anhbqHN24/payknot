package main

import (
	"log"
	"net/http"
	"solana_paywall/backend/api"
	"solana_paywall/backend/database"
	"solana_paywall/backend/middleware"
	"solana_paywall/backend/watcher"
)

func main() {
	database.Connect()
	database.ConnectRedis()
	watcher.Start()

	http.HandleFunc("/api/auth/register", api.Register)
	http.HandleFunc("/api/auth/login", api.Login)
	http.HandleFunc("/api/auth/google", api.GoogleLogin)
	http.HandleFunc("/api/auth/verify-email", api.VerifyEmail)
	http.HandleFunc("/api/auth/resend-verification", api.ResendVerification)
	http.HandleFunc("/api/auth/forgot-password", api.ForgotPassword)
	http.HandleFunc("/api/auth/reset-password", api.ResetPassword)
	http.HandleFunc("/api/auth/me", middleware.RequireAuthOrAgentKey(api.Me))
	http.HandleFunc("/api/auth/logout", api.Logout)

	http.HandleFunc("/api/events", middleware.RequireAuthOrAgentKey(api.EventsRoot))
	http.HandleFunc("/api/events/", middleware.RequireAuthOrAgentKey(api.EventsSubroutes))
	http.HandleFunc("/api/events/import/luma", middleware.RequireAuthOrAgentKey(api.ImportLumaEvent))
	http.HandleFunc("/api/agent-keys", middleware.RequireSessionAuth(api.AgentKeysRoot))
	http.HandleFunc("/api/agent-keys/revoke", middleware.RequireSessionAuth(api.RevokeAgentKey))
	http.HandleFunc("/api/agent/pats", middleware.RequireSessionAuth(api.AgentPATsRoot))
	http.HandleFunc("/api/agent/pats/revoke", middleware.RequireSessionAuth(api.RevokeAgentPAT))
	http.HandleFunc("/api/checkout/invoice", api.CreateCheckoutInvoice)
	http.HandleFunc("/api/checkout/cancel", api.CancelCheckoutInvoice)
	http.HandleFunc("/api/checkout/confirm", api.ConfirmCheckoutPayment)
	// Add verify payment api /w ratelimit protection middleware
	http.HandleFunc("/api/checkout/recheck", middleware.RateLimit(api.RecheckCheckoutPayment))
	http.HandleFunc("/api/checkout/manual-verify", middleware.RateLimit(api.ManualVerifyCheckoutPayment))
	http.HandleFunc("/api/checkout/participant-status", api.GetParticipantStatus)
	http.HandleFunc("/api/checkout/detect", middleware.RateLimit(api.DetectCheckoutPayment))
	http.HandleFunc("/api/checkout/status", api.GetCheckoutStatus)
	http.HandleFunc("/api/checkout/", api.GetCheckoutBySlug)

	// Agent nonce/JWT auth + settlement automation
	http.HandleFunc("/api/agent/auth/nonce", api.AgentAuthNonce)
	http.HandleFunc("/api/agent/auth/token", api.AgentAuthToken)
	http.HandleFunc("/api/agent/auth/pat", api.AgentAuthPAT)
	http.HandleFunc("/api/agent/auth/me", middleware.RequireAgentJWT(api.AgentAuthMe))
	http.HandleFunc("/api/agent/checkout/create", middleware.RequireAgentSignedSession(api.AgentCheckoutCreate))

	// Headless v1 API (server-owned session state)
	http.HandleFunc("/api/v1/payment-sessions", api.V1CreatePaymentSession)
	http.HandleFunc("/api/v1/payment-sessions/", api.V1PaymentSessionsSubroutes)

	log.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
