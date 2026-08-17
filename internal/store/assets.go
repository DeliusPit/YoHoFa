package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const ObjectTypeAsset = "asset"

type Asset struct {
	ID                 int64  `json:"id"`
	HouseholdID        int64  `json:"householdId"`
	AssetType          string `json:"assetType"`
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	Status             string `json:"status"`
	AcquiredAt         string `json:"acquiredAt,omitempty"`
	DisposedAt         string `json:"disposedAt,omitempty"`
	PurchaseValueCents int64  `json:"purchaseValueCents"`
	CurrentValueCents  int64  `json:"currentValueCents"`
	Currency           string `json:"currency"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type SaveAsset struct {
	AssetType          string `json:"assetType"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Status             string `json:"status"`
	AcquiredAt         string `json:"acquiredAt"`
	DisposedAt         string `json:"disposedAt"`
	PurchaseValueCents int64  `json:"purchaseValueCents"`
	Currency           string `json:"currency"`
}

type AssetFilter struct {
	AssetType string
	Status    string
}

type AssetRepository interface {
	Assets(context.Context, AssetFilter) ([]Asset, error)
	Asset(context.Context, int64) (Asset, error)
	CreateAsset(context.Context, SaveAsset) (Asset, error)
	UpdateAsset(context.Context, int64, SaveAsset) (Asset, error)
	DeleteAsset(context.Context, int64) error
	AssetValuations(context.Context, int64) ([]AssetValuation, error)
	CreateAssetValuation(context.Context, int64, SaveAssetValuation) (AssetValuation, error)
	UpdateAssetValuation(context.Context, int64, int64, SaveAssetValuation) (AssetValuation, error)
	DeleteAssetValuation(context.Context, int64, int64) error
}

