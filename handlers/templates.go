package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type templatesHandler struct {
	db *sql.DB
}

func (h *templatesHandler) list(w http.ResponseWriter, r *http.Request) {
	novelId := r.PathValue("novelId")

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, template_data, is_default, extra_data, created_at, updated_at
		FROM concept_templates WHERE novel_id = $1 ORDER BY name
	`, novelId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []map[string]any{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, t)
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *templatesHandler) create(w http.ResponseWriter, r *http.Request) {
	novelId := r.PathValue("novelId")

	var body struct {
		ID           *string         `json:"id"`
		Name         string          `json:"name"`
		TemplateData json.RawMessage `json:"template_data"`
		IsDefault    bool            `json:"is_default"`
		ExtraData    json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tmpl := normalizeExtra(body.TemplateData)
	extra := normalizeExtra(body.ExtraData)

	var id string
	var err error
	if body.ID != nil {
		err = h.db.QueryRowContext(r.Context(), `
			INSERT INTO concept_templates (id, novel_id, name, template_data, is_default, extra_data)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id
		`, body.ID, novelId, body.Name, tmpl, body.IsDefault, extra).Scan(&id)
	} else {
		err = h.db.QueryRowContext(r.Context(), `
			INSERT INTO concept_templates (novel_id, name, template_data, is_default, extra_data)
			VALUES ($1, $2, $3, $4, $5) RETURNING id
		`, novelId, body.Name, tmpl, body.IsDefault, extra).Scan(&id)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *templatesHandler) update(w http.ResponseWriter, r *http.Request) {
	id, novelId := r.PathValue("id"), r.PathValue("novelId")

	var body struct {
		Name         *string         `json:"name"`
		TemplateData json.RawMessage `json:"template_data"`
		IsDefault    *bool           `json:"is_default"`
		ExtraData    json.RawMessage `json:"extra_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tmpl := normalizeExtra(body.TemplateData)
	extra := normalizeExtra(body.ExtraData)

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE concept_templates SET
			name          = COALESCE($3, name),
			template_data = CASE WHEN $4::jsonb IS NOT NULL THEN $4 ELSE template_data END,
			is_default    = COALESCE($5, is_default),
			extra_data    = extra_data || $6,
			updated_at    = NOW()
		WHERE id = $1 AND novel_id = $2
	`, id, novelId, body.Name, tmpl, body.IsDefault, extra)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *templatesHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, novelId := r.PathValue("id"), r.PathValue("novelId")
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM concept_templates WHERE id = $1 AND novel_id = $2`, id, novelId); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scanTemplate(s scanner) (map[string]any, error) {
	var id, name, createdAt, updatedAt string
	var isDefault bool
	var templateData, extra []byte

	if err := s.Scan(&id, &name, &templateData, &isDefault, &extra, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var tmplMap, extraMap map[string]any
	if len(templateData) > 0 {
		_ = json.Unmarshal(templateData, &tmplMap)
	}
	if len(extra) > 0 {
		_ = json.Unmarshal(extra, &extraMap)
	}
	return map[string]any{
		"id": id, "name": name, "template_data": tmplMap,
		"is_default": isDefault, "extra_data": extraMap,
		"created_at": createdAt, "updated_at": updatedAt,
	}, nil
}
