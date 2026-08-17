package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AssetValuation struct {
	ID            int64  `json:"id"`
	AssetID       int64  `json:"assetId"`
	ValuationDate string `json:"valuationDate"`
	ValueCents    int64  `json:"valueCents"`
	ValuationType string `json:"valuationType"`
	Source        string `json:"source,omitempty"`
	Notes         string `json:"notes,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type SaveAssetValuation struct {
	ValuationDate string `json:"valuationDate"`
	ValueCents    int64  `json:"valueCents"`
	ValuationType string `json:"valuationType"`
	Source        string `json:"source"`
	Notes         string `json:"notes"`
}

func (s *Postgres) AssetValuations(ctx context.Context, assetID int64) ([]AssetValuation, error) {
	rows, err := s.pool.Query(ctx, `
		select id, asset_id, valuation_date::text, value, valuation_type, coalesce(source, ''), coalesce(notes, ''), created_at::text
		from asset_valuations
		where asset_id = $1
		order by valuation_date desc, id desc
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AssetValuation{}
	for rows.Next() {
		var item AssetValuation
		if err := scanAssetValuation(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) CreateAssetValuation(ctx context.Context, assetID int64, input SaveAssetValuation) (AssetValuation, error) {
	input, valuationDate, err := normalizeAssetValuation(input)
	if err != nil {
		return AssetValuation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AssetValuation{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockAsset(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	var item AssetValuation
	if err := tx.QueryRow(ctx, `
		insert into asset_valuations (asset_id, valuation_date, value, valuation_type, source, notes)
		values ($1, $2, $3, $4, $5, $6)
		returning id, asset_id, valuation_date::text, value, valuation_type, coalesce(source, ''), coalesce(notes, ''), created_at::text
	`, assetID, valuationDate, input.ValueCents, input.ValuationType, input.Source, input.Notes).Scan(
		&item.ID, &item.AssetID, &item.ValuationDate, &item.ValueCents, &item.ValuationType, &item.Source, &item.Notes, &item.CreatedAt,
	); err != nil {
		return AssetValuation{}, err
	}
	if err := updateAssetCurrentValue(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_added", EventDate: valuationDate, Description: "Waardering toegevoegd"}); err != nil {
		return AssetValuation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AssetValuation{}, err
	}
	return item, nil
}

func (s *Postgres) UpdateAssetValuation(ctx context.Context, assetID, id int64, input SaveAssetValuation) (AssetValuation, error) {
	input, valuationDate, err := normalizeAssetValuation(input)
	if err != nil {
		return AssetValuation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AssetValuation{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockAsset(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	var item AssetValuation
	err = tx.QueryRow(ctx, `
		update asset_valuations
		set valuation_date = $1, value = $2, valuation_type = $3, source = $4, notes = $5
		where id = $6 and asset_id = $7
		returning id, asset_id, valuation_date::text, value, valuation_type, coalesce(source, ''), coalesce(notes, ''), created_at::text
	`, valuationDate, input.ValueCents, input.ValuationType, input.Source, input.Notes, id, assetID).Scan(
		&item.ID, &item.AssetID, &item.ValuationDate, &item.ValueCents, &item.ValuationType, &item.Source, &item.Notes, &item.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return AssetValuation{}, fmt.Errorf("asset valuation not found")
	} else if err != nil {
		return AssetValuation{}, err
	}
	if err := updateAssetCurrentValue(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_updated", EventDate: valuationDate, Description: "Waardering bijgewerkt"}); err != nil {
		return AssetValuation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AssetValuation{}, err
	}
	return item, nil
}

func (s *Postgres) DeleteAssetValuation(ctx context.Context, assetID, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockAsset(ctx, tx, assetID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `delete from asset_valuations where id = $1 and asset_id = $2`, id, assetID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("asset valuation not found")
	}
	if err := updateAssetCurrentValue(ctx, tx, assetID); err != nil {
		return err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_removed", CurrentDate: true, Description: "Waardering verwijderd"}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type valuationScanner interface {
	Scan(...any) error
}

func scanAssetValuation(scanner valuationScanner, item *AssetValuation) error {
	return scanner.Scan(&item.ID, &item.AssetID, &item.ValuationDate, &item.ValueCents, &item.ValuationType, &item.Source, &item.Notes, &item.CreatedAt)
}

func normalizeAssetValuation(input SaveAssetValuation) (SaveAssetValuation, time.Time, error) {
	input.ValuationDate = strings.TrimSpace(input.ValuationDate)
	input.ValuationType = strings.TrimSpace(input.ValuationType)
	input.Source = strings.TrimSpace(input.Source)
	input.Notes = strings.TrimSpace(input.Notes)
	valuationDate, err := time.Parse("2006-01-02", input.ValuationDate)
	if err != nil {
		return SaveAssetValuation{}, time.Time{}, fmt.Errorf("invalid valuation date")
	}
	if input.ValueCents < 0 {
		return SaveAssetValuation{}, time.Time{}, fmt.Errorf("valuation cannot be negative")
	}
	if !validValuationType(input.ValuationType) {
		return SaveAssetValuation{}, time.Time{}, fmt.Errorf("invalid valuation type")
	}
	return input, valuationDate, nil
}

func validValuationType(value string) bool {
	switch value {
	case "purchase", "estimate", "appraisal", "tax", "market", "other":
		return true
	default:
		return false
	}
}

func lockAsset(ctx context.Context, tx pgx.Tx, assetID int64) error {
	var lockedID int64
	if err := tx.QueryRow(ctx, `select id from assets where id = $1 for update`, assetID).Scan(&lockedID); err == pgx.ErrNoRows {
		return fmt.Errorf("asset not found")
	} else if err != nil {
		return err
	}
	return nil
}

func updateAssetCurrentValue(ctx context.Context, tx pgx.Tx, assetID int64) error {
	_, err := tx.Exec(ctx, `
		update assets
		set current_value = coalesce((
			select value from asset_valuations
			where asset_id = $1
			order by valuation_date desc, id desc
			limit 1
		), 0), updated_at = now()
		where id = $1
	`, assetID)
	return err
}
