package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
)

func NewRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	novels := &novelsHandler{db: db}
	mux.HandleFunc("GET /api/v1/novels", novels.list)
	mux.HandleFunc("POST /api/v1/novels", novels.create)
	mux.HandleFunc("GET /api/v1/novels/{id}", novels.get)
	mux.HandleFunc("PUT /api/v1/novels/{id}", novels.update)
	mux.HandleFunc("DELETE /api/v1/novels/{id}", novels.delete)
	mux.HandleFunc("GET /api/v1/novels/{id}/full", novels.getFull)
	mux.HandleFunc("PUT /api/v1/novels/{id}/full", novels.putFull)
	mux.HandleFunc("POST /api/v1/novels/import", novels.importNovel)

	concepts := &conceptsHandler{db: db}
	mux.HandleFunc("GET /api/v1/novels/{novelId}/concepts", concepts.list)
	mux.HandleFunc("POST /api/v1/novels/{novelId}/concepts", concepts.create)
	mux.HandleFunc("GET /api/v1/novels/{novelId}/concepts/{id}", concepts.get)
	mux.HandleFunc("PUT /api/v1/novels/{novelId}/concepts/{id}", concepts.update)
	mux.HandleFunc("DELETE /api/v1/novels/{novelId}/concepts/{id}", concepts.delete)

	acts := &actsHandler{db: db}
	mux.HandleFunc("GET /api/v1/novels/{novelId}/acts", acts.list)
	mux.HandleFunc("POST /api/v1/novels/{novelId}/acts", acts.create)
	mux.HandleFunc("GET /api/v1/novels/{novelId}/acts/{id}", acts.get)
	mux.HandleFunc("PUT /api/v1/novels/{novelId}/acts/{id}", acts.update)
	mux.HandleFunc("DELETE /api/v1/novels/{novelId}/acts/{id}", acts.delete)

	chapters := &chaptersHandler{db: db}
	mux.HandleFunc("GET /api/v1/acts/{actId}/chapters", chapters.list)
	mux.HandleFunc("POST /api/v1/acts/{actId}/chapters", chapters.create)
	mux.HandleFunc("GET /api/v1/acts/{actId}/chapters/{id}", chapters.get)
	mux.HandleFunc("PUT /api/v1/acts/{actId}/chapters/{id}", chapters.update)
	mux.HandleFunc("DELETE /api/v1/acts/{actId}/chapters/{id}", chapters.delete)

	scenes := &scenesHandler{db: db}
	mux.HandleFunc("GET /api/v1/chapters/{chapterId}/scenes", scenes.list)
	mux.HandleFunc("POST /api/v1/chapters/{chapterId}/scenes", scenes.create)
	mux.HandleFunc("GET /api/v1/chapters/{chapterId}/scenes/{id}", scenes.get)
	mux.HandleFunc("PUT /api/v1/chapters/{chapterId}/scenes/{id}", scenes.update)
	mux.HandleFunc("DELETE /api/v1/chapters/{chapterId}/scenes/{id}", scenes.delete)

	templates := &templatesHandler{db: db}
	mux.HandleFunc("GET /api/v1/novels/{novelId}/concept-templates", templates.list)
	mux.HandleFunc("POST /api/v1/novels/{novelId}/concept-templates", templates.create)
	mux.HandleFunc("PUT /api/v1/novels/{novelId}/concept-templates/{id}", templates.update)
	mux.HandleFunc("DELETE /api/v1/novels/{novelId}/concept-templates/{id}", templates.delete)

	return corsMiddleware(mux)
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
