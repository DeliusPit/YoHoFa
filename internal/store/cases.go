package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	ObjectTypeCase       = "case"
	CaseTypeMainCategory = "main_category"
)

type Case struct {
	ID             int64  `json:"id"`
	HouseholdID    int64  `json:"householdId"`
	MainCategoryID int64  `json:"mainCategoryId,omitempty"`
	CaseType       string `json:"caseType"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	StartDate      string `json:"startDate,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type SaveCase struct {
	CaseType    string `json:"caseType"`
	Title       string `json:"title"`
	Description string `json:"description"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Status      string `json:"status"`
}

type CaseFilter struct {
	CaseType string
	Status   string
}

type CaseRepository interface {
	Cases(context.Context, CaseFilter) ([]Case, error)
	Case(context.Context, int64) (Case, error)
	CreateCase(context.Context, SaveCase) (Case, error)
	UpdateCase(context.Context, int64, SaveCase) (Case, error)
	DeleteCase(context.Context, int64) error
}

func (s *Postgres) Cases(ctx context.Context, filter CaseFilter) ([]Case, error) {
	filter.CaseType = strings.TrimSpace(filter.CaseType)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" && !validCaseStatus(filter.Status) {
		return nil, fmt.Errorf("invalid case status")
	}
	rows, err := s.pool.Query(ctx, `
		select id, household_id, coalesce(main_category_id, 0), case_type, title, coalesce(description, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			status, created_at::text, updated_at::text
		from cases
		where ($1::text = '' or case_type = $1)
		and ($2::text = '' or status = $2)
		order by status, title, id
	`, filter.CaseType, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Case{}
	for rows.Next() {
		var item Case
		if err := scanCase(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) Case(ctx context.Context, id int64) (Case, error) {
	var item Case
	err := scanCase(s.pool.QueryRow(ctx, `
		select id, household_id, coalesce(main_category_id, 0), case_type, title, coalesce(description, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			status, created_at::text, updated_at::text
		from cases where id = $1
	`, id), &item)
	if err == pgx.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	}
	return item, err
}

func (s *Postgres) CreateCase(ctx context.Context, input SaveCase) (Case, error) {
	input, startDate, endDate, err := normalizeCaseInput(input)
	if err != nil {
		return Case{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	var item Case
	err = scanCase(tx.QueryRow(ctx, `
		insert into cases (household_id, case_type, title, description, start_date, end_date, status)
		values ((select id from households order by id limit 1), $1, $2, $3, $4, $5, $6)
		returning id, household_id, coalesce(main_category_id, 0), case_type, title, coalesce(description, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			status, created_at::text, updated_at::text
	`, input.CaseType, input.Title, input.Description, startDate, endDate, input.Status), &item)
	if err != nil {
		return Case{}, err
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: item.ID, EventType: "case_created", CurrentDate: true, Description: "Dossier aangemaakt"}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return item, nil
}

func (s *Postgres) UpdateCase(ctx context.Context, id int64, input SaveCase) (Case, error) {
	input, startDate, endDate, err := normalizeCaseInput(input)
	if err != nil {
		return Case{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	var oldStatus string
	var mainCategoryID int64
	if err := tx.QueryRow(ctx, `select status, coalesce(main_category_id, 0) from cases where id = $1 for update`, id).Scan(&oldStatus, &mainCategoryID); err == pgx.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	} else if err != nil {
		return Case{}, err
	}
	if mainCategoryID != 0 {
		return Case{}, fmt.Errorf("case is managed by a main category")
	}
	var item Case
	err = scanCase(tx.QueryRow(ctx, `
		update cases
		set case_type = $1, title = $2, description = $3, start_date = $4, end_date = $5, status = $6, updated_at = now()
		where id = $7
		returning id, household_id, coalesce(main_category_id, 0), case_type, title, coalesce(description, ''),
			coalesce(to_char(start_date, 'YYYY-MM-DD'), ''), coalesce(to_char(end_date, 'YYYY-MM-DD'), ''),
			status, created_at::text, updated_at::text
	`, input.CaseType, input.Title, input.Description, startDate, endDate, input.Status, id), &item)
	if err == pgx.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	} else if err != nil {
		return Case{}, err
	}
	eventType, description := "case_updated", "Dossier bijgewerkt"
	if oldStatus != "completed" && item.Status == "completed" {
		eventType, description = "case_completed", "Dossier afgerond"
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: id, EventType: eventType, CurrentDate: true, Description: description}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	return item, nil
}

func (s *Postgres) DeleteCase(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedID, mainCategoryID int64
	if err := tx.QueryRow(ctx, `select id, coalesce(main_category_id, 0) from cases where id = $1 for update`, id).Scan(&lockedID, &mainCategoryID); err == pgx.ErrNoRows {
		return fmt.Errorf("case not found")
	} else if err != nil {
		return err
	}
	if mainCategoryID != 0 {
		return fmt.Errorf("case is managed by a main category")
	}
	var documentCount, relationCount int
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where source_type = $1 and target_type = $2 and target_id = $3 and relation_type = $4
	`, ObjectTypeDocument, ObjectTypeCase, id, RelationTypeEvidenceFor).Scan(&documentCount); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `
		select count(*) from entity_relations
		where ((source_type = $1 and source_id = $2) or (target_type = $1 and target_id = $2))
		and not (source_type = $3 and target_type = $1 and target_id = $2 and relation_type = $4)
	`, ObjectTypeCase, id, ObjectTypeDocument, RelationTypeEvidenceFor).Scan(&relationCount); err != nil {
		return err
	}
	if documentCount > 0 || relationCount > 0 {
		return fmt.Errorf("case still has %d document links and %d object relations", documentCount, relationCount)
	}
	if err := cleanupDomainEvents(ctx, tx, ObjectTypeCase, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from cases where id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type caseScanner interface {
	Scan(...any) error
}

func scanCase(scanner caseScanner, item *Case) error {
	return scanner.Scan(&item.ID, &item.HouseholdID, &item.MainCategoryID, &item.CaseType, &item.Title, &item.Description,
		&item.StartDate, &item.EndDate, &item.Status, &item.CreatedAt, &item.UpdatedAt)
}

func normalizeCaseInput(input SaveCase) (SaveCase, any, any, error) {
	input.CaseType = strings.TrimSpace(input.CaseType)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	input.Status = strings.TrimSpace(input.Status)
	if input.CaseType == "" {
		return SaveCase{}, nil, nil, fmt.Errorf("case type is required")
	}
	if input.Title == "" {
		return SaveCase{}, nil, nil, fmt.Errorf("title is required")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if !validCaseStatus(input.Status) {
		return SaveCase{}, nil, nil, fmt.Errorf("invalid case status")
	}
	startDate, err := optionalDate(input.StartDate)
	if err != nil {
		return SaveCase{}, nil, nil, fmt.Errorf("invalid start date")
	}
	endDate, err := optionalDate(input.EndDate)
	if err != nil {
		return SaveCase{}, nil, nil, fmt.Errorf("invalid end date")
	}
	if startDate != nil && endDate != nil && endDate.(time.Time).Before(startDate.(time.Time)) {
		return SaveCase{}, nil, nil, fmt.Errorf("end date cannot be before start date")
	}
	return input, startDate, endDate, nil
}

func validCaseStatus(status string) bool {
	switch status {
	case "planned", "active", "completed", "archived":
		return true
	default:
		return false
	}
}
