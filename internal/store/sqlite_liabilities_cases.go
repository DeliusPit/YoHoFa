package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *SQLite) Liabilities(ctx context.Context, filter LiabilityFilter) ([]Liability, error) {
	rows, err := s.db.QueryContext(ctx, sqliteLiabilitySelect+`
		where (? = '' or liability_type = ?) and (? = '' or status = ?)
		order by status, name, id
	`, filter.LiabilityType, filter.LiabilityType, filter.Status, filter.Status)
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

func (s *SQLite) Liability(ctx context.Context, id int64) (Liability, error) {
	var item Liability
	err := scanLiability(s.db.QueryRowContext(ctx, sqliteLiabilitySelect+` where id = ?`, id), &item)
	if err == sql.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	}
	return item, err
}

func (s *SQLite) CreateLiability(ctx context.Context, input SaveLiability) (Liability, error) {
	input, _, _, err := normalizeLiabilityInput(input)
	if err != nil {
		return Liability{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Liability{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		insert into liabilities (household_id, liability_type, name, description, creditor, start_date, end_date, original_amount, current_balance, currency, status)
		values ((select id from households order by id limit 1), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.LiabilityType, input.Name, input.Description, input.Creditor, sqliteOptionalDate(input.StartDate), sqliteOptionalDate(input.EndDate),
		input.OriginalAmountCents, input.CurrentBalanceCents, input.Currency, input.Status)
	if err != nil {
		return Liability{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Liability{}, err
	}
	var item Liability
	if err := scanLiability(tx.QueryRowContext(ctx, sqliteLiabilitySelect+` where id = ?`, id), &item); err != nil {
		return Liability{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeLiability, EntityID: id, EventType: "liability_created", CurrentDate: true, Description: "Schuld aangemaakt"}); err != nil {
		return Liability{}, err
	}
	if err := tx.Commit(); err != nil {
		return Liability{}, err
	}
	return item, nil
}

func (s *SQLite) UpdateLiability(ctx context.Context, id int64, input SaveLiability) (Liability, error) {
	input, _, _, err := normalizeLiabilityInput(input)
	if err != nil {
		return Liability{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Liability{}, err
	}
	defer tx.Rollback()
	var oldStatus string
	if err := tx.QueryRowContext(ctx, `select status from liabilities where id = ?`, id).Scan(&oldStatus); err == sql.ErrNoRows {
		return Liability{}, fmt.Errorf("liability not found")
	} else if err != nil {
		return Liability{}, err
	}
	_, err = tx.ExecContext(ctx, `
		update liabilities set liability_type = ?, name = ?, description = ?, creditor = ?, start_date = ?, end_date = ?,
			original_amount = ?, current_balance = ?, currency = ?, status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		where id = ?
	`, input.LiabilityType, input.Name, input.Description, input.Creditor, sqliteOptionalDate(input.StartDate), sqliteOptionalDate(input.EndDate),
		input.OriginalAmountCents, input.CurrentBalanceCents, input.Currency, input.Status, id)
	if err != nil {
		return Liability{}, err
	}
	var item Liability
	if err := scanLiability(tx.QueryRowContext(ctx, sqliteLiabilitySelect+` where id = ?`, id), &item); err != nil {
		return Liability{}, err
	}
	eventType, description := "liability_updated", "Schuld bijgewerkt"
	if oldStatus != "repaid" && item.Status == "repaid" {
		eventType, description = "liability_repaid", "Schuld afgelost"
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeLiability, EntityID: id, EventType: eventType, CurrentDate: true, Description: description}); err != nil {
		return Liability{}, err
	}
	if err := tx.Commit(); err != nil {
		return Liability{}, err
	}
	return item, nil
}

func (s *SQLite) DeleteLiability(ctx context.Context, id int64) error {
	return s.sqliteDeleteDomainObject(ctx, ObjectTypeLiability, id, "liability")
}

func (s *SQLite) Cases(ctx context.Context, filter CaseFilter) ([]Case, error) {
	filter.CaseType = strings.TrimSpace(filter.CaseType)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" && !validCaseStatus(filter.Status) {
		return nil, fmt.Errorf("invalid case status")
	}
	rows, err := s.db.QueryContext(ctx, sqliteCaseSelect+`
		where (? = '' or case_type = ?) and (? = '' or status = ?)
		order by status, title, id
	`, filter.CaseType, filter.CaseType, filter.Status, filter.Status)
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

func (s *SQLite) Case(ctx context.Context, id int64) (Case, error) {
	var item Case
	err := scanCase(s.db.QueryRowContext(ctx, sqliteCaseSelect+` where id = ?`, id), &item)
	if err == sql.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	}
	return item, err
}

func (s *SQLite) CreateCase(ctx context.Context, input SaveCase) (Case, error) {
	input, _, _, err := normalizeCaseInput(input)
	if err != nil {
		return Case{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		insert into cases (household_id, case_type, title, description, start_date, end_date, status)
		values ((select id from households order by id limit 1), ?, ?, ?, ?, ?, ?)
	`, input.CaseType, input.Title, input.Description, sqliteOptionalDate(input.StartDate), sqliteOptionalDate(input.EndDate), input.Status)
	if err != nil {
		return Case{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Case{}, err
	}
	var item Case
	if err := scanCase(tx.QueryRowContext(ctx, sqliteCaseSelect+` where id = ?`, id), &item); err != nil {
		return Case{}, err
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: id, EventType: "case_created", CurrentDate: true, Description: "Dossier aangemaakt"}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(); err != nil {
		return Case{}, err
	}
	return item, nil
}

func (s *SQLite) UpdateCase(ctx context.Context, id int64, input SaveCase) (Case, error) {
	input, _, _, err := normalizeCaseInput(input)
	if err != nil {
		return Case{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback()
	var oldStatus string
	var mainCategoryID int64
	if err := tx.QueryRowContext(ctx, `select status, coalesce(main_category_id, 0) from cases where id = ?`, id).Scan(&oldStatus, &mainCategoryID); err == sql.ErrNoRows {
		return Case{}, fmt.Errorf("case not found")
	} else if err != nil {
		return Case{}, err
	}
	if mainCategoryID != 0 {
		return Case{}, fmt.Errorf("case is managed by a main category")
	}
	_, err = tx.ExecContext(ctx, `
		update cases set case_type = ?, title = ?, description = ?, start_date = ?, end_date = ?, status = ?,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') where id = ?
	`, input.CaseType, input.Title, input.Description, sqliteOptionalDate(input.StartDate), sqliteOptionalDate(input.EndDate), input.Status, id)
	if err != nil {
		return Case{}, err
	}
	var item Case
	if err := scanCase(tx.QueryRowContext(ctx, sqliteCaseSelect+` where id = ?`, id), &item); err != nil {
		return Case{}, err
	}
	eventType, description := "case_updated", "Dossier bijgewerkt"
	if oldStatus != "completed" && item.Status == "completed" {
		eventType, description = "case_completed", "Dossier afgerond"
	}
	if err := appendSQLiteDomainEvent(ctx, tx, AppendDomainEvent{EntityType: ObjectTypeCase, EntityID: id, EventType: eventType, CurrentDate: true, Description: description}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(); err != nil {
		return Case{}, err
	}
	return item, nil
}

func (s *SQLite) DeleteCase(ctx context.Context, id int64) error {
	return s.sqliteDeleteDomainObject(ctx, ObjectTypeCase, id, "case")
}

func (s *SQLite) sqliteDeleteDomainObject(ctx context.Context, objectType string, id int64, publicName string) error {
	var table string
	switch objectType {
	case ObjectTypeAsset:
		table = "assets"
	case ObjectTypeLiability:
		table = "liabilities"
	case ObjectTypeCase:
		table = "cases"
	default:
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := sqliteValidateEntity(ctx, tx, objectType, id); err != nil {
		return err
	}
	if objectType == ObjectTypeCase {
		var mainCategoryID int64
		if err := tx.QueryRowContext(ctx, `select coalesce(main_category_id, 0) from cases where id = ?`, id).Scan(&mainCategoryID); err != nil {
			return err
		}
		if mainCategoryID != 0 {
			return fmt.Errorf("case is managed by a main category")
		}
	}
	var documentCount, relationCount int
	if err := tx.QueryRowContext(ctx, `
		select count(*) from entity_relations
		where source_type = 'document' and target_type = ? and target_id = ? and relation_type = 'evidence_for'
	`, objectType, id).Scan(&documentCount); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `
		select count(*) from entity_relations
		where ((source_type = ? and source_id = ?) or (target_type = ? and target_id = ?))
		and not (source_type = 'document' and target_type = ? and target_id = ? and relation_type = 'evidence_for')
	`, objectType, id, objectType, id, objectType, id).Scan(&relationCount); err != nil {
		return err
	}
	if documentCount > 0 || relationCount > 0 {
		return fmt.Errorf("%s still has %d document links and %d object relations", publicName, documentCount, relationCount)
	}
	if err := cleanupSQLiteDomainEvents(ctx, tx, objectType, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "delete from "+table+" where id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

const sqliteLiabilitySelect = `
	select id, household_id, liability_type, name, coalesce(description, ''), coalesce(creditor, ''),
		coalesce(start_date, ''), coalesce(end_date, ''), original_amount, current_balance, currency, status, created_at, updated_at
	from liabilities`

const sqliteCaseSelect = `
	select id, household_id, coalesce(main_category_id, 0), case_type, title, coalesce(description, ''), coalesce(start_date, ''), coalesce(end_date, ''),
		status, created_at, updated_at
	from cases`
