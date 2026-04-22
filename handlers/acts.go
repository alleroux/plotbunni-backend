package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type actsHandler struct {
	db *sql.DB
}

func (h *actsHandler) list(w http.ResponseWriter, r *http.Request) {
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
		SELECT id, name, position, extra_data, created_at, updated_at
		FROM acts WHERE novel_id = $1 ORDER BY position
	`, novelId)
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()

	results := []map[string]any{}
	for rows.Next() {
		a, err := scanAct(rows)
		if err != nil {
			internalError(w, err)
			return
		}
		results = append(results, a)
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *actsHandler) create(w http.ResponseWriter, r *http.Request) {
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
			INSERT INTO acts (id, novel_id, name, position, extra_data)
			VALUES ($1, $2, $3, $4, $5) RETURNING id
		`, body.ID, novelId, body.Name, body.Position, extra).Scan(&id)
	} else {
		err = h.db.QueryRowContext(r.Context(), `
			INSERT INTO acts (novel_id, name, position, extra_data)
			VALUES ($1, $2, $3, $4) RETURNING id
		`, novelId, body.Name, body.Position, extra).Scan(&id)
	}
	if err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *actsHandler) get(w http.ResponseWriter, r *http.Request) {
	id, novelId := r.PathValue("id"), r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	row := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, position, extra_data, created_at, updated_at
		FROM acts WHERE id = $1 AND novel_id = $2
	`, id, novelId)

	a, err := scanAct(row)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *actsHandler) update(w http.ResponseWriter, r *http.Request) {
	id, novelId := r.PathValue("id"), r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
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
		UPDATE acts SET
			name       = COALESCE($3, name),
			position   = COALESCE($4, position),
			extra_data = extra_data || $5,
			updated_at = NOW()
		WHERE id = $1 AND novel_id = $2
	`, id, novelId, body.Name, body.Position, extra)
	if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *actsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, novelId := r.PathValue("id"), r.PathValue("novelId")
	userID, _ := getUserID(r)

	if ok, err := checkNovelOwner(r.Context(), h.db, novelId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM acts WHERE id = $1 AND novel_id = $2`, id, novelId); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scanAct(s scanner) (map[string]any, error) {
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
