package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

type billingHandler struct {
	db *sql.DB
}

func newBillingHandler(db *sql.DB) *billingHandler {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	return &billingHandler{db: db}
}

// POST /api/v1/billing/checkout — creates a Stripe Checkout session and returns the URL.
func (h *billingHandler) checkout(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)

	var email string
	var stripeCustomerID *string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT email, stripe_customer_id FROM users WHERE id = $1`, userID).
		Scan(&email, &stripeCustomerID)
	if err != nil {
		internalError(w, err)
		return
	}

	customerID, err := h.ensureStripeCustomer(r.Context(), userID, email, stripeCustomerID)
	if err != nil {
		internalError(w, err)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(os.Getenv("STRIPE_PRICE_ID")), Quantity: stripe.Int64(1)},
		},
		SuccessURL: stripe.String(frontendURL + "/#/billing/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(frontendURL + "/#/billing"),
	}

	s, err := checkoutsession.New(params)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": s.URL})
}

// POST /api/v1/billing/portal — creates a Stripe Customer Portal session and returns the URL.
func (h *billingHandler) portal(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)

	var stripeCustomerID *string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT stripe_customer_id FROM users WHERE id = $1`, userID).Scan(&stripeCustomerID)
	if err != nil || stripeCustomerID == nil {
		http.Error(w, "no billing account found", http.StatusBadRequest)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*stripeCustomerID),
		ReturnURL: stripe.String(frontendURL + "/#/billing"),
	}

	s, err := session.New(params)
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": s.URL})
}

// POST /webhooks/stripe — public endpoint, verifies Stripe signature.
func (h *billingHandler) webhook(w http.ResponseWriter, r *http.Request) {
	// Limit body size before reading to prevent memory exhaustion attacks.
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), os.Getenv("STRIPE_WEBHOOK_SECRET"))
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	h.logStripeTransaction(r.Context(), event)

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		h.handleSubscriptionChanged(r.Context(), event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r.Context(), event)
	case "invoice.payment_failed":
		h.handlePaymentFailed(r.Context(), event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *billingHandler) handleSubscriptionChanged(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil || sub.Customer == nil {
		return
	}

	status := mapSubscriptionStatus(sub.Status)
	var endsAt *int64
	if sub.CancelAt > 0 {
		endsAt = &sub.CancelAt
	}

	tier := "pro"
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		tier = sub.Items.Data[0].Price.ID
	}

	// Never overwrite admin-granted free access via Stripe events.
	_, _ = h.db.ExecContext(ctx, `
		UPDATE users SET
			subscription_status  = $2,
			subscription_tier    = $3,
			subscription_ends_at = CASE WHEN $4::bigint IS NULL THEN NULL
			                            ELSE to_timestamp($4) END
		WHERE stripe_customer_id = $1 AND subscription_tier != 'free_grant'
	`, sub.Customer.ID, status, tier, endsAt)
}

func (h *billingHandler) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil || sub.Customer == nil {
		return
	}
	_, _ = h.db.ExecContext(ctx, `
		UPDATE users SET subscription_status = 'canceled', subscription_tier = NULL
		WHERE stripe_customer_id = $1 AND subscription_tier != 'free_grant'
	`, sub.Customer.ID)
}

func (h *billingHandler) handlePaymentFailed(ctx context.Context, event stripe.Event) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil || inv.Customer == nil {
		return
	}
	_, _ = h.db.ExecContext(ctx, `
		UPDATE users SET subscription_status = 'past_due'
		WHERE stripe_customer_id = $1 AND subscription_tier != 'free_grant'
	`, inv.Customer.ID)
}

func (h *billingHandler) logStripeTransaction(ctx context.Context, event stripe.Event) {
	var (
		userID         *string
		customerID     *string
		subscriptionID *string
		amountCents    *int64
		cur            *string
		status         *string
	)

	switch event.Type {
	case "invoice.payment_succeeded", "invoice.payment_failed":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err == nil && inv.Customer != nil {
			cid := inv.Customer.ID
			customerID = &cid
			if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil &&
				inv.Parent.SubscriptionDetails.Subscription != nil {
				sid := inv.Parent.SubscriptionDetails.Subscription.ID
				subscriptionID = &sid
			}
			amt := inv.AmountPaid
			amountCents = &amt
			c := string(inv.Currency)
			cur = &c
			st := string(inv.Status)
			status = &st
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err == nil && sub.Customer != nil {
			cid := sub.Customer.ID
			customerID = &cid
			sid := sub.ID
			subscriptionID = &sid
			st := string(sub.Status)
			status = &st
		}
	}

	if customerID != nil {
		uid, err := h.userIDByStripeCustomer(ctx, *customerID)
		if err == nil {
			userID = &uid
		}
	}

	raw, _ := json.Marshal(event)
	_, _ = h.db.ExecContext(ctx, `
		INSERT INTO stripe_transactions
			(user_id, stripe_event_id, stripe_customer_id, stripe_subscription_id,
			 event_type, amount_cents, currency, status, raw_event)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (stripe_event_id) DO NOTHING
	`, userID, event.ID, customerID, subscriptionID,
		string(event.Type), amountCents, cur, status, raw)
}

func (h *billingHandler) ensureStripeCustomer(ctx context.Context, userID, email string, existing *string) (string, error) {
	if existing != nil && *existing != "" {
		return *existing, nil
	}
	params := &stripe.CustomerParams{
		Email:    stripe.String(email),
		Metadata: map[string]string{"user_id": userID},
	}
	c, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	_, err = h.db.ExecContext(ctx, `UPDATE users SET stripe_customer_id = $1 WHERE id = $2`, c.ID, userID)
	return c.ID, err
}

func (h *billingHandler) userIDByStripeCustomer(ctx context.Context, customerID string) (string, error) {
	var id string
	return id, h.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE stripe_customer_id = $1`, customerID).Scan(&id)
}

func mapSubscriptionStatus(s stripe.SubscriptionStatus) string {
	switch s {
	case stripe.SubscriptionStatusActive:
		return "active"
	case stripe.SubscriptionStatusTrialing:
		return "trialing"
	case stripe.SubscriptionStatusPastDue:
		return "past_due"
	case stripe.SubscriptionStatusCanceled:
		return "canceled"
	default:
		return string(s)
	}
}
