package db

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Novel is the flat representation returned by GET /api/v1/novels/{id}.
type Novel struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Synopsis   *string         `json:"synopsis"`
	CoverImage *string         `json:"cover_image"`
	Author     *string         `json:"author"`
	POV        *string         `json:"pov"`
	Genre      *string         `json:"genre"`
	TimePeriod *string         `json:"time_period"`
	Audience   *string         `json:"audience"`
	Themes     []string        `json:"themes"`
	Tone       *string         `json:"tone"`
	ExtraData  json.RawMessage `json:"extra_data"`
	UserID     *string         `json:"user_id,omitempty"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

// FullNovel mirrors the shape the frontend stores in IndexedDB so that the
// import/export endpoints are a lossless round-trip even when the upstream
// frontend adds new fields (they land in ExtraData).
type FullNovel struct {
	Novel
	Concepts          []map[string]any `json:"concepts"`
	Acts              []FullAct        `json:"acts"`
	ConceptTemplates  []map[string]any `json:"concept_templates"`
}

type FullAct struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Position  int           `json:"position"`
	ExtraData json.RawMessage `json:"extra_data"`
	Chapters  []FullChapter `json:"chapters"`
}

type FullChapter struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Position  int             `json:"position"`
	ExtraData json.RawMessage `json:"extra_data"`
	Scenes    []map[string]any `json:"scenes"`
}

func GetNovel(ctx context.Context, db *sql.DB, id string) (*Novel, error) {
	var n Novel
	var extra []byte
	var themes []byte

	err := db.QueryRowContext(ctx, `
		SELECT id, name, synopsis, cover_image, author, pov, genre, time_period, audience,
		       array_to_json(themes), tone, extra_data, user_id, created_at, updated_at
		FROM novels WHERE id = $1
	`, id).Scan(
		&n.ID, &n.Name, &n.Synopsis, &n.CoverImage, &n.Author,
		&n.POV, &n.Genre, &n.TimePeriod, &n.Audience,
		&themes, &n.Tone, &extra, &n.UserID, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(themes) > 0 {
		_ = json.Unmarshal(themes, &n.Themes)
	}
	if len(extra) > 0 {
		n.ExtraData = extra
	} else {
		n.ExtraData = json.RawMessage("{}")
	}
	return &n, nil
}

// ImportFullNovelForUser is like ImportFullNovel but assigns ownership to userID.
func ImportFullNovelForUser(ctx context.Context, db *sql.DB, userID string, payload *FullNovel) (string, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	extra := jsonOrEmpty(payload.ExtraData)

	var novelId string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO novels (name, synopsis, cover_image, author, pov, genre, time_period, audience, tone, extra_data, user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id
	`, payload.Name, payload.Synopsis, payload.CoverImage, payload.Author,
		payload.POV, payload.Genre, payload.TimePeriod, payload.Audience, payload.Tone, extra, userID).Scan(&novelId)
	if err != nil {
		return "", err
	}

	for _, c := range payload.Concepts {
		if err := insertConceptMap(ctx, tx, novelId, c); err != nil {
			return "", err
		}
	}

	for _, act := range payload.Acts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO acts (id, novel_id, name, position, extra_data)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE
			SET name=$3, position=$4, extra_data=$5
		`, act.ID, novelId, act.Name, act.Position, jsonOrEmpty(act.ExtraData)); err != nil {
			return "", err
		}
		for _, ch := range act.Chapters {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO chapters (id, act_id, name, position, extra_data)
				VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE
				SET name=$3, position=$4, extra_data=$5
			`, ch.ID, act.ID, ch.Name, ch.Position, jsonOrEmpty(ch.ExtraData)); err != nil {
				return "", err
			}
			for _, s := range ch.Scenes {
				if err := insertSceneMap(ctx, tx, ch.ID, s); err != nil {
					return "", err
				}
			}
		}
	}

	for _, t := range payload.ConceptTemplates {
		if err := insertTemplateMap(ctx, tx, novelId, t); err != nil {
			return "", err
		}
	}

	return novelId, tx.Commit()
}

