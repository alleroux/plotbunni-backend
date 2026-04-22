package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

type adminHandler struct {
	db *sql.DB
}

// adminMiddleware checks that the authenticated user has is_admin = true.
// Must run after authMiddleware (userID already in context).
func adminMiddleware(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserID(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var isAdmin bool
		err := db.QueryRowContext(r.Context(), `SELECT is_admin FROM users WHERE id = $1`, userID).Scan(&isAdmin)
		if err != nil || !isAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GET /api/v1/admin/users?page=1&q=email
func (h *adminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 50
	q := "%" + r.URL.Query().Get("q") + "%"

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, email, name, avatar_url, is_admin,
		       subscription_status, subscription_tier, subscription_ends_at,
		       stripe_customer_id, created_at
		FROM users
		WHERE email ILIKE $1 OR name ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, q, pageSize, (page-1)*pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type userRow struct {
		ID                 string  `json:"id"`
		Email              string  `json:"email"`
		Name               string  `json:"name"`
		AvatarURL          string  `json:"avatar_url"`
		IsAdmin            bool    `json:"is_admin"`
		SubscriptionStatus string  `json:"subscription_status"`
		SubscriptionTier   *string `json:"subscription_tier"`
		SubscriptionEndsAt *string `json:"subscription_ends_at"`
		StripeCustomerID   *string `json:"stripe_customer_id"`
		CreatedAt          string  `json:"created_at"`
	}

	results := []userRow{}
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.IsAdmin,
			&u.SubscriptionStatus, &u.SubscriptionTier, &u.SubscriptionEndsAt,
			&u.StripeCustomerID, &u.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, u)
	}

	writeJSON(w, http.StatusOK, results)
}

// PUT /api/v1/admin/users/{id}/subscription
// Body: {"action": "grant_free" | "revoke_free" | "set_admin" | "revoke_admin"}
func (h *adminHandler) updateUserSubscription(w http.ResponseWriter, r *http.Request) {
	adminID, _ := getUserID(r)
	targetID := r.PathValue("id")

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Capture before state for audit log
	var before struct {
		SubscriptionStatus string  `json:"subscription_status"`
		SubscriptionTier   *string `json:"subscription_tier"`
		IsAdmin            bool    `json:"is_admin"`
	}
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT subscription_status, subscription_tier, is_admin FROM users WHERE id = $1`, targetID).
		Scan(&before.SubscriptionStatus, &before.SubscriptionTier, &before.IsAdmin)

	var execErr error
	switch body.Action {
	case "grant_free":
		_, execErr = h.db.ExecContext(r.Context(), `
			UPDATE users SET subscription_status = 'active', subscription_tier = 'free_grant',
			                 subscription_ends_at = NULL
			WHERE id = $1`, targetID)
	case "revoke_free":
		_, execErr = h.db.ExecContext(r.Context(), `
			UPDATE users SET subscription_status = 'free', subscription_tier = NULL,
			                 subscription_ends_at = NULL
			WHERE id = $1`, targetID)
	case "set_admin":
		_, execErr = h.db.ExecContext(r.Context(), `UPDATE users SET is_admin = TRUE WHERE id = $1`, targetID)
	case "revoke_admin":
		_, execErr = h.db.ExecContext(r.Context(), `UPDATE users SET is_admin = FALSE WHERE id = $1`, targetID)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	if execErr != nil {
		http.Error(w, execErr.Error(), http.StatusInternalServerError)
		return
	}

	// Capture after state for audit log
	var after struct {
		SubscriptionStatus string  `json:"subscription_status"`
		SubscriptionTier   *string `json:"subscription_tier"`
		IsAdmin            bool    `json:"is_admin"`
	}
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT subscription_status, subscription_tier, is_admin FROM users WHERE id = $1`, targetID).
		Scan(&after.SubscriptionStatus, &after.SubscriptionTier, &after.IsAdmin)

	details, _ := json.Marshal(map[string]any{"action": body.Action, "before": before, "after": after})
	_, _ = h.db.ExecContext(r.Context(), `
		INSERT INTO admin_logs (admin_user_id, action, target_user_id, details)
		VALUES ($1, $2, $3, $4)
	`, adminID, body.Action, targetID, details)

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/admin/logs?page=1
func (h *adminHandler) listLogs(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 100

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT l.id, l.action, l.details, l.created_at,
		       a.email AS admin_email,
		       t.email AS target_email
		FROM admin_logs l
		JOIN users a ON a.id = l.admin_user_id
		LEFT JOIN users t ON t.id = l.target_user_id
		ORDER BY l.created_at DESC
		LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type logRow struct {
		ID          string          `json:"id"`
		Action      string          `json:"action"`
		Details     json.RawMessage `json:"details"`
		CreatedAt   string          `json:"created_at"`
		AdminEmail  string          `json:"admin_email"`
		TargetEmail *string         `json:"target_email"`
	}

	results := []logRow{}
	for rows.Next() {
		var l logRow
		var details []byte
		if err := rows.Scan(&l.ID, &l.Action, &details, &l.CreatedAt, &l.AdminEmail, &l.TargetEmail); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		l.Details = json.RawMessage(details)
		results = append(results, l)
	}

	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/admin/transactions?page=1&user_id=...
func (h *adminHandler) listTransactions(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 100
	filterUserID := r.URL.Query().Get("user_id")

	var (
		rows *sql.Rows
		err  error
	)
	if filterUserID != "" {
		rows, err = h.db.QueryContext(r.Context(), `
			SELECT t.id, t.stripe_event_id, t.event_type, t.amount_cents, t.currency,
			       t.status, t.stripe_customer_id, t.stripe_subscription_id, t.created_at,
			       u.email
			FROM stripe_transactions t
			LEFT JOIN users u ON u.id = t.user_id
			WHERE t.user_id = $1
			ORDER BY t.created_at DESC
			LIMIT $2 OFFSET $3
		`, filterUserID, pageSize, (page-1)*pageSize)
	} else {
		rows, err = h.db.QueryContext(r.Context(), `
			SELECT t.id, t.stripe_event_id, t.event_type, t.amount_cents, t.currency,
			       t.status, t.stripe_customer_id, t.stripe_subscription_id, t.created_at,
			       u.email
			FROM stripe_transactions t
			LEFT JOIN users u ON u.id = t.user_id
			ORDER BY t.created_at DESC
			LIMIT $1 OFFSET $2
		`, pageSize, (page-1)*pageSize)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type txRow struct {
		ID                    string  `json:"id"`
		StripeEventID         string  `json:"stripe_event_id"`
		EventType             string  `json:"event_type"`
		AmountCents           *int64  `json:"amount_cents"`
		Currency              *string `json:"currency"`
		Status                *string `json:"status"`
		StripeCustomerID      *string `json:"stripe_customer_id"`
		StripeSubscriptionID  *string `json:"stripe_subscription_id"`
		CreatedAt             string  `json:"created_at"`
		UserEmail             *string `json:"user_email"`
	}

	results := []txRow{}
	for rows.Next() {
		var t txRow
		if err := rows.Scan(&t.ID, &t.StripeEventID, &t.EventType, &t.AmountCents, &t.Currency,
			&t.Status, &t.StripeCustomerID, &t.StripeSubscriptionID, &t.CreatedAt, &t.UserEmail); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, t)
	}

	writeJSON(w, http.StatusOK, results)
}
