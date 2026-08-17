package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const ObjectTypeLiability = "liability"

type Liability struct {
	ID                  int64  `json:"id"`
	HouseholdID         int64  `json:"householdId"`
	LiabilityType       string `json:"liabilityType"`
	Name                string `json:"name"`
	Description         string `json:"description,omitempty"`
	Creditor            string `json:"creditor,omitempty"`
	StartDate           string `json:"startDate,omitempty"`
	EndDate             string `json:"endDate,omitempty"`
	OriginalAmountCents int64  `json:"originalAmountCents"`
	CurrentBalanceCents int64  `json:"currentBalanceCents"`
	Currency            string `json:"currency"`
	Status              string `json:"status"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type SaveLiability struct {
	LiabilityType       string `json:"liabilityType"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	Creditor            string `json:"creditor"`
	StartDate           string `json:"startDate"`
	EndDate             string `json:"endDate"`
	OriginalAmountCents int64  `json:"originalAmountCents"`
	CurrentBalanceCents int64  `json:"currentBalanceCents"`
	Currency            string `json:"currency"`
	Status              string `json:"status"`
}

type LiabilityFilter struct {
	LiabilityType string
	Status        string
}

type LiabilityRepository interface {
	Liabilities(context.Context, LiabilityFilter) ([]Liability, error)
	Liability(context.Context, int64) (Liability, error)
	CreateLiability(context.Context, SaveLiability) (Liability, error)
	UpdateLiability(context.Context, int64, SaveLiability) (Liability, error)
	DeleteLiability(context.Context, int64) error
}

