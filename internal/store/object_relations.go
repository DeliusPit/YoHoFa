package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	RelationTypeRelatesTo   = "relates_to"
	RelationTypeEvidenceFor = "evidence_for"
	RelationTypeFinancedBy  = "financed_by"
	ObjectTypeDocument      = "document"
)

type EntityRelation struct {
	ID           int64  `json:"id"`
	SourceType   string `json:"sourceType"`
	SourceID     int64  `json:"sourceId"`
	SourceLabel  string `json:"sourceLabel"`
	TargetType   string `json:"targetType"`
	TargetID     int64  `json:"targetId"`
	TargetLabel  string `json:"targetLabel"`
	RelationType string `json:"relationType"`
	ValidFrom    string `json:"validFrom,omitempty"`
	ValidTo      string `json:"validTo,omitempty"`
	Notes        string `json:"notes,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type SaveEntityRelation struct {
	SourceType   string
	SourceID     int64
	TargetType   string
	TargetID     int64
	RelationType string
	ValidFrom    string
	ValidTo      string
	Notes        string
}

type EntityRelationRepository interface {
	CreateEntityRelation(context.Context, SaveEntityRelation) (EntityRelation, error)
	DeleteEntityRelation(context.Context, int64, string, int64) error
	LinkAssetTransaction(context.Context, int64, int64) error
	UnlinkAssetTransaction(context.Context, int64, int64) error
	AssetTransactions(context.Context, int64) ([]Transaction, error)
	LinkLiabilityTransaction(context.Context, int64, int64) error
	UnlinkLiabilityTransaction(context.Context, int64, int64) error
	LiabilityTransactions(context.Context, int64) ([]Transaction, error)
	LinkCaseTransaction(context.Context, int64, int64) error
	UnlinkCaseTransaction(context.Context, int64, int64) error
	CaseTransactions(context.Context, int64) ([]Transaction, error)
	RelationsForObject(context.Context, string, int64) ([]EntityRelation, error)
}

func (s *Postgres) CreateEntityRelation(ctx context.Context, input SaveEntityRelation) (EntityRelation, error) {
	input, validFrom, validTo, err := normalizeEntityRelation(input)
	if err != nil {
		return EntityRelation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EntityRelation{}, err
	}
	defer tx.Rollback(ctx)
	firstType, firstID := input.SourceType, input.SourceID
	secondType, secondID := input.TargetType, input.TargetID
	if secondType < firstType || (secondType == firstType && secondID < firstID) {
		firstType, secondType = secondType, firstType
		firstID, secondID = secondID, firstID
	}
	if err := validateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return EntityRelation{}, err
	}
	if err := validateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return EntityRelation{}, err
	}

	var item EntityRelation
	inserted := true
	err = tx.QueryRow(ctx, `
		insert into entity_relations (source_type, source_id, target_type, target_id, relation_type, valid_from, valid_to, notes)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict do nothing
		returning id, source_type, source_id, target_type, target_id, relation_type,
			coalesce(to_char(valid_from, 'YYYY-MM-DD'), ''), coalesce(to_char(valid_to, 'YYYY-MM-DD'), ''), coalesce(notes, ''), created_at::text
	`, input.SourceType, input.SourceID, input.TargetType, input.TargetID, input.RelationType, validFrom, validTo, input.Notes).Scan(
		&item.ID, &item.SourceType, &item.SourceID, &item.TargetType, &item.TargetID, &item.RelationType,
		&item.ValidFrom, &item.ValidTo, &item.Notes, &item.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		inserted = false
		err = tx.QueryRow(ctx, `
			select id, source_type, source_id, target_type, target_id, relation_type,
				coalesce(to_char(valid_from, 'YYYY-MM-DD'), ''), coalesce(to_char(valid_to, 'YYYY-MM-DD'), ''), coalesce(notes, ''), created_at::text
			from entity_relations
			where source_type = $1 and source_id = $2 and target_type = $3 and target_id = $4 and relation_type = $5
			and valid_from is not distinct from $6::date and valid_to is not distinct from $7::date
		`, input.SourceType, input.SourceID, input.TargetType, input.TargetID, input.RelationType, validFrom, validTo).Scan(
			&item.ID, &item.SourceType, &item.SourceID, &item.TargetType, &item.TargetID, &item.RelationType,
			&item.ValidFrom, &item.ValidTo, &item.Notes, &item.CreatedAt,
		)
	}
	if err != nil {
		return EntityRelation{}, err
	}
	if inserted {
		if err := s.appendRelationDomainEvents(ctx, tx, item, true); err != nil {
			return EntityRelation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return EntityRelation{}, err
	}
	if err := s.addEntityLabels(ctx, &item); err != nil {
		return EntityRelation{}, err
	}
	return item, nil
}

func (s *Postgres) DeleteEntityRelation(ctx context.Context, id int64, objectType string, objectID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var item EntityRelation
	err = tx.QueryRow(ctx, `
		select id, source_type, source_id, target_type, target_id, relation_type,
			coalesce(to_char(valid_from, 'YYYY-MM-DD'), ''), coalesce(to_char(valid_to, 'YYYY-MM-DD'), ''), coalesce(notes, ''), created_at::text
		from entity_relations
		where id = $1 and ((source_type = $2 and source_id = $3) or (target_type = $2 and target_id = $3))
	`, id, objectType, objectID).Scan(&item.ID, &item.SourceType, &item.SourceID, &item.TargetType, &item.TargetID, &item.RelationType,
		&item.ValidFrom, &item.ValidTo, &item.Notes, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("entity relation not found")
	} else if err != nil {
		return err
	}
	firstType, firstID := item.SourceType, item.SourceID
	secondType, secondID := item.TargetType, item.TargetID
	if secondType < firstType || (secondType == firstType && secondID < firstID) {
		firstType, secondType = secondType, firstType
		firstID, secondID = secondID, firstID
	}
	if err := validateAndLockEntity(ctx, tx, firstType, firstID); err != nil {
		return err
	}
	if err := validateAndLockEntity(ctx, tx, secondType, secondID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `delete from entity_relations where id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("entity relation not found")
	}
	if err := s.appendRelationDomainEvents(ctx, tx, item, false); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Postgres) LinkAssetTransaction(ctx context.Context, assetID, transactionID int64) error {
	return s.linkObjectTransaction(ctx, ObjectTypeAsset, assetID, transactionID)
}