func (s *Postgres) Assets(ctx context.Context, filter AssetFilter) ([]Asset, error) {
	rows, err := s.pool.Query(ctx, `
		select id, household_id, asset_type, name, coalesce(description, ''), status,
			coalesce(to_char(acquired_at, 'YYYY-MM-DD'), ''), coalesce(to_char(disposed_at, 'YYYY-MM-DD'), ''),
			purchase_value, current_value, currency, created_at::text, updated_at::text
		from assets
		where ($1::text = '' or asset_type = $1)
		and ($2::text = '' or status = $2)
		order by status, name, id
	`, filter.AssetType, filter.Status)
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

func (s *Postgres) Asset(ctx context.Context, id int64) (Asset, error) {
	var item Asset
	err := scanAsset(s.pool.QueryRow(ctx, `
		select id, household_id, asset_type, name, coalesce(description, ''), status,
			coalesce(to_char(acquired_at, 'YYYY-MM-DD'), ''), coalesce(to_char(disposed_at, 'YYYY-MM-DD'), ''),
			purchase_value, current_value, currency, created_at::text, updated_at::text
		from assets where id = $1
	`, id), &item)
	if err == pgx.ErrNoRows {
		return Asset{}, fmt.Errorf("asset not found")
	}
	return item, err
}

func (s *Postgres) CreateAsset(ctx context.Context, input SaveAsset) (Asset, error) {
	input, acquiredAt, disposedAt, err := normalizeAssetInput(input)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback(ctx)
	var item Asset
	err = scanAsset(tx.QueryRow(ctx, `
		insert into assets (household_id, asset_type, name, description, status, acquired_at, disposed_at, purchase_value, currency)
		values ((select id from households order by id limit 1), $1, $2, $3, $4, $5, $6, $7, $8)
		returning id, household_id, asset_type, name, coalesce(description, ''), status,
			coalesce(to_char(acquired_at, 'YYYY-MM-DD'), ''), coalesce(to_char(disposed_at, 'YYYY-MM-DD'), ''),
			purchase_value, current_value, currency, created_at::text, updated_at::text
	`, input.AssetType, input.Name, input.Description, input.Status, acquiredAt, disposedAt, input.PurchaseValueCents, input.Currency), &item)
	if err != nil {
		return Asset{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: item.ID, EventType: "asset_created", CurrentDate: true, Description: "Bezitting aangemaakt"}); err != nil {
		return Asset{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Asset{}, err
	}
	return item, nil
}

func (s *Postgres) UpdateAsset(ctx context.Context, id int64, input SaveAsset) (Asset, error) {
	input, acquiredAt, disposedAt, err := normalizeAssetInput(input)
	if err != nil {
		return Asset{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback(ctx)
	var currentCurrency string
	var currentPurchaseValue int64
	if err := tx.QueryRow(ctx, `select currency, purchase_value from assets where id = $1 for update`, id).Scan(&currentCurrency, &currentPurchaseValue); err == pgx.ErrNoRows {
		return Asset{}, fmt.Errorf("asset not found")
	} else if err != nil {
		return Asset{}, err
	}
	if strings.TrimSpace(currentCurrency) != input.Currency {
		var valuationCount int
		if err := tx.QueryRow(ctx, `select count(*) from asset_valuations where asset_id = $1`, id).Scan(&valuationCount); err != nil {
			return Asset{}, err
		}
		if valuationCount > 0 || currentPurchaseValue > 0 {
			return Asset{}, fmt.Errorf("currency cannot change while asset values exist")
		}
	}
	var item Asset
	err = scanAsset(tx.QueryRow(ctx, `
		update assets
		set asset_type = $1, name = $2, description = $3, status = $4, acquired_at = $5, disposed_at = $6,
			purchase_value = $7, currency = $8, updated_at = now()
		where id = $9
		returning id, household_id, asset_type, name, coalesce(description, ''), status,
			coalesce(to_char(acquired_at, 'YYYY-MM-DD'), ''), coalesce(to_char(disposed_at, 'YYYY-MM-DD'), ''),
			purchase_value, current_value, currency, created_at::text, updated_at::text
	`, input.AssetType, input.Name, input.Description, input.Status, acquiredAt, disposedAt, input.PurchaseValueCents, input.Currency, id), &item)
	if err != nil {
		return Asset{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeAsset, EntityID: id, EventType: "asset_updated", CurrentDate: true, Description: "Bezitting bijgewerkt"}); err != nil {
		return Asset{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Asset{}, err
	}
	return item, nil
}

func (s *Postgres) DeleteAsset(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var lockedID int64
	if err := tx.QueryRow(ctx, `select id from assets where id = $1 for update`, id).Scan(&lockedID); err == pgx.ErrNoRows {
		return fmt.Errorf("asset not found")
	} else if err != nil {
		return err
	}
	var documentCount, relationCount int
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where source_type = $1 and target_type = $2 and target_id = $3 and relation_type = $4
	`, ObjectTypeDocument, ObjectTypeAsset, id, RelationTypeEvidenceFor).Scan(&documentCount); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where ((source_type = $1 and source_id = $2) or (target_type = $1 and target_id = $2))
		and not (source_type = $3 and target_type = $1 and target_id = $2 and relation_type = $4)
	`, ObjectTypeAsset, id, ObjectTypeDocument, RelationTypeEvidenceFor).Scan(&relationCount); err != nil {
		return err
	}
	if documentCount > 0 || relationCount > 0 {
		return fmt.Errorf("asset still has %d document links and %d object relations", documentCount, relationCount)
	}
	if err := cleanupDomainEvents(ctx, tx, ObjectTypeAsset, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from assets where id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type assetScanner interface {
	Scan(...any) error
}

func scanAsset(scanner assetScanner, item *Asset) error {
	return scanner.Scan(&item.ID, &item.HouseholdID, &item.AssetType, &item.Name, &item.Description, &item.Status,
		&item.AcquiredAt, &item.DisposedAt, &item.PurchaseValueCents, &item.CurrentValueCents, &item.Currency, &item.CreatedAt, &item.UpdatedAt)
}

func normalizeAssetInput(input SaveAsset) (SaveAsset, any, any, error) {
	input.AssetType = strings.TrimSpace(input.AssetType)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.TrimSpace(input.Status)
	input.AcquiredAt = strings.TrimSpace(input.AcquiredAt)
	input.DisposedAt = strings.TrimSpace(input.DisposedAt)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Name == "" {
		return SaveAsset{}, nil, nil, fmt.Errorf("name is required")
	}
	if !validAssetType(input.AssetType) {
		return SaveAsset{}, nil, nil, fmt.Errorf("invalid asset type")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "planned" && input.Status != "active" && input.Status != "disposed" {
		return SaveAsset{}, nil, nil, fmt.Errorf("invalid asset status")
	}
	if input.PurchaseValueCents < 0 {
		return SaveAsset{}, nil, nil, fmt.Errorf("asset values cannot be negative")
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	if !validCurrency(input.Currency) {
		return SaveAsset{}, nil, nil, fmt.Errorf("currency must contain three letters")
	}
	acquiredAt, err := optionalDate(input.AcquiredAt)
	if err != nil {
		return SaveAsset{}, nil, nil, fmt.Errorf("invalid acquired date")
	}
	disposedAt, err := optionalDate(input.DisposedAt)
	if err != nil {
		return SaveAsset{}, nil, nil, fmt.Errorf("invalid disposed date")
	}
	return input, acquiredAt, disposedAt, nil
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validAssetType(value string) bool {
	switch value {
	case "real_estate", "bank_account", "investment_account", "vehicle", "equipment", "valuable", "cash", "other":
		return true
	default:
		return false
	}
}

func optionalDate(value string) (any, error) {
	if value == "" {
		return nil, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return date, nil
}
