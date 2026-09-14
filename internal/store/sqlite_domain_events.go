package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *SQLite) DomainEventsForObject(ctx context.Context, entityType string, entityID int64) ([]DomainEvent, error) {
	if !validDomainEventOwner(entityType) || entityID <= 0 {
		return nil, fmt.Errorf("invalid domain event entity")
	}
	if err := sqliteValidateDomainEventOwner(ctx, s.db, entityType, entityID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		select id, entity_type, entity_id, event_type, event_date, description,
			related_entity_type, related_entity_id, created_at
		from domain_events where entity_type = ? and entity_id = ?
		order by event_date desc, id desc limit 200
	`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	items := []DomainEvent{}
	for rows.Next() {
		var item DomainEvent
		if err := rows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.EventType, &item.EventDate, &item.Description,
			&item.RelatedEntityType, &item.RelatedEntityID, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].RelatedEntityType == nil {
			continue
		}
		label, err := s.sqliteEntityLabel(ctx, *items[i].RelatedEntityType, *items[i].RelatedEntityID)
		if err != nil {
			return nil, err
		}
		items[i].RelatedEntityLabel = &label
	}
	return items, nil
}

func appendSQLiteDomainEvent(ctx context.Context, tx *sql.Tx, input AppendDomainEvent) error {
	input.EntityType = strings.TrimSpace(input.EntityType)
	input.EventType = strings.TrimSpace(input.EventType)
	input.Description = strings.TrimSpace(input.Description)
	input.RelatedEntityType = strings.TrimSpace(input.RelatedEntityType)
	if err := validateAppendDomainEvent(input); err != nil {
		return err
	}
	var relatedType, relatedID any
	if input.RelatedEntityType != "" {
		relatedType, relatedID = input.RelatedEntityType, input.RelatedEntityID
	}
	eventDate := input.EventDate.UTC().Format("2006-01-02")
	if input.CurrentDate {
		eventDate = time.Now().UTC().Format("2006-01-02")
	}
	_, err := tx.ExecContext(ctx, `
		insert into domain_events (entity_type, entity_id, event_type, event_date, description, related_entity_type, related_entity_id)
		values (?, ?, ?, ?, ?, ?, ?)
	`, input.EntityType, input.EntityID, input.EventType, eventDate, input.Description, relatedType, relatedID)
	return err
}

type sqliteQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func sqliteValidateDomainEventOwner(ctx context.Context, query sqliteQueryRower, entityType string, entityID int64) error {
	var table string
	switch entityType {
	case ObjectTypeAsset:
		table = "assets"
	case ObjectTypeLiability:
		table = "liabilities"
	case ObjectTypeCase:
		table = "cases"
	default:
		return fmt.Errorf("invalid domain event entity")
	}
	var exists int
	if err := query.QueryRowContext(ctx, "select exists(select 1 from "+table+" where id = ?)", entityID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("%s not found", entityType)
	}
	return nil
}

func cleanupSQLiteDomainEvents(ctx context.Context, tx *sql.Tx, entityType string, entityID int64) error {
	if _, err := tx.ExecContext(ctx, `delete from domain_events where entity_type = ? and entity_id = ?`, entityType, entityID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		update domain_events set related_entity_type = null, related_entity_id = null
		where related_entity_type = ? and related_entity_id = ?
	`, entityType, entityID)
	return err
}

func (s *SQLite) sqliteEntityLabel(ctx context.Context, entityType string, entityID int64) (string, error) {
	var label string
	var err error
	switch entityType {
	case ObjectTypeAsset:
		err = s.db.QueryRowContext(ctx, `select name from assets where id = ?`, entityID).Scan(&label)
	case ObjectTypeLiability:
		err = s.db.QueryRowContext(ctx, `select name from liabilities where id = ?`, entityID).Scan(&label)
	case ObjectTypeTransaction:
		err = s.db.QueryRowContext(ctx, `select booked_on || ' - ' || description from transactions where id = ?`, entityID).Scan(&label)
	case ObjectTypeDocument:
		err = s.db.QueryRowContext(ctx, `select original_filename from documents where id = ?`, entityID).Scan(&label)
	case ObjectTypeCase:
		err = s.db.QueryRowContext(ctx, `select title from cases where id = ?`, entityID).Scan(&label)
	default:
		return fmt.Sprintf("%s #%d", entityType, entityID), nil
	}
	if err == sql.ErrNoRows {
		return fmt.Sprintf("%s #%d (niet gevonden)", entityType, entityID), nil
	}
	return label, err
}