func (s *Postgres) UnlinkAssetTransaction(ctx context.Context, assetID, transactionID int64) error {
	return s.unlinkObjectTransaction(ctx, ObjectTypeAsset, assetID, transactionID)
}

func (s *Postgres) AssetTransactions(ctx context.Context, assetID int64) ([]Transaction, error) {
	return s.objectTransactions(ctx, ObjectTypeAsset, assetID)
}

func (s *Postgres) LinkLiabilityTransaction(ctx context.Context, liabilityID, transactionID int64) error {
	return s.linkObjectTransaction(ctx, ObjectTypeLiability, liabilityID, transactionID)
}

func (s *Postgres) UnlinkLiabilityTransaction(ctx context.Context, liabilityID, transactionID int64) error {
	return s.unlinkObjectTransaction(ctx, ObjectTypeLiability, liabilityID, transactionID)
}

func (s *Postgres) LiabilityTransactions(ctx context.Context, liabilityID int64) ([]Transaction, error) {
	return s.objectTransactions(ctx, ObjectTypeLiability, liabilityID)
}

func (s *Postgres) LinkCaseTransaction(ctx context.Context, caseID, transactionID int64) error {
	return s.linkObjectTransaction(ctx, ObjectTypeCase, caseID, transactionID)
}

func (s *Postgres) UnlinkCaseTransaction(ctx context.Context, caseID, transactionID int64) error {
	return s.unlinkObjectTransaction(ctx, ObjectTypeCase, caseID, transactionID)
}

func (s *Postgres) CaseTransactions(ctx context.Context, caseID int64) ([]Transaction, error) {
	return s.objectTransactions(ctx, ObjectTypeCase, caseID)
}

func (s *Postgres) linkObjectTransaction(ctx context.Context, objectType string, objectID, transactionID int64) error {
	_, err := s.CreateEntityRelation(ctx, SaveEntityRelation{
		SourceType: objectType, SourceID: objectID,
		TargetType: ObjectTypeTransaction, TargetID: transactionID,
		RelationType: RelationTypeRelatesTo,
	})
	return err
}

