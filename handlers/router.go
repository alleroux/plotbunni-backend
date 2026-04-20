package handlers

import (
	"database/sql"
	"net/http"
)

func NewRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	novels := &novelsHandler{db: db}
	mux.HandleFunc("GET /novels", novels.list)
	mux.HandleFunc("POST /novels", novels.create)
	mux.HandleFunc("GET /novels/{id}", novels.get)
	mux.HandleFunc("PUT /novels/{id}", novels.update)
	mux.HandleFunc("DELETE /novels/{id}", novels.delete)

	return mux
}
