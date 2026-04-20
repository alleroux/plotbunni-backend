package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ReplaceFullNovel atomically replaces all data for an existing novel.
// It deletes all child records (concepts, acts → chapters → scenes) and
// re-inserts from the payload, preserving the novel row's ID.
func ReplaceFullNovel(ctx context.Context, db *sql.DB, id string, payload *FullNovel) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	extra := jsonOrEmpty(payload.ExtraData)

	if _, err := tx.ExecContext(ctx, `
		UPDATE novels SET
			synopsis    = $2,
			cover_image = $3,
			author      = $4,
			pov         = $5,
			genre       = $6,
			time_period = $7,
			audience    = $8,
			tone        = $9,
			extra_data  = $10,
			updated_at  = NOW()
		WHERE id = $1
	`, id, payload.Synopsis, payload.CoverImage, payload.Author,
		payload.POV, payload.Genre, payload.TimePeriod, payload.Audience,
		payload.Tone, extra); err != nil {
		return fmt.Errorf("update novel: %w", err)
	}

	// Delete in dependency order; ON DELETE CASCADE handles chapters/scenes/scene_concepts.
	if _, err := tx.ExecContext(ctx, `DELETE FROM concepts WHERE novel_id = $1`, id); err != nil {
		return fmt.Errorf("delete concepts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM acts WHERE novel_id = $1`, id); err != nil {
		return fmt.Errorf("delete acts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM concept_templates WHERE novel_id = $1`, id); err != nil {
		return fmt.Errorf("delete templates: %w", err)
	}

	for _, c := range payload.Concepts {
		if err := insertConceptMap(ctx, tx, id, c); err != nil {
			return fmt.Errorf("insert concept: %w", err)
		}
	}

	for _, act := range payload.Acts {
		actExtra := jsonOrEmpty(act.ExtraData)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO acts (id, novel_id, name, position, extra_data)
			VALUES (COALESCE(NULLIF($1,'')::uuid, gen_random_uuid()), $2, $3, $4, $5)
		`, act.ID, id, act.Name, act.Position, actExtra); err != nil {
			return fmt.Errorf("insert act: %w", err)
		}

		for _, ch := range act.Chapters {
			chExtra := jsonOrEmpty(ch.ExtraData)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO chapters (id, act_id, name, position, extra_data)
				VALUES (COALESCE(NULLIF($1,'')::uuid, gen_random_uuid()), $2, $3, $4, $5)
			`, ch.ID, act.ID, ch.Name, ch.Position, chExtra); err != nil {
				return fmt.Errorf("insert chapter: %w", err)
			}

			for _, s := range ch.Scenes {
				if err := insertSceneMap(ctx, tx, ch.ID, s); err != nil {
					return fmt.Errorf("insert scene: %w", err)
				}
			}
		}
	}

	for _, t := range payload.ConceptTemplates {
		if err := insertTemplateMap(ctx, tx, id, t); err != nil {
			return fmt.Errorf("insert template: %w", err)
		}
	}

	return tx.Commit()
}

func jsonOrEmptyRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil || b == nil {
		return json.RawMessage("{}")
	}
	return b
}
