package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type conceptsHandler struct {
	db *sql.DB
}

func (h *conceptsHandler) list(w http.ResponseWriter, r *http.Request) {
	novelId := r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, type, name, aliases, tags, description, notes, priority, image, extra_data, created_at, updated_at
		FROM concepts WHERE novel_id = $1 ORDER BY priority DESC, name
	`, novelId)
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()

	results := []map[string]any{}
	for rows.Next() {
		c, err := scanConcept(rows)
		if err != nil {
			internalError(w, err)
			return
		}
		results = append(results, c)
	}

	writeJSON(w, http.StatusOK, results)
}

func (h *conceptsHandler) create(w http.ResponseWriter, r *http.Request) {
	novelId := r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		ID          *string         `json:"id"`
		Type        string          `json:"type"`
		Name        string          `json:"name"`
		Aliases     []string        `json:"aliases"`
		Tags        []string        `json:"tags"`
		Description *string         `json:"description"`
		Notes       *string         `json:"notes"`
		Priority    int             `json:"priority"`
		Image       *string         `json:"image"`
		ExtraData   json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	var id string
	var err error
	if body.ID != nil {
		err = h.db.QueryRowContext(r.Context(), `
			INSERT INTO concepts (id, novel_id, type, name, aliases, tags, description, notes, priority, image, extra_data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, body.ID, novelId, body.Type, body.Name, pqArray(body.Aliases), pqArray(body.Tags),
			body.Description, body.Notes, body.Priority, body.Image, extra).Scan(&id)
	} else {
		err = h.db.QueryRowContext(r.Context(), `
			INSERT INTO concepts (novel_id, type, name, aliases, tags, description, notes, priority, image, extra_data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id
		`, novelId, body.Type, body.Name, pqArray(body.Aliases), pqArray(body.Tags),
			body.Description, body.Notes, body.Priority, body.Image, extra).Scan(&id)
	}
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *conceptsHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	novelId := r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	row := h.db.QueryRowContext(r.Context(), `
		SELECT id, type, name, aliases, tags, description, notes, priority, image, extra_data, created_at, updated_at
		FROM concepts WHERE id = $1 AND novel_id = $2
	`, id, novelId)

	c, err := scanConcept(row)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *conceptsHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	novelId := r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		Name        *string         `json:"name"`
		Type        *string         `json:"type"`
		Aliases     []string        `json:"aliases"`
		Tags        []string        `json:"tags"`
		Description *string         `json:"description"`
		Notes       *string         `json:"notes"`
		Priority    *int            `json:"priority"`
		Image       *string         `json:"image"`
		ExtraData   json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE concepts SET
			name        = COALESCE($3, name),
			type        = COALESCE($4, type),
			aliases     = COALESCE($5, aliases),
			tags        = COALESCE($6, tags),
			description = COALESCE($7, description),
			notes       = COALESCE($8, notes),
			priority    = COALESCE($9, priority),
			image       = COALESCE($10, image),
			extra_data  = extra_data || $11,
			updated_at  = NOW()
		WHERE id = $1 AND novel_id = $2
	`, id, novelId, body.Name, body.Type, pqArray(body.Aliases), pqArray(body.Tags),
		body.Description, body.Notes, body.Priority, body.Image, extra)
	if err != nil {
		internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *conceptsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	novelId := r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM concepts WHERE id = $1 AND novel_id = $2`, id, novelId); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConcept(s scanner) (map[string]any, error) {
	var (
		id, cType, name      string
		aliases, tags        []string
		description, notes, image *string
		priority             int
		extra                []byte
		createdAt, updatedAt string
	)
	err := s.Scan(&id, &cType, &name, (*pqStringArray)(&aliases), (*pqStringArray)(&tags),
		&description, &notes, &priority, &image, &extra, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	var extraMap map[string]any
	if len(extra) > 0 {
		_ = json.Unmarshal(extra, &extraMap)
	}

	return map[string]any{
		"id": id, "type": cType, "name": name,
		"aliases": aliases, "tags": tags,
		"description": description, "notes": notes,
		"priority": priority, "image": image,
		"extra_data": extraMap,
		"created_at": createdAt, "updated_at": updatedAt,
	}, nil
}
