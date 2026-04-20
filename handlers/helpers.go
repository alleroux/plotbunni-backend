package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// normalizeExtra returns '{}' when raw is nil/empty so Postgres JSONB is always valid.
func normalizeExtra(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

// pqArray wraps a []string for lib/pq array binding. Returns nil when the
// slice is nil so COALESCE in UPDATE statements leaves the column unchanged.
func pqArray(s []string) interface{} {
	if s == nil {
		return nil
	}
	return (*pqStringArray)(&s)
}

// pqStringArray implements driver.Valuer and sql.Scanner for TEXT[] columns.
type pqStringArray []string

func (a pqStringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return fmt.Sprintf("{%s}", joinQuoted(a)), nil
}

func (a *pqStringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
	*a = parseArray(s)
	return nil
}

func joinQuoted(ss []string) string {
	out := make([]byte, 0, 64)
	for i, s := range ss {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, '"')
		for _, c := range s {
			if c == '"' || c == '\\' {
				out = append(out, '\\')
			}
			out = append(out, byte(c))
		}
		out = append(out, '"')
	}
	return string(out)
}

func parseArray(s string) []string {
	if s == "{}" || s == "" {
		return []string{}
	}
	s = s[1 : len(s)-1]
	var result []string
	var cur []byte
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == '\\' && inQuote:
			i++
			cur = append(cur, s[i])
		case c == ',' && !inQuote:
			result = append(result, string(cur))
			cur = cur[:0]
		default:
			cur = append(cur, c)
		}
	}
	if len(cur) > 0 || len(result) > 0 {
		result = append(result, string(cur))
	}
	return result
}

// upsertSceneConcepts inserts concept links for a scene within a transaction.
func upsertSceneConcepts(ctx context.Context, tx *sql.Tx, sceneId string, conceptIds []string) error {
	for _, cid := range conceptIds {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scene_concepts (scene_id, concept_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, sceneId, cid); err != nil {
			return err
		}
	}
	return nil
}
