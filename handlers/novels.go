package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/alleroux/plotbunni-backend/db"
)

type novelsHandler struct {
	db *sql.DB
}

func (h *novelsHandler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, synopsis, cover_image, author, updated_at
		FROM novels ORDER BY updated_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Synopsis   *string `json:"synopsis"`
		CoverImage *string `json:"cover_image"`
		Author     *string `json:"author"`
		UpdatedAt  string  `json:"updated_at"`
	}

	results := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Name, &r.Synopsis, &r.CoverImage, &r.Author, &r.UpdatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, r)
	}

	writeJSON(w, http.StatusOK, results)
}

func (h *novelsHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string          `json:"name"`
		Synopsis   *string         `json:"synopsis"`
		CoverImage *string         `json:"cover_image"`
		Author     *string         `json:"author"`
		ExtraData  json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	var id string
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO novels (name, synopsis, cover_image, author, extra_data)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, body.Name, body.Synopsis, body.CoverImage, body.Author, extra).Scan(&id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *novelsHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	novel, err := db.GetNovel(r.Context(), h.db, id)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, novel)
}

func (h *novelsHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name       *string         `json:"name"`
		Synopsis   *string         `json:"synopsis"`
		CoverImage *string         `json:"cover_image"`
		Author     *string         `json:"author"`
		POV        *string         `json:"pov"`
		Genre      *string         `json:"genre"`
		TimePeriod *string         `json:"time_period"`
		Audience   *string         `json:"audience"`
		Tone       *string         `json:"tone"`
		Themes     []string        `json:"themes"`
		ExtraData  json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE novels SET
			name        = COALESCE($2, name),
			synopsis    = COALESCE($3, synopsis),
			cover_image = COALESCE($4, cover_image),
			author      = COALESCE($5, author),
			pov         = COALESCE($6, pov),
			genre       = COALESCE($7, genre),
			time_period = COALESCE($8, time_period),
			audience    = COALESCE($9, audience),
			tone        = COALESCE($10, tone),
			themes      = COALESCE($11, themes),
			extra_data  = extra_data || $12,
			updated_at  = NOW()
		WHERE id = $1
	`, id, body.Name, body.Synopsis, body.CoverImage, body.Author,
		body.POV, body.Genre, body.TimePeriod, body.Audience, body.Tone,
		pqArray(body.Themes), extra)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *novelsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM novels WHERE id = $1`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getFull returns the complete novel in the IndexedDB shape the frontend expects.
// This makes the frontend adapter resilient to upstream data model changes: new
// top-level fields land in extra_data and are passed through transparently.
func (h *novelsHandler) getFull(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	full, err := db.GetFullNovel(r.Context(), h.db, id)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, full)
}

// importNovel accepts a full novel payload in the IndexedDB shape and persists it.
func (h *novelsHandler) importNovel(w http.ResponseWriter, r *http.Request) {
	var payload db.FullNovel
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := db.ImportFullNovel(r.Context(), h.db, &payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}
