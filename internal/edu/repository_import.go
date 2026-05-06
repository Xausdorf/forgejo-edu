package edu

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
)

func (r *xormRepository) CreateImportDraft(ctx context.Context, d *ImportDraft) error {
	_, err := db.GetEngine(ctx).Insert(d)
	if err != nil {
		return fmt.Errorf("insert import draft: %w", err)
	}
	return nil
}

func (r *xormRepository) GetImportDraft(ctx context.Context, id int64) (*ImportDraft, error) {
	d := &ImportDraft{}
	has, err := db.GetEngine(ctx).ID(id).Get(d)
	if err != nil {
		return nil, fmt.Errorf("get import draft: %w", err)
	}
	if !has {
		return nil, nil
	}
	return d, nil
}

func (r *xormRepository) CreateImportDraftRows(ctx context.Context, rows []*ImportDraftRow) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := db.GetEngine(ctx).Insert(&rows)
	if err != nil {
		return fmt.Errorf("insert import draft rows: %w", err)
	}
	return nil
}

func (r *xormRepository) GetImportDraftRows(ctx context.Context, draftID int64) ([]*ImportDraftRow, error) {
	var rows []*ImportDraftRow
	err := db.GetEngine(ctx).Where("draft_id = ?", draftID).OrderBy("id ASC").Find(&rows)
	if err != nil {
		return nil, fmt.Errorf("find import draft rows: %w", err)
	}
	return rows, nil
}

func (r *xormRepository) UpdateImportDraftRow(ctx context.Context, row *ImportDraftRow) error {
	_, err := db.GetEngine(ctx).ID(row.ID).Cols("username", "email", "group_name", "status", "error_msg").Update(row)
	if err != nil {
		return fmt.Errorf("update import draft row: %w", err)
	}
	return nil
}

func (r *xormRepository) UpdateImportDraft(ctx context.Context, d *ImportDraft) error {
	_, err := db.GetEngine(ctx).ID(d.ID).Cols("status").Update(d)
	if err != nil {
		return fmt.Errorf("update import draft: %w", err)
	}
	return nil
}

func (r *xormRepository) DeleteImportDraft(ctx context.Context, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)

		// Delete rows first
		if _, err := e.Where("draft_id = ?", id).Delete(&ImportDraftRow{}); err != nil {
			return fmt.Errorf("delete import draft rows: %w", err)
		}

		// Delete draft
		if _, err := e.ID(id).Delete(&ImportDraft{}); err != nil {
			return fmt.Errorf("delete import draft: %w", err)
		}

		return nil
	})
}