// GetFullNovel assembles the complete novel tree in one database round-trip per layer.
func GetFullNovel(ctx context.Context, db *sql.DB, id string) (*FullNovel, error) {
	novel, err := GetNovel(ctx, db, id)
	if err != nil {
		return nil, err
	}

	full := &FullNovel{Novel: *novel}

	// Concepts
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, name, array_to_json(aliases), array_to_json(tags),
		       description, notes, priority, image, extra_data, created_at, updated_at
		FROM concepts WHERE novel_id = $1 ORDER BY priority DESC, name
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanConceptRow(rows)
		if err != nil {
			return nil, err
		}
		full.Concepts = append(full.Concepts, c)
	}

	// Acts → Chapters → Scenes
	actRows, err := db.QueryContext(ctx, `
		SELECT id, name, position, extra_data FROM acts WHERE novel_id = $1 ORDER BY position
	`, id)
	if err != nil {
		return nil, err
	}
	defer actRows.Close()
	for actRows.Next() {
		var a FullAct
		var extra []byte
		if err := actRows.Scan(&a.ID, &a.Name, &a.Position, &extra); err != nil {
			return nil, err
		}
		a.ExtraData = jsonOrEmpty(extra)

		chRows, err := db.QueryContext(ctx, `
			SELECT id, name, position, extra_data FROM chapters WHERE act_id = $1 ORDER BY position
		`, a.ID)
		if err != nil {
			return nil, err
		}
		for chRows.Next() {
			var ch FullChapter
			var chExtra []byte
			if err := chRows.Scan(&ch.ID, &ch.Name, &ch.Position, &chExtra); err != nil {
				chRows.Close()
				return nil, err
			}
			ch.ExtraData = jsonOrEmpty(chExtra)

			scRows, err := db.QueryContext(ctx, `
				SELECT s.id, s.name, s.synopsis, s.content, array_to_json(s.tags), s.auto_update_context,
				       s.position, s.extra_data, s.created_at, s.updated_at,
				       COALESCE(json_agg(sc.concept_id) FILTER (WHERE sc.concept_id IS NOT NULL), '[]')
				FROM scenes s
				LEFT JOIN scene_concepts sc ON sc.scene_id = s.id
				WHERE s.chapter_id = $1
				GROUP BY s.id ORDER BY s.position
			`, ch.ID)
			if err != nil {
				chRows.Close()
				return nil, err
			}
			for scRows.Next() {
				s, err := scanSceneRow(scRows)
				if err != nil {
					scRows.Close()
					chRows.Close()
					return nil, err
				}
				ch.Scenes = append(ch.Scenes, s)
			}
			scRows.Close()
			a.Chapters = append(a.Chapters, ch)
		}
		chRows.Close()
		full.Acts = append(full.Acts, a)
	}

	// Concept templates
	tmplRows, err := db.QueryContext(ctx, `
		SELECT id, name, template_data, is_default, extra_data, created_at, updated_at
		FROM concept_templates WHERE novel_id = $1 ORDER BY name
	`, id)
	if err != nil {
		return nil, err
	}
	defer tmplRows.Close()
	for tmplRows.Next() {
		t, err := scanTemplateRow(tmplRows)
		if err != nil {
			return nil, err
		}
		full.ConceptTemplates = append(full.ConceptTemplates, t)
	}

	return full, nil
}

