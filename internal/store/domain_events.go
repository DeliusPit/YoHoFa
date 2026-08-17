package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type DomainEvent struct {
	ID                 int64   `json:"id"`
	EntityType         string  `json:"entityType"`
	EntityID           int64   `json:"entityId"`
	EventType          string  `json:"eventType"`
	EventDate          string  `json:"eventDate"`
	Description        string  `json:"description"`
	RelatedEntityType  *string `json:"relatedEntityType,omitempty"`
	RelatedEntityID    *int64  `json:"relatedEntityId,omitempty"`
	RelatedEntityLabel *string `json:"relatedEntityLabel,omitempty"`
	CreatedAt          string  `json:"createdAt"`
}

type DomainEventRepository interface {
	DomainEventsForObject(context.Context, string, int64) ([]DomainEvent, error)
}

type AppendDomainEvent struct {
	EntityType        string
	EntityID          int64
	EventType         string
	EventDate         time.Time
	CurrentDate       bool
	Description       string
	RelatedEntityType string
	RelatedEntityID   int64
}

func (s *Postgres) DomainEventsForObject(ctx context.Context, entityType string, entityID int64) ([]DomainEvent, error) {
	if !validDomainEventOwner(entityType) || entityID <= 0 {
		return nil, fmt.Errorf("invalid domain event entity")
	}
	if err := validateDomainEventOwnerExists(ctx, s.pool, entityType, entityID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		select id, entity_type, entity_id, event_type, event_date::text, description,
			related_entity_type, related_entity_id, created_at::text
		from domain_events where entity_type = $1 and entity_id = $2
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
		label, err := s.entityLabel(ctx, *items[i].RelatedEntityType, *items[i].RelatedEntityID)
		if err != nil {
			return nil, err
		}
		items[i].RelatedEntityLabel = &label
	}
	return items, nil
}

func appendDomainEvent(ctx context.Context, tx pgx.Tx, input AppendDomainEvent) error {
	input.EntityType = strings.TrimSpace(input.EntityType)
	input.EventType = strings.TrimSpace(input.EventType)
	input.Description = strings.TrimSpace(input.Description)
	input.RelatedEntityType = strings.TrimSpace(input.RelatedEntityType)
	if err := validateAppendDomainEvent(input); err != nil {
		return err
	}
	var relatedType any
	var relatedID any
	if input.RelatedEntityType != "" {
		relatedType, relatedID = input.RelatedEntityType, input.RelatedEntityID
	}
	var eventDate any
	if !input.CurrentDate {
		eventDate = input.EventDate
	}
	_, err := tx.Exec(ctx, `
		insert into domain_events (entity_type, entity_id, event_type, event_date, description, related_entity_type, related_entity_id)
		values ($1, $2, $3, coalesce($4::date, current_date), $5, $6, $7)
	`, input.EntityType, input.EntityID, input.EventType, eventDate, input.Description, relatedType, relatedID)
	return err
}

func validateAppendDomainEvent(input AppendDomainEvent) error {
	if !validDomainEventOwner(input.EntityType) || input.EntityID <= 0 || input.EventType == "" || input.Description == "" || (input.EventDate.IsZero() == !input.CurrentDate) {
		return fmt.Errorf("invalid domain event")
	}
	if (input.RelatedEntityType == "") != (input.RelatedEntityID == 0) || (input.RelatedEntityType != "" && !validDomainEventRelatedType(input.RelatedEntityType)) {
		return fmt.Errorf("invalid related domain event entity")
	}
	if input.RelatedEntityType == input.EntityType && input.RelatedEntityID == input.EntityID {
		return fmt.Errorf("domain event cannot relate to itself")
	}
	return nil
}

func validDomainEventOwner(entityType string) bool {
	return entityType == ObjectTypeAsset || entityType == ObjectTypeLiability || entityType == ObjectTypeCase
}

func validDomainEventRelatedType(entityType string) bool {
	return validDomainEventOwner(entityType) || entityType == ObjectTypeDocument || entityType == ObjectTypeTransaction
}

type domainEventQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func validateDomainEventOwnerExists(ctx context.Context, query domainEventQuerier, entityType string, entityID int64) error {
	var exists bool
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
	if err := query.QueryRow(ctx, "select exists(select 1 from "+table+" where id = $1)", entityID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s not found", entityType)
	}
	return nil
}

func cleanupDomainEvents(ctx context.Context, tx pgx.Tx, entityType string, entityID int64) error {
	if _, err := tx.Exec(ctx, `delete from domain_events where entity_type = $1 and entity_id = $2`, entityType, entityID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		update domain_events set related_entity_type = null, related_entity_id = null
		where related_entity_type = $1 and related_entity_id = $2
	`, entityType, entityID)
	return err
}
