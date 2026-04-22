package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
)

func NewRouter(db *sql.DB) http.Handler {
	// Public mux: auth + health + Stripe webhook (no JWT)
	public := http.NewServeMux()
	auth := newAuthHandler(db)
	billing := newBillingHandler(db)

	// Stricter rate limiters for endpoints that trigger external side-effects.
	// Auth: 5 attempts per minute per IP (burst 5).
	authLimiter := newRateLimiter(5.0/60, 5)
	// Billing: 3 checkout/portal requests per minute per IP (burst 3).
	billingLimiter := newRateLimiter(3.0/60, 3)

	public.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	public.Handle("GET /auth/google/login", authLimiter.wrap(http.HandlerFunc(auth.login)))
	public.HandleFunc("GET /auth/google/callback", auth.callback)
	public.HandleFunc("POST /webhooks/stripe", billing.webhook) // body size limited inside handler

	// Protected mux: all API routes require a valid JWT
	api := http.NewServeMux()

	api.HandleFunc("GET /api/v1/me", auth.me)

	// Billing
	api.Handle("POST /api/v1/billing/checkout", billingLimiter.wrap(http.HandlerFunc(billing.checkout)))
	api.Handle("POST /api/v1/billing/portal", billingLimiter.wrap(http.HandlerFunc(billing.portal)))

	// Admin — additional adminMiddleware check applied in the combined handler below
	adm := &adminHandler{db: db}
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /api/v1/admin/users", adm.listUsers)
	adminMux.HandleFunc("PUT /api/v1/admin/users/{id}/subscription", adm.updateUserSubscription)
	adminMux.HandleFunc("GET /api/v1/admin/logs", adm.listLogs)
	adminMux.HandleFunc("GET /api/v1/admin/transactions", adm.listTransactions)

	novels := &novelsHandler{db: db}
	api.HandleFunc("GET /api/v1/novels", novels.list)
	api.HandleFunc("POST /api/v1/novels", novels.create)
	api.HandleFunc("GET /api/v1/novels/{id}", novels.get)
	api.HandleFunc("PUT /api/v1/novels/{id}", novels.update)
	api.HandleFunc("DELETE /api/v1/novels/{id}", novels.delete)
	api.HandleFunc("GET /api/v1/novels/{id}/full", novels.getFull)
	api.HandleFunc("PUT /api/v1/novels/{id}/full", novels.putFull)
	api.HandleFunc("POST /api/v1/novels/import", novels.importNovel)

	concepts := &conceptsHandler{db: db}
	api.HandleFunc("GET /api/v1/novels/{novelId}/concepts", concepts.list)
	api.HandleFunc("POST /api/v1/novels/{novelId}/concepts", concepts.create)
	api.HandleFunc("GET /api/v1/novels/{novelId}/concepts/{id}", concepts.get)
	api.HandleFunc("PUT /api/v1/novels/{novelId}/concepts/{id}", concepts.update)
	api.HandleFunc("DELETE /api/v1/novels/{novelId}/concepts/{id}", concepts.delete)

	acts := &actsHandler{db: db}
	api.HandleFunc("GET /api/v1/novels/{novelId}/acts", acts.list)
	api.HandleFunc("POST /api/v1/novels/{novelId}/acts", acts.create)
	api.HandleFunc("GET /api/v1/novels/{novelId}/acts/{id}", acts.get)
	api.HandleFunc("PUT /api/v1/novels/{novelId}/acts/{id}", acts.update)
	api.HandleFunc("DELETE /api/v1/novels/{novelId}/acts/{id}", acts.delete)

	chapters := &chaptersHandler{db: db}
	api.HandleFunc("GET /api/v1/acts/{actId}/chapters", chapters.list)
	api.HandleFunc("POST /api/v1/acts/{actId}/chapters", chapters.create)
	api.HandleFunc("GET /api/v1/acts/{actId}/chapters/{id}", chapters.get)
	api.HandleFunc("PUT /api/v1/acts/{actId}/chapters/{id}", chapters.update)
	api.HandleFunc("DELETE /api/v1/acts/{actId}/chapters/{id}", chapters.delete)

	scenes := &scenesHandler{db: db}
	api.HandleFunc("GET /api/v1/chapters/{chapterId}/scenes", scenes.list)
	api.HandleFunc("POST /api/v1/chapters/{chapterId}/scenes", scenes.create)
	api.HandleFunc("GET /api/v1/chapters/{chapterId}/scenes/{id}", scenes.get)
	api.HandleFunc("PUT /api/v1/chapters/{chapterId}/scenes/{id}", scenes.update)
	api.HandleFunc("DELETE /api/v1/chapters/{chapterId}/scenes/{id}", scenes.delete)

	templates := &templatesHandler{db: db}
	api.HandleFunc("GET /api/v1/novels/{novelId}/concept-templates", templates.list)
	api.HandleFunc("POST /api/v1/novels/{novelId}/concept-templates", templates.create)
	api.HandleFunc("PUT /api/v1/novels/{novelId}/concept-templates/{id}", templates.update)
	api.HandleFunc("DELETE /api/v1/novels/{novelId}/concept-templates/{id}", templates.delete)

	// Combine: public → no auth; /api/v1/admin/* → auth + admin check; /api/* → auth only
	combined := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/admin/"):
			authMiddleware(adminMiddleware(db, adminMux)).ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/"):
			authMiddleware(api).ServeHTTP(w, r)
		default:
			public.ServeHTTP(w, r)
		}
	})

	// Global middleware stack (outermost first):
	//   1. CORS
	//   2. 10 MB body size limit (prevents memory exhaustion on all non-webhook endpoints;
	//      the webhook endpoint enforces its own tighter 64 KB limit internally)
	return corsMiddleware(maxBytesMiddleware(combined))
}

// maxBytesMiddleware limits request bodies to 10 MB to prevent memory exhaustion.
func maxBytesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	allowed := allowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowed(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigins() []string {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return []string{"http://localhost:5173", "http://localhost:4173"}
	}
	return strings.Split(raw, ",")
}

func isAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return true
		}
	}
	return false
}
