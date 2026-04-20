package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
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
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Synopsis    *string `json:"synopsis"`
		CoverImage  *string `json:"cover_image"`
		Author      *string `json:"author"`
		UpdatedAt   string  `json:"updated_at"`
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *novelsHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string  `json:"name"`
		Synopsis   *string `json:"synopsis"`
		CoverImage *string `json:"cover_image"`
		Author     *string `json:"author"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var id string
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO novels (name, synopsis, cover_image, author)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, body.Name, body.Synopsis, body.CoverImage, body.Author).Scan(&id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *novelsHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var result struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Synopsis   *string  `json:"synopsis"`
		CoverImage *string  `json:"cover_image"`
		Author     *string  `json:"author"`
		POV        *string  `json:"pov"`
		Genre      *string  `json:"genre"`
		TimePeriod *string  `json:"time_period"`
		Audience   *string  `json:"audience"`
		Tone       *string  `json:"tone"`
		UpdatedAt  string   `json:"updated_at"`
		CreatedAt  string   `json:"created_at"`
	}

	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, synopsis, cover_image, author, pov, genre, time_period, audience, tone, updated_at, created_at
		FROM novels WHERE id = $1
	`, id).Scan(
		&result.ID, &result.Name, &result.Synopsis, &result.CoverImage,
		&result.Author, &result.POV, &result.Genre, &result.TimePeriod,
		&result.Audience, &result.Tone, &result.UpdatedAt, &result.CreatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *novelsHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name       *string `json:"name"`
		Synopsis   *string `json:"synopsis"`
		CoverImage *string `json:"cover_image"`
		Author     *string `json:"author"`
		POV        *string `json:"pov"`
		Genre      *string `json:"genre"`
		TimePeriod *string `json:"time_period"`
		Audience   *string `json:"audience"`
		Tone       *string `json:"tone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
			updated_at  = NOW()
		WHERE id = $1
	`, id, body.Name, body.Synopsis, body.CoverImage, body.Author,
		body.POV, body.Genre, body.TimePeriod, body.Audience, body.Tone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *novelsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(), `DELETE FROM novels WHERE id = $1`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