func (s *Postgres) unlinkObjectTransaction(ctx context.Context, objectType string, objectID, transactionID int64) error {
	if !validDomainEventOwner(objectType) {
		return fmt.Errorf("unsupported object type")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := validateAndLockEntity(ctx, tx, objectType, objectID); err != nil {
		return err
	}
	if err := validateAndLockEntity(ctx, tx, ObjectTypeTransaction, transactionID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		delete from entity_relations
		where source_type = $1 and source_id = $2 and target_type = $3 and target_id = $4 and relation_type = $5
	`, objectType, objectID, ObjectTypeTransaction, transactionID, RelationTypeRelatesTo)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s transaction link not found", objectType)
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{
		EntityType: objectType, EntityID: objectID, EventType: "transaction_unlinked", CurrentDate: true,
		Description: "Transactie ontkoppeld", RelatedEntityType: ObjectTypeTransaction, RelatedEntityID: transactionID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Postgres) appendRelationDomainEvents(ctx context.Context, tx pgx.Tx, relation EntityRelation, linked bool) error {
	var eventDate time.Time
	currentDate := true
	if linked {
		switch {
		case relation.RelationType == RelationTypeEvidenceFor:
			if err := tx.QueryRow(ctx, `select coalesce(document_date, current_date) from documents where id = $1`, relation.SourceID).Scan(&eventDate); err != nil {
				return err
			}
			currentDate = false
		case relation.RelationType == RelationTypeRelatesTo && relation.TargetType == ObjectTypeTransaction:
			if err := tx.QueryRow(ctx, `select booked_on from transactions where id = $1`, relation.TargetID).Scan(&eventDate); err != nil {
				return err
			}
			currentDate = false
		case relation.ValidFrom != "":
			parsed, err := time.Parse("2006-01-02", relation.ValidFrom)
			if err != nil {
				return err
			}
			eventDate = parsed
			currentDate = false
		}
	}
	if relation.RelationType == RelationTypeEvidenceFor && validDomainEventOwner(relation.TargetType) {
		eventType, description := "document_linked", "Document gekoppeld"
		if !linked {
			eventType, description = "document_unlinked", "Document ontkoppeld"
		}
		return appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.TargetType, EntityID: relation.TargetID, EventType: eventType,
			EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: ObjectTypeDocument, RelatedEntityID: relation.SourceID})
	}
	if relation.RelationType == RelationTypeRelatesTo && relation.TargetType == ObjectTypeTransaction && validDomainEventOwner(relation.SourceType) {
		eventType, description := "transaction_linked", "Transactie gekoppeld"
		if !linked {
			eventType, description = "transaction_unlinked", "Transactie ontkoppeld"
		}
		return appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.SourceType, EntityID: relation.SourceID, EventType: eventType,
			EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: ObjectTypeTransaction, RelatedEntityID: relation.TargetID})
	}
	objectRelation := relation.RelationType == RelationTypeFinancedBy ||
		(relation.RelationType == RelationTypeRelatesTo && relation.SourceType == ObjectTypeCase &&
			(relation.TargetType == ObjectTypeAsset || relation.TargetType == ObjectTypeLiability))
	if !objectRelation {
		return nil
	}
	eventType, description := "relation_linked", "Gerelateerd object gekoppeld"
	if !linked {
		eventType, description = "relation_unlinked", "Gerelateerd object ontkoppeld"
	}
	if err := appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.SourceType, EntityID: relation.SourceID, EventType: eventType,
		EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: relation.TargetType, RelatedEntityID: relation.TargetID}); err != nil {
		return err
	}
	return appendDomainEvent(ctx, tx, AppendDomainEvent{EntityType: relation.TargetType, EntityID: relation.TargetID, EventType: eventType,
		EventDate: eventDate, CurrentDate: currentDate, Description: description, RelatedEntityType: relation.SourceType, RelatedEntityID: relation.SourceID})
}

func (s *Postgres) objectTransactions(ctx context.Context, objectType string, objectID int64) ([]Transaction, error) {
	rows, err := s.pool.Query(ctx, `
		select transactions.id, transactions.account_id, coalesce(transactions.category_id, 0), transactions.booked_on::text, transactions.description,
			coalesce(parties.name, transactions.counterparty_name_raw, transactions.counterparty, ''), coalesce(transactions.counterparty_iban_raw, transactions.counterparty_iban, ''), transactions.amount_cents,
			coalesce(transactions.external_reference, ''), coalesce(transactions.import_fingerprint, ''), transactions.hidden,
			(select count(distinct source_id) from entity_relations where source_type = 'document' and target_type = 'transaction' and target_id = transactions.id and relation_type = 'evidence_for')
		from transactions
		left join parties on parties.id = transactions.counterparty_party_id
		where exists (
			select 1 from entity_relations
			where target_type = $1 and target_id = transactions.id
			and source_type = $2 and source_id = $3 and relation_type = $4
		)
		order by transactions.booked_on desc, transactions.id desc
	`, ObjectTypeTransaction, objectType, objectID, RelationTypeRelatesTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Transaction{}
	for rows.Next() {
		var item Transaction
		if err := rows.Scan(&item.ID, &item.AccountID, &item.CategoryID, &item.BookedOn, &item.Description, &item.Counterparty,
			&item.CounterpartyIBAN, &item.AmountCents, &item.ExternalReference, &item.ImportFingerprint, &item.Hidden, &item.DocumentCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Postgres) RelationsForObject(ctx context.Context, objectType string, objectID int64) ([]EntityRelation, error) {
	rows, err := s.pool.Query(ctx, `
		select id, source_type, source_id, target_type, target_id, relation_type,
			coalesce(to_char(valid_from, 'YYYY-MM-DD'), ''), coalesce(to_char(valid_to, 'YYYY-MM-DD'), ''), coalesce(notes, ''), created_at::text
		from entity_relations
		where (source_type = $1 and source_id = $2) or (target_type = $1 and target_id = $2)
		order by created_at, id
	`, objectType, objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EntityRelation{}
	for rows.Next() {
		var item EntityRelation
		if err := rows.Scan(&item.ID, &item.SourceType, &item.SourceID, &item.TargetType, &item.TargetID, &item.RelationType,
			&item.ValidFrom, &item.ValidTo, &item.Notes, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := s.addEntityLabels(ctx, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeEntityRelation(input SaveEntityRelation) (SaveEntityRelation, any, any, error) {
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.RelationType = strings.TrimSpace(input.RelationType)
	input.ValidFrom = strings.TrimSpace(input.ValidFrom)
	input.ValidTo = strings.TrimSpace(input.ValidTo)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.SourceID <= 0 || input.TargetID <= 0 {
		return SaveEntityRelation{}, nil, nil, fmt.Errorf("source and target are required")
	}
	if input.RelationType == RelationTypeEvidenceFor && input.TargetType == ObjectTypeDocument && input.SourceType != ObjectTypeDocument {
		input.SourceType, input.TargetType = input.TargetType, input.SourceType
		input.SourceID, input.TargetID = input.TargetID, input.SourceID
	}
	if input.RelationType == RelationTypeFinancedBy && input.SourceType == ObjectTypeLiability && input.TargetType == ObjectTypeAsset {
		input.SourceType, input.TargetType = input.TargetType, input.SourceType
		input.SourceID, input.TargetID = input.TargetID, input.SourceID
	}
	if input.RelationType == RelationTypeRelatesTo && input.TargetType == ObjectTypeCase &&
		(input.SourceType == ObjectTypeTransaction || input.SourceType == ObjectTypeAsset || input.SourceType == ObjectTypeLiability) {
		input.SourceType, input.TargetType = input.TargetType, input.SourceType
		input.SourceID, input.TargetID = input.TargetID, input.SourceID
	}
	if input.RelationType == RelationTypeRelatesTo && input.SourceType == ObjectTypeTransaction &&
		(input.TargetType == ObjectTypeAsset || input.TargetType == ObjectTypeLiability) {
		input.SourceType, input.TargetType = input.TargetType, input.SourceType
		input.SourceID, input.TargetID = input.TargetID, input.SourceID
	}
	if input.SourceType == input.TargetType && input.SourceID == input.TargetID {
		return SaveEntityRelation{}, nil, nil, fmt.Errorf("self relations are not allowed")
	}
	if input.RelationType == RelationTypeRelatesTo {
		caseRelation := input.SourceType == ObjectTypeCase &&
			(input.TargetType == ObjectTypeTransaction || input.TargetType == ObjectTypeAsset || input.TargetType == ObjectTypeLiability)
		transactionRelation := (input.SourceType == ObjectTypeAsset || input.SourceType == ObjectTypeLiability) && input.TargetType == ObjectTypeTransaction
		if !caseRelation && !transactionRelation {
			return SaveEntityRelation{}, nil, nil, fmt.Errorf("unsupported relates_to relation")
		}
	} else if input.RelationType == RelationTypeEvidenceFor {
		if input.SourceType != ObjectTypeDocument ||
			(input.TargetType != ObjectTypeAsset && input.TargetType != ObjectTypeTransaction && input.TargetType != ObjectTypeLiability && input.TargetType != ObjectTypeCase && input.TargetType != ObjectTypeReservation) {
			return SaveEntityRelation{}, nil, nil, fmt.Errorf("unsupported evidence_for relation")
		}
	} else if input.RelationType == RelationTypeFinancedBy {
		if input.SourceType != ObjectTypeAsset || input.TargetType != ObjectTypeLiability {
			return SaveEntityRelation{}, nil, nil, fmt.Errorf("financed_by supports assets financed by liabilities")
		}
	} else {
		return SaveEntityRelation{}, nil, nil, fmt.Errorf("unsupported relation type")
	}
	validFrom, err := optionalRelationDate(input.ValidFrom)
	if err != nil {
		return SaveEntityRelation{}, nil, nil, fmt.Errorf("invalid valid_from")
	}
	validTo, err := optionalRelationDate(input.ValidTo)
	if err != nil {
		return SaveEntityRelation{}, nil, nil, fmt.Errorf("invalid valid_to")
	}
	if validFrom != nil && validTo != nil && validTo.(time.Time).Before(validFrom.(time.Time)) {
		return SaveEntityRelation{}, nil, nil, fmt.Errorf("valid_to cannot be before valid_from")
	}
	return input, validFrom, validTo, nil
}

func optionalRelationDate(value string) (any, error) {
	if value == "" {
		return nil, nil
	}
	return time.Parse("2006-01-02", value)
}

func validateAndLockEntity(ctx context.Context, tx pgx.Tx, entityType string, entityID int64) error {
	var id int64
	var err error
	switch entityType {
	case ObjectTypeAsset:
		err = tx.QueryRow(ctx, `select id from assets where id = $1 for key share`, entityID).Scan(&id)
	case ObjectTypeLiability:
		err = tx.QueryRow(ctx, `select id from liabilities where id = $1 for key share`, entityID).Scan(&id)
	case ObjectTypeReservation:
		err = tx.QueryRow(ctx, `select id from reservations where id = $1 for key share`, entityID).Scan(&id)
	case ObjectTypeTransaction:
		err = tx.QueryRow(ctx, `select id from transactions where id = $1 for key share`, entityID).Scan(&id)
	case ObjectTypeDocument:
		err = tx.QueryRow(ctx, `select id from documents where id = $1 for key share`, entityID).Scan(&id)
	case ObjectTypeCase:
		err = tx.QueryRow(ctx, `select id from cases where id = $1 for key share`, entityID).Scan(&id)
	default:
		return fmt.Errorf("unsupported entity type")
	}
	if err == pgx.ErrNoRows {
		return fmt.Errorf("%s not found", entityType)
	}
	return err
}

func (s *Postgres) addEntityLabels(ctx context.Context, relation *EntityRelation) error {
	sourceLabel, err := s.entityLabel(ctx, relation.SourceType, relation.SourceID)
	if err != nil {
		return err
	}
	targetLabel, err := s.entityLabel(ctx, relation.TargetType, relation.TargetID)
	if err != nil {
		return err
	}
	relation.SourceLabel = sourceLabel
	relation.TargetLabel = targetLabel
	return nil
}

func (s *Postgres) entityLabel(ctx context.Context, entityType string, entityID int64) (string, error) {
	var label string
	var err error
	switch entityType {
	case ObjectTypeAsset:
		err = s.pool.QueryRow(ctx, `select name from assets where id = $1`, entityID).Scan(&label)
	case ObjectTypeLiability:
		err = s.pool.QueryRow(ctx, `select name from liabilities where id = $1`, entityID).Scan(&label)
	case ObjectTypeReservation:
		err = s.pool.QueryRow(ctx, `select name from reservations where id = $1`, entityID).Scan(&label)
	case ObjectTypeTransaction:
		err = s.pool.QueryRow(ctx, `select booked_on::text || ' - ' || description from transactions where id = $1`, entityID).Scan(&label)
	case ObjectTypeDocument:
		err = s.pool.QueryRow(ctx, `select original_filename from documents where id = $1`, entityID).Scan(&label)
	case ObjectTypeCase:
		err = s.pool.QueryRow(ctx, `select title from cases where id = $1`, entityID).Scan(&label)
	default:
		return fmt.Sprintf("%s #%d", entityType, entityID), nil
	}
	if err == pgx.ErrNoRows {
		return fmt.Sprintf("%s #%d (niet gevonden)", entityType, entityID), nil
	}
	return label, err
}
