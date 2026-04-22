package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type scenesHandler struct {
	db *sql.DB
}

func (h *scenesHandler) list(w http.ResponseWriter, r *http.Request) {
	chapterId := r.PathValue("chapterId")
	userID, _ := getUserID(r)

	if ok, err := checkChapterOwner(r.Context(), h.db, chapterId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT s.id, s.name, s.synopsis, s.content, s.tags, s.auto_update_context,
		       s.position, s.extra_data, s.created_at, s.updated_at,
		       COALESCE(array_agg(sc.concept_id) FILTER (WHERE sc.concept_id IS NOT NULL), '{}') AS concept_ids
		FROM scenes s
		LEFT JOIN scene_concepts sc ON sc.scene_id = s.id
		WHERE s.chapter_id = $1
		GROUP BY s.id
		ORDER BY s.position
	`, chapterId)
	if err != nil {
		internalError(w, err)
		return
	}
	defer rows.Close()

	results := []map[string]any{}
	for rows.Next() {
		s, err := scanScene(rows)
		if err != nil {
			internalError(w, err)
			return
		}
		results = append(results, s)
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *scenesHandler) create(w http.ResponseWriter, r *http.Request) {
	chapterId := r.PathValue("chapterId")
	userID, _ := getUserID(r)

	if ok, err := checkChapterOwner(r.Context(), h.db, chapterId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		ID                *string         `json:"id"`
		Name              string          `json:"name"`
		Synopsis          *string         `json:"synopsis"`
		Content           *string         `json:"content"`
		Tags              []string        `json:"tags"`
		AutoUpdateContext bool            `json:"auto_update_context"`
		Position          int             `json:"position"`
		ConceptIDs        []string        `json:"concept_ids"`
		ExtraData         json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		internalError(w, err)
		return
	}
	defer tx.Rollback()

	var id string
	if body.ID != nil {
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO scenes (id, chapter_id, name, synopsis, content, tags, auto_update_context, position, extra_data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id
		`, body.ID, chapterId, body.Name, body.Synopsis, body.Content,
			pqArray(body.Tags), body.AutoUpdateContext, body.Position, extra).Scan(&id)
	} else {
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO scenes (chapter_id, name, synopsis, content, tags, auto_update_context, position, extra_data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
		`, chapterId, body.Name, body.Synopsis, body.Content,
			pqArray(body.Tags), body.AutoUpdateContext, body.Position, extra).Scan(&id)
	}
	if err != nil {
		internalError(w, err)
		return
	}

	if err := upsertSceneConcepts(r.Context(), tx, id, body.ConceptIDs); err != nil {
		internalError(w, err)
		return
	}

	if err := tx.Commit(); err != nil {
		internalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *scenesHandler) get(w http.ResponseWriter, r *http.Request) {
	id, chapterId := r.PathValue("id"), r.PathValue("chapterId")
	userID, _ := getUserID(r)

	if ok, err := checkChapterOwner(r.Context(), h.db, chapterId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	row := h.db.QueryRowContext(r.Context(), `
		SELECT s.id, s.name, s.synopsis, s.content, s.tags, s.auto_update_context,
		       s.position, s.extra_data, s.created_at, s.updated_at,
		       COALESCE(array_agg(sc.concept_id) FILTER (WHERE sc.concept_id IS NOT NULL), '{}') AS concept_ids
		FROM scenes s
		LEFT JOIN scene_concepts sc ON sc.scene_id = s.id
		WHERE s.id = $1 AND s.chapter_id = $2
		GROUP BY s.id
	`, id, chapterId)

	s, err := scanScene(row)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *scenesHandler) update(w http.ResponseWriter, r *http.Request) {
	id, chapterId := r.PathValue("id"), r.PathValue("chapterId")
	userID, _ := getUserID(r)

	if ok, err := checkChapterOwner(r.Context(), h.db, chapterId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var body struct {
		Name              *string         `json:"name"`
		Synopsis          *string         `json:"synopsis"`
		Content           *string         `json:"content"`
		Tags              []string        `json:"tags"`
		AutoUpdateContext *bool           `json:"auto_update_context"`
		Position          *int            `json:"position"`
		ConceptIDs        []string        `json:"concept_ids"`
		ExtraData         json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	extra := normalizeExtra(body.ExtraData)

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		internalError(w, err)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(r.Context(), `
		UPDATE scenes SET
			name               = COALESCE($3, name),
			synopsis           = COALESCE($4, synopsis),
			content            = COALESCE($5, content),
			tags               = COALESCE($6, tags),
			auto_update_context = COALESCE($7, auto_update_context),
			position           = COALESCE($8, position),
			extra_data         = extra_data || $9,
			updated_at         = NOW()
		WHERE id = $1 AND chapter_id = $2
	`, id, chapterId, body.Name, body.Synopsis, body.Content,
		pqArray(body.Tags), body.AutoUpdateContext, body.Position, extra)
	if err != nil {
		internalError(w, err)
		return
	}

	if body.ConceptIDs != nil {
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM scene_concepts WHERE scene_id = $1`, id); err != nil {
			internalError(w, err)
			return
		}
		if err := upsertSceneConcepts(r.Context(), tx, id, body.ConceptIDs); err != nil {
			internalError(w, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *scenesHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, chapterId := r.PathValue("id"), r.PathValue("chapterId")
	userID, _ := getUserID(r)

	if ok, err := checkChapterOwner(r.Context(), h.db, chapterId, userID); err != nil {
		internalError(w, err)
		return
	} else if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM scenes WHERE id = $1 AND chapter_id = $2`, id, chapterId); err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scanScene(s scanner) (map[string]any, error) {
	var id, name, createdAt, updatedAt string
	var synopsis, content *string
	var tags, conceptIds []string
	var autoUpdateContext bool
	var position int
	var extra []byte

	err := s.Scan(&id, &name, &synopsis, &content, (*pqStringArray)(&tags), &autoUpdateContext,
		&position, &extra, &createdAt, &updatedAt, (*pqStringArray)(&conceptIds))
	if err != nil {
		return nil, err
	}

	var extraMap map[string]any
	if len(extra) > 0 {
		_ = json.Unmarshal(extra, &extraMap)
	}

	return map[string]any{
		"id": id, "name": name, "synopsis": synopsis, "content": content,
		"tags": tags, "auto_update_context": autoUpdateContext,
		"position": position, "concept_ids": conceptIds,
		"extra_data": extraMap, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}
