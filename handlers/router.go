package handlers

import (
	"database/sql"
	"net/http"
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

	return mux
}