func (s *Postgres) Liabilities(ctx context.Context, filter LiabilityFilter) ([]Liability, error) {
	rows, err := s.pool.Query(ctx, `
		select id, household_id, liability_type, name, coalesce(description, ''), coalesce(creditor, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			original_amount, current_balance, currency, status, created_at::text, updated_at::text
		from liabilities
		where ($1::text = '' or liability_type = $1)
		and ($2::text = '' or status = $2)
		order by status, name, id
	`, filter.LiabilityType, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Liability{}
	for rows.Next() {
		var item Liability
		if err := scanLiability(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) Liability(ctx context.Context, id int64) (Liability, error) {
	var item Liability
	err := scanLiability(s.pool.QueryRow(ctx, `
		select id, household_id, liability_type, name, coalesce(description, ''), coalesce(creditor, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			original_amount, current_balance, currency, status, created_at::text, updated_at::text
		from liabilities where id = $1
	`, id), &item)
	if err == pgx.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	}
	return item, err
}

func (s *Postgres) CreateLiability(ctx context.Context, input SaveLiability) (Liability, error) {
	input, startDate, endDate, err := normalizeLiabilityInput(input)
	if err != nil {
		return Liability{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Liability{}, err
	}
	defer tx.Rollback(ctx)
	var item Liability
	err = scanLiability(tx.QueryRow(ctx, `
		insert into liabilities (household_id, liability_type, name, description, creditor, start_date, end_date, original_amount, current_balance, currency, status)
		values ((select id from households order by id limit 1), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		returning id, household_id, liability_type, name, coalesce(description, ''), coalesce(creditor, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			original_amount, current_balance, currency, status, created_at::text, updated_at::text
	`, input.LiabilityType, input.Name, input.Description, input.Creditor, startDate, endDate,
		input.OriginalAmountCents, input.CurrentBalanceCents, input.Currency, input.Status), &item)
	if err != nil {
		return Liability{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeLiability, EntityID: item.ID, EventType: "liability_created", CurrentDate: true, Description: "Schuld aangemaakt"}); err != nil {
		return Liability{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Liability{}, err
	}
	return item, nil
}

func (s *Postgres) UpdateLiability(ctx context.Context, id int64, input SaveLiability) (Liability, error) {
	input, startDate, endDate, err := normalizeLiabilityInput(input)
	if err != nil {
		return Liability{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Liability{}, err
	}
	defer tx.Rollback(ctx)
	var oldStatus string
	if err := tx.QueryRow(ctx, `select status from liabilities where id = $1 for update`, id).Scan(&oldStatus); err == pgx.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	} else if err != nil {
		return Liability{}, err
	}
	var item Liability
	err = scanLiability(tx.QueryRow(ctx, `
		update liabilities
		set liability_type = $1, name = $2, description = $3, creditor = $4, start_date = $5, end_date = $6,
			original_amount = $7, current_balance = $8, currency = $9, status = $10, updated_at = now()
		where id = $11
		returning id, household_id, liability_type, name, coalesce(description, ''), coalesce(creditor, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			original_amount, current_balance, currency, status, created_at::text, updated_at::text
	`, input.LiabilityType, input.Name, input.Description, input.Creditor, startDate, endDate,
		input.OriginalAmountCents, input.CurrentBalanceCents, input.Currency, input.Status, id), &item)
	if err == pgx.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	} else if err != nil {
		return Liability{}, err
	}
	eventType, description := "liability_updated", "Schuld bijgewerkt"
	if oldStatus != "repaid" && item.Status == "repaid" {
		eventType, description = "liability_repaid", "Schuld afgelost"
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeLiability, EntityID: id, EventType: eventType, CurrentDate: true, Description: description}); err != nil {
		return Liability{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Liability{}, err
	}
	return item, nil
}

func (s *Postgres) DeleteLiability(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedID int64
	if err := tx.QueryRow(ctx, `select id from liabilities where id = $1 for update`, id).Scan(&lockedID); err == pgx.ErrNoRows {
		return fmt.Errorf("liability not found")
	} else if err != nil {
		return err
	}
	var documentCount, relationCount int
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where source_type = $1 and target_type = $2 and target_id = $3 and relation_type = $4
	`, ObjectTypeDocument, ObjectTypeLiability, id, RelationTypeEvidenceFor).Scan(&documentCount); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where ((source_type = $1 and source_id = $2) or (target_type = $1 and target_id = $2))
		and not (source_type = $3 and target_type = $1 and target_id = $2 and relation_type = $4)
	`, ObjectTypeLiability, id, ObjectTypeDocument, RelationTypeEvidenceFor).Scan(&relationCount); err != nil {
		return err
	}
	if documentCount > 0 || relationCount > 0 {
		return fmt.Errorf("liability still has %d document links and %d object relations", documentCount, relationCount)
	}
	if err := cleanupDomainEvents(ctx, tx, ObjectTypeLiability, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from liabilities where id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type liabilityScanner interface {
	Scan(...any) error
}

func scanLiability(scanner liabilityScanner, item *Liability) error {
	return scanner.Scan(&item.ID, &item.HouseholdID, &item.LiabilityType, &item.Name, &item.Description, &item.Creditor,
		&item.StartDate, &item.EndDate, &item.OriginalAmountCents, &item.CurrentBalanceCents, &item.Currency, &item.Status,
		&item.CreatedAt, &item.UpdatedAt)
}

func normalizeLiabilityInput(input SaveLiability) (SaveLiability, any, any, error) {
	input.LiabilityType = strings.TrimSpace(input.LiabilityType)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Creditor = strings.TrimSpace(input.Creditor)
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" {
		return SaveLiability{}, nil, nil, fmt.Errorf("name is required")
	}
	if !validLiabilityType(input.LiabilityType) {
		return SaveLiability{}, nil, nil, fmt.Errorf("invalid liability type")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "planned" && input.Status != "active" && input.Status != "repaid" && input.Status != "cancelled" {
		return SaveLiability{}, nil, nil, fmt.Errorf("invalid liability status")
	}
	if input.OriginalAmountCents < 0 || input.CurrentBalanceCents < 0 {
		return SaveLiability{}, nil, nil, fmt.Errorf("liability amounts cannot be negative")
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}
	if !validCurrency(input.Currency) {
		return SaveLiability{}, nil, nil, fmt.Errorf("currency must contain three letters")
	}
	startDate, err := optionalDate(input.StartDate)
	if err != nil {
		return SaveLiability{}, nil, nil, fmt.Errorf("invalid start date")
	}
	endDate, err := optionalDate(input.EndDate)
	if err != nil {
		return SaveLiability{}, nil, nil, fmt.Errorf("invalid end date")
	}
	if startDate != nil && endDate != nil && endDate.(time.Time).Before(startDate.(time.Time)) {
		return SaveLiability{}, nil, nil, fmt.Errorf("end date cannot be before start date")
	}
	return input, startDate, endDate, nil
}

func validLiabilityType(value string) bool {
	switch value {
	case "mortgage", "loan", "tax_debt", "credit", "other":
		return true
	default:
		return false
	}
}
