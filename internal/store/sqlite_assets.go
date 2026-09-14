package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *SQLite) Assets(ctx context.Context, filter AssetFilter) ([]Asset, error) {
	rows, err := s.db.QueryContext(ctx, sqliteAssetSelect+`
		where (? = '' or asset_type = ?) and (? = '' or status = ?)
		order by status, name, id
	`, filter.AssetType, filter.AssetType, filter.Status, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Asset{}
	for rows.Next() {
		var item Asset
		if err := scanAsset(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLite) Asset(ctx context.Context, id int64) (Asset, error) {
	var item Asset
	err := scanAsset(s.db.QueryRowContext(ctx, sqliteAssetSelect+` where id = ?`, id), &item)
	if err == sql.ErrNoRows {
		return Asset{}, fmt.Errorf("asset not found")
	}
	return item, err
}

func (s *SQLite) CreateAsset(ctx context.Context, input SaveAsset) (Asset, error) {
	input, _, _, err := normalizeAssetInput(input)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		insert into assets (household_id, asset_type, name, description, status, acquired_at, disposed_at, purchase_value, currency)
		values ((select id from households order by id limit 1), ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.AssetType, input.Name, input.Description, input.Status, sqliteOptionalDate(input.AcquiredAt), sqliteOptionalDate(input.DisposedAt), input.PurchaseValueCents, input.Currency)
	if err != nil {
		return Asset{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Asset{}, err
	}
	var item Asset
	if err := scanAsset(tx.QueryRowContext(ctx, sqliteAssetSelect+` where id = ?`, id), &item); err != nil {
		return Asset{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: id, EventType: "asset_created", CurrentDate: true, Description: "Bezitting aangemaakt"}); err != nil {
		return Asset{}, err
	}
	if err := tx.Commit(); err != nil {
		return Asset{}, err
	}
	return item, nil
}

func (s *SQLite) UpdateAsset(ctx context.Context, id int64, input SaveAsset) (Asset, error) {
	input, _, _, err := normalizeAssetInput(input)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback()
	var currentCurrency string
	var currentPurchaseValue int64
	if err := tx.QueryRowContext(ctx, `select currency, purchase_value from assets where id = ?`, id).Scan(&currentCurrency, &currentPurchaseValue); err == sql.ErrNoRows {
		return Asset{}, fmt.Errorf("asset not found")
	} else if err != nil {
		return Asset{}, err
	}
	if strings.TrimSpace(currentCurrency) != input.Currency {
		var valuationCount int
		if err := tx.QueryRowContext(ctx, `select count(*) from asset_valuations where asset_id = ?`, id).Scan(&valuationCount); err != nil {
			return Asset{}, err
		}
		if valuationCount > 0 || currentPurchaseValue > 0 {
			return Asset{}, fmt.Errorf("currency cannot change while asset values exist")
		}
	}
	_, err = tx.ExecContext(ctx, `
		update assets set asset_type = ?, name = ?, description = ?, status = ?, acquired_at = ?, disposed_at = ?,
			purchase_value = ?, currency = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		where id = ?
	`, input.AssetType, input.Name, input.Description, input.Status, sqliteOptionalDate(input.AcquiredAt), sqliteOptionalDate(input.DisposedAt), input.PurchaseValueCents, input.Currency, id)
	if err != nil {
		return Asset{}, err
	}
	var item Asset
	if err := scanAsset(tx.QueryRowContext(ctx, sqliteAssetSelect+` where id = ?`, id), &item); err != nil {
		return Asset{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: id, EventType: "asset_updated", CurrentDate: true, Description: "Bezitting bijgewerkt"}); err != nil {
		return Asset{}, err
	}
	if err := tx.Commit(); err != nil {
		return Asset{}, err
	}
	return item, nil
}

func (s *SQLite) DeleteAsset(ctx context.Context, id int64) error {
	return s.sqliteDeleteDomainObject(ctx, ObjectTypeAsset, id, "asset")
}

func (s *SQLite) AssetValuations(ctx context.Context, assetID int64) ([]AssetValuation, error) {
	rows, err := s.db.QueryContext(ctx, sqliteValuationSelect+` where asset_id = ? order by valuation_date desc, id desc`, assetID)
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

func (s *SQLite) CreateAssetValuation(ctx context.Context, assetID int64, input SaveAssetValuation) (AssetValuation, error) {
	input, valuationDate, err := normalizeAssetValuation(input)
	if err != nil {
		return AssetValuation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetValuation{}, err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, ObjectTypeAsset, assetID); err != nil {
		return AssetValuation{}, err
	}
	result, err := tx.ExecContext(ctx, `
		insert into asset_valuations (asset_id, valuation_date, value, valuation_type, source, notes)
		values (?, ?, ?, ?, ?, ?)
	`, assetID, input.ValuationDate, input.ValueCents, input.ValuationType, input.Source, input.Notes)
	if err != nil {
		return AssetValuation{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return AssetValuation{}, err
	}
	var item AssetValuation
	if err := scanAssetValuation(tx.QueryRowContext(ctx, sqliteValuationSelect+` where id = ?`, id), &item); err != nil {
		return AssetValuation{}, err
	}
	if err := updateSQLiteAssetCurrentValue(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_added", EventDate: valuationDate, Description: "Waardering toegevoegd"}); err != nil {
		return AssetValuation{}, err
	}
	if err := tx.Commit(); err != nil {
		return AssetValuation{}, err
	}
	return item, nil
}

func (s *SQLite) UpdateAssetValuation(ctx context.Context, assetID, id int64, input SaveAssetValuation) (AssetValuation, error) {
	input, valuationDate, err := normalizeAssetValuation(input)
	if err != nil {
		return AssetValuation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetValuation{}, err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, ObjectTypeAsset, assetID); err != nil {
		return AssetValuation{}, err
	}
	result, err := tx.ExecContext(ctx, `
		update asset_valuations set valuation_date = ?, value = ?, valuation_type = ?, source = ?, notes = ?
		where id = ? and asset_id = ?
	`, input.ValuationDate, input.ValueCents, input.ValuationType, input.Source, input.Notes, id, assetID)
	if err != nil {
		return AssetValuation{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return AssetValuation{}, fmt.Errorf("asset valuation not found")
	}
	var item AssetValuation
	if err := scanAssetValuation(tx.QueryRowContext(ctx, sqliteValuationSelect+` where id = ?`, id), &item); err != nil {
		return AssetValuation{}, err
	}
	if err := updateSQLiteAssetCurrentValue(ctx, tx, assetID); err != nil {
		return AssetValuation{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_updated", EventDate: valuationDate, Description: "Waardering bijgewerkt"}); err != nil {
		return AssetValuation{}, err
	}
	if err := tx.Commit(); err != nil {
		return AssetValuation{}, err
	}
	return item, nil
}

func (s *SQLite) DeleteAssetValuation(ctx context.Context, assetID, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, ObjectTypeAsset, assetID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `delete from asset_valuations where id = ? and asset_id = ?`, id, assetID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("asset valuation not found")
	}
	if err := updateSQLiteAssetCurrentValue(ctx, tx, assetID); err != nil {
		return err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: assetID, EventType: "valuation_removed", CurrentDate: true, Description: "Waardering verwijderd"}); err != nil {
		return err
	}
	return tx.Commit()
}

func updateSQLiteAssetCurrentValue(ctx context.Context, tx *sql.Tx, assetID int64) error {
	_, err := tx.ExecContext(ctx, `
		update assets set current_value = coalesce((
			select value from asset_valuations where asset_id = ? order by valuation_date desc, id desc limit 1
		), 0), updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') where id = ?
	`, assetID, assetID)
	return err
}

const sqliteAssetSelect = `
	select id, household_id, asset_type, name, coalesce(description, ''), status,
		coalesce(acquired_at, ''), coalesce(disposed_at, ''), purchase_value, current_value, currency, created_at, updated_at
	from assets`

const sqliteValuationSelect = `
	select id, asset_id, valuation_date, value, valuation_type, coalesce(source, ''), coalesce(notes, ''), created_at
	from asset_valuations`
