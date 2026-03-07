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
	http.HandleFunc("/api/auth/me", middleware.RequireAuth(api.Me))
	http.HandleFunc("/api/auth/logout", api.Logout)

	http.HandleFunc("/api/events", middleware.RequireAuth(api.EventsRoot))
	http.HandleFunc("/api/events/", middleware.RequireAuth(api.EventsSubroutes))
	http.HandleFunc("/api/checkouts/", middleware.RequireAuth(api.CheckoutsSubroutes))
	http.HandleFunc("/api/checkout/invoice", api.CreateCheckoutInvoice)
	http.HandleFunc("/api/checkout/cancel", api.CancelCheckoutInvoice)
	http.HandleFunc("/api/checkout/confirm", api.ConfirmCheckoutPayment)
	// Add verify payment api /w ratelimit protection middleware
	http.HandleFunc("/api/checkout/recheck", middleware.RateLimit(api.RecheckCheckoutPayment))
	http.HandleFunc("/api/checkout/manual-verify", middleware.RateLimit(api.ManualVerifyCheckoutPayment))
	http.HandleFunc("/api/checkout/status", api.GetCheckoutStatus)
	http.HandleFunc("/api/checkout/", api.GetCheckoutBySlug)
	http.HandleFunc("/api/invite/validate", api.ValidateInviteCode)
	http.HandleFunc("/api/invite/status", api.GetInviteStatus)

	log.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
