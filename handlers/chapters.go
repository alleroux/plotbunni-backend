package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type chaptersHandler struct {
	db *sql.DB
}

func (h *chaptersHandler) list(w http.ResponseWriter, r *http.Request) {
	actId := r.PathValue("actId")
	userID, _ := getUserID(r)

	if ok, err := checkActOwner(r.Context(), h.db, actId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, position, extra_data, created_at, updated_at
		FROM chapters WHERE act_id = $1 ORDER BY position
	`, actId)
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()

	results := []map[string]any{}
	for rows.Next() {
		c, err := scanChapter(rows)
		if err != nil {
			internalError(w, err)
			return
		}
		results = append(results, c)
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *chaptersHandler) create(w http.ResponseWriter, r *http.Request) {
	actId := r.PathValue("actId")
	userID, _ := getUserID(r)

	if ok, err := checkActOwner(r.Context(), h.db, actId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		ID        *string         `json:"id"`
		Name      string          `json:"name"`
		Position  int             `json:"position"`
		ExtraData json.RawMessage `json:"extra_data"`
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
			INSERT INTO chapters (id, act_id, name, position, extra_data)
			VALUES ($1, $2, $3, $4, $5) RETURNING id
		`, body.ID, actId, body.Name, body.Position, extra).Scan(&id)
	} else {
		err = h.db.QueryRowContext(r.Context(), `
			INSERT INTO chapters (act_id, name, position, extra_data)
			VALUES ($1, $2, $3, $4) RETURNING id
		`, actId, body.Name, body.Position, extra).Scan(&id)
	}
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *chaptersHandler) get(w http.ResponseWriter, r *http.Request) {
	id, actId := r.PathValue("id"), r.PathValue("actId")
	userID, _ := getUserID(r)

	if ok, err := checkActOwner(r.Context(), h.db, actId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	row := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, position, extra_data, created_at, updated_at
		FROM chapters WHERE id = $1 AND act_id = $2
	`, id, actId)

	c, err := scanChapter(row)
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

func (h *chaptersHandler) update(w http.ResponseWriter, r *http.Request) {
	id, actId := r.PathValue("id"), r.PathValue("actId")
	userID, _ := getUserID(r)

	if ok, err := checkActOwner(r.Context(), h.db, actId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		Name      *string         `json:"name"`
		Position  *int            `json:"position"`
		ExtraData json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE chapters SET
			name       = COALESCE($3, name),
			position   = COALESCE($4, position),
			extra_data = extra_data || $5,
			updated_at = NOW()
		WHERE id = $1 AND act_id = $2
	`, id, actId, body.Name, body.Position, extra)
	if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *chaptersHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, actId := r.PathValue("id"), r.PathValue("actId")
	userID, _ := getUserID(r)

	if ok, err := checkActOwner(r.Context(), h.db, actId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM chapters WHERE id = $1 AND act_id = $2`, id, actId); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scanChapter(s scanner) (map[string]any, error) {
	var id, name, createdAt, updatedAt string
	var position int
	var extra []byte

	if err := s.Scan(&id, &name, &position, &extra, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var extraMap map[string]any
	if len(extra) > 0 {
		_ = json.Unmarshal(extra, &extraMap)
	}
	return map[string]any{
		"id": id, "name": name, "position": position,
		"extra_data": extraMap, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}