// ImportFullNovel persists a complete FullNovel payload in a single transaction.
// Existing IDs are preserved so the frontend can diff against its own state.
func ImportFullNovel(ctx context.Context, db *sql.DB, payload *FullNovel) (string, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	extra := jsonOrEmpty(payload.ExtraData)

	var novelId string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO novels (name, synopsis, cover_image, author, pov, genre, time_period, audience, tone, extra_data)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id
	`, payload.Name, payload.Synopsis, payload.CoverImage, payload.Author,
		payload.POV, payload.Genre, payload.TimePeriod, payload.Audience, payload.Tone, extra).Scan(&novelId)
	if err != nil {
		return "", err
	}

	for _, c := range payload.Concepts {
		if err := insertConceptMap(ctx, tx, novelId, c); err != nil {
			return "", err
		}
	}

	for _, act := range payload.Acts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO acts (id, novel_id, name, position, extra_data)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE
			SET name=$3, position=$4, extra_data=$5
		`, act.ID, novelId, act.Name, act.Position, jsonOrEmpty(act.ExtraData)); err != nil {
			return "", err
		}
		for _, ch := range act.Chapters {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO chapters (id, act_id, name, position, extra_data)
				VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE
				SET name=$3, position=$4, extra_data=$5
			`, ch.ID, act.ID, ch.Name, ch.Position, jsonOrEmpty(ch.ExtraData)); err != nil {
				return "", err
			}
			for _, s := range ch.Scenes {
				if err := insertSceneMap(ctx, tx, ch.ID, s); err != nil {
					return "", err
				}
			}
		}
	}

	for _, t := range payload.ConceptTemplates {
		if err := insertTemplateMap(ctx, tx, novelId, t); err != nil {
			return "", err
		}
	}

	return novelId, tx.Commit()
}

func jsonOrEmpty(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return b
}

func scanConceptRow(rows *sql.Rows) (map[string]any, error) {
	var id, cType, name, createdAt, updatedAt string
	var aliases, tags []byte
	var description, notes, image *string
	var priority int
	var extra []byte

	if err := rows.Scan(&id, &cType, &name, &aliases, &tags, &description, &notes, &priority, &image, &extra, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var aliasSlice, tagSlice []string
	_ = json.Unmarshal(aliases, &aliasSlice)
	_ = json.Unmarshal(tags, &tagSlice)
	var extraMap map[string]any
	_ = json.Unmarshal(extra, &extraMap)
	return map[string]any{
		"id": id, "type": cType, "name": name,
		"aliases": aliasSlice, "tags": tagSlice,
		"description": description, "notes": notes,
		"priority": priority, "image": image,
		"extra_data": extraMap, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func scanSceneRow(rows *sql.Rows) (map[string]any, error) {
	var id, name, createdAt, updatedAt string
	var synopsis, content *string
	var tags, conceptIds []byte
	var autoUpdateContext bool
	var position int
	var extra []byte

	if err := rows.Scan(&id, &name, &synopsis, &content, &tags, &autoUpdateContext,
		&position, &extra, &createdAt, &updatedAt, &conceptIds); err != nil {
		return nil, err
	}
	var tagSlice, conceptSlice []string
	_ = json.Unmarshal(tags, &tagSlice)
	_ = json.Unmarshal(conceptIds, &conceptSlice)
	var extraMap map[string]any
	_ = json.Unmarshal(extra, &extraMap)
	return map[string]any{
		"id": id, "name": name, "synopsis": synopsis, "content": content,
		"tags": tagSlice, "auto_update_context": autoUpdateContext,
		"position": position, "concept_ids": conceptSlice,
		"extra_data": extraMap, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func scanTemplateRow(rows *sql.Rows) (map[string]any, error) {
	var id, name, createdAt, updatedAt string
	var isDefault bool
	var templateData, extra []byte

	if err := rows.Scan(&id, &name, &templateData, &isDefault, &extra, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var tmplMap, extraMap map[string]any
	_ = json.Unmarshal(templateData, &tmplMap)
	_ = json.Unmarshal(extra, &extraMap)
	return map[string]any{
		"id": id, "name": name, "template_data": tmplMap,
		"is_default": isDefault, "extra_data": extraMap,
		"created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func insertConceptMap(ctx context.Context, tx *sql.Tx, novelId string, c map[string]any) error {
	id, _ := c["id"].(string)
	name, _ := c["name"].(string)
	cType, _ := c["type"].(string)
	description, _ := c["description"].(*string)
	notes, _ := c["notes"].(*string)
	image, _ := c["image"].(*string)
	priority, _ := c["priority"].(float64)
	extra, _ := json.Marshal(c["extra_data"])

	_, err := tx.ExecContext(ctx, `
		INSERT INTO concepts (id, novel_id, type, name, description, notes, priority, image, extra_data)
		VALUES (COALESCE(NULLIF($1,''), gen_random_uuid()::text)::uuid, $2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE
		SET name=$4, description=$5, notes=$6, priority=$7, image=$8, extra_data=$9
	`, id, novelId, cType, name, description, notes, int(priority), image, extra)
	return err
}

func insertSceneMap(ctx context.Context, tx *sql.Tx, chapterId string, s map[string]any) error {
	id, _ := s["id"].(string)
	name, _ := s["name"].(string)
	synopsis, _ := s["synopsis"].(*string)
	content, _ := s["content"].(*string)
	autoUpdate, _ := s["auto_update_context"].(bool)
	position, _ := s["position"].(float64)
	extra, _ := json.Marshal(s["extra_data"])

	var sceneId string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO scenes (id, chapter_id, name, synopsis, content, auto_update_context, position, extra_data)
		VALUES (COALESCE(NULLIF($1,''), gen_random_uuid()::text)::uuid, $2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE
		SET name=$3, synopsis=$4, content=$5, auto_update_context=$6, position=$7, extra_data=$8
		RETURNING id
	`, id, chapterId, name, synopsis, content, autoUpdate, int(position), extra).Scan(&sceneId); err != nil {
		return err
	}

	conceptIds, _ := s["concept_ids"].([]any)
	for _, cid := range conceptIds {
		cidStr, _ := cid.(string)
		if cidStr == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scene_concepts (scene_id, concept_id) VALUES ($1,$2) ON CONFLICT DO NOTHING
		`, sceneId, cidStr); err != nil {
			return err
		}
	}
	return nil
}

func insertTemplateMap(ctx context.Context, tx *sql.Tx, novelId string, t map[string]any) error {
	id, _ := t["id"].(string)
	name, _ := t["name"].(string)
	isDefault, _ := t["is_default"].(bool)
	tmplData, _ := json.Marshal(t["template_data"])
	extra, _ := json.Marshal(t["extra_data"])

	_, err := tx.ExecContext(ctx, `
		INSERT INTO concept_templates (id, novel_id, name, template_data, is_default, extra_data)
		VALUES (COALESCE(NULLIF($1,''), gen_random_uuid()::text)::uuid, $2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE
		SET name=$3, template_data=$4, is_default=$5, extra_data=$6
	`, id, novelId, name, tmplData, isDefault, extra)
	return err
}
